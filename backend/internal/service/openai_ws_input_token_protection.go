package service

import (
	"bytes"

	"github.com/Wei-Shaw/sub2api/internal/pkg/wireusage"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// One request estimate survives retries. Each provider call retains its own
// observations; a cumulative usage event can update either count independently.
type openAIWSTurnInputProtection struct {
	turn         int
	evidence     *InputTokenProtection
	projector    *wireusage.Projector
	price        func(int, OpenAIForwardResult) bool
	projectUsage bool
	observed     openAIWSUsageObservation
	merged       []*OpenAIForwardResult
	clippedInput map[string]int
}

type openAIWSUsageObservation struct {
	round OpenAIChatUsageRound
	model upstreamResponseModelObserver
}

func (h *OpenAIWSIngressHooks) inputProtection(turn int, original []byte, model string) *openAIWSTurnInputProtection {
	if h == nil || h.InputProtectionForTurn == nil {
		return nil
	}
	evidence := h.InputProtectionForTurn(turn, original, model)
	if evidence == nil {
		return nil
	}
	return &openAIWSTurnInputProtection{turn: turn, evidence: evidence, projector: wireusage.New(evidence.Cap, false), price: h.ProjectUsageForTurn, projectUsage: h.ProjectUsageForTurn == nil}
}

func (p *openAIWSTurnInputProtection) beginLeaf() {
	if p != nil {
		p.observed = openAIWSUsageObservation{}
		p.merged = nil
		p.clippedInput = nil
		p.projectUsage = p.price == nil
	}
}

// Observe before client model restoration or usage projection. The relay's
// legacy totals parser can reject an entire partial terminal; keep the valid
// counters here independently for protected settlement.
func (p *openAIWSTurnInputProtection) observe(frame []byte, snapshot func() OpenAIForwardResult) {
	if p == nil {
		return
	}
	o := &p.observed
	if bytes.Contains(frame, []byte(`"model"`)) {
		o.model.ObserveOpenAI(frame, gjson.GetBytes(frame, "type").String())
	}
	if !bytes.Contains(frame, []byte(`"usage"`)) || !gjson.ValidBytes(frame) {
		return
	}
	obj := gjson.GetBytes(frame, "response.usage")
	if !obj.IsObject() {
		obj = gjson.GetBytes(frame, "usage")
	}
	if !obj.IsObject() {
		return
	}
	o.round.mergeUsage(obj)
	if !o.round.CacheReadReported {
		if n, ok := openAIChatReportedCount(obj, "prompt_cache_hit_tokens"); ok {
			o.round.CacheReadReported = true
			o.round.Usage.CacheReadInputTokens = n
		}
	}
	imageGen := gjson.GetBytes(frame, "response.tool_usage.image_gen")
	if !imageGen.Exists() {
		imageGen = gjson.GetBytes(frame, "tool_usage.image_gen")
	}
	mergeHostedImageGenToolUsage(imageGen, &o.round.Usage)
	if p.price != nil {
		r := snapshot()
		r.UpstreamTerminalEvent = normalizeOpenAIWSTerminalEvent(gjson.GetBytes(frame, "type").String())
		r.UpstreamResponseModel = o.model.Model()
		r.UpstreamResponseModelConflict = o.model.Conflict()
		r.UpstreamResponseServiceTier = o.model.ServiceTier()
		p.apply(&r)
		p.projectUsage = p.price(p.turn, r)
	}
}

func (p *openAIWSTurnInputProtection) apply(r *OpenAIForwardResult) {
	if p == nil || r == nil {
		return
	}
	o := &p.observed.round
	r.Usage.InputTokens, r.Usage.OutputTokens = o.Usage.InputTokens, o.Usage.OutputTokens
	r.InputTokensSource, r.OutputTokensSource = UsageTokenSourceUnreported, UsageTokenSourceUnreported
	if o.InputReported {
		r.InputTokensSource = UsageTokenSourceUpstream
	}
	if o.OutputReported {
		r.OutputTokensSource = UsageTokenSourceUpstream
	}
	if o.CacheReadReported {
		r.Usage.CacheReadInputTokens = o.Usage.CacheReadInputTokens
	}
	if o.CacheWriteReported {
		r.Usage.CacheCreationInputTokens = o.Usage.CacheCreationInputTokens
		r.Usage.CacheCreation30mInputTokens = o.Usage.CacheCreation30mInputTokens
		r.Usage.CacheCreation12hInputTokens = o.Usage.CacheCreation12hInputTokens
	}
	r.Usage.ImageInputTokens = o.Usage.ImageInputTokens
	r.Usage.ImageOutputTokens = o.Usage.ImageOutputTokens
}

func (p *openAIWSTurnInputProtection) project(frame []byte) []byte {
	if p != nil && len(p.merged) > 0 && bytes.Contains(frame, []byte(`"usage"`)) {
		// Continuation emits a single synthesized terminal after all leaves
		// return. Allocate its cap across only the token-priced leaves.
		tokenPriced := make(map[*OpenAIForwardResult]bool, len(p.merged))
		for _, r := range p.merged {
			if !continuationRoundFailedWithoutOutput(r.Outcome, r.DeliveredOutputTokens) && (p.price == nil || p.price(p.turn, *r)) {
				tokenPriced[r] = true
			}
		}
		if len(tokenPriced) == 0 {
			return frame
		}
		customer := projectOpenAIInputForBilling(&OpenAIForwardResult{ContinuationResults: p.merged}, p.evidence, tokenPriced)
		gross := InputTokenProtection{Limit: customer.Usage.InputTokens}
		return wireusage.New(func(read, input, short, long int) (int, int, int, int) {
			return gross.Cap(min(read, customer.Usage.CacheReadInputTokens), input, short, long)
		}, false).JSON(frame)
	}
	if p == nil {
		return frame
	}
	if !p.projectUsage && len(p.clippedInput) == 0 {
		return frame
	}
	if !bytes.Contains(frame, []byte(`"usage"`)) {
		return frame
	}
	path := "response.usage"
	obj := gjson.GetBytes(frame, path)
	if !obj.IsObject() {
		path = "usage"
		obj = gjson.GetBytes(frame, path)
	}
	if !obj.IsObject() {
		return frame
	}
	if !p.projectUsage {
		// A fixed-price terminal may report only output. Correct any inputs
		// provisionally clipped on an earlier cumulative frame.
		for field, n := range p.clippedInput {
			if !obj.Get(field).Exists() {
				frame, _ = sjson.SetBytes(frame, path+"."+field, n)
			}
		}
		p.clippedInput = nil
		return frame
	}
	for field := range p.clippedInput {
		if obj.Get(field).Exists() {
			delete(p.clippedInput, field)
		}
	}
	projected := p.projector.JSON(frame)
	if p.price != nil && !bytes.Equal(frame, projected) {
		if p.clippedInput == nil {
			p.clippedInput = make(map[string]int)
		}
		rememberClippedWSInputs(p.clippedInput, "", obj, gjson.GetBytes(projected, path))
	}
	return projected
}

func (p *openAIWSTurnInputProtection) stamp(r *OpenAIForwardResult) {
	if p != nil && r != nil {
		r.InputTokenProtection = p.evidence
	}
}

// Only store numeric input fields that the shared wire projector changed.
// Walk changed objects so cache TTL and image details retain their wire aliases.
func rememberClippedWSInputs(saved map[string]int, prefix string, raw, projected gjson.Result) {
	raw.ForEach(func(k, v gjson.Result) bool {
		key := k.String()
		other := projected.Get(key)
		if v.Raw == other.Raw {
			return true
		}
		path := prefix + key
		if v.IsObject() && other.IsObject() {
			rememberClippedWSInputs(saved, path+".", v, other)
		} else if v.Type == gjson.Number && other.Type == gjson.Number && key != "total_tokens" {
			saved[path] = int(v.Int())
		}
		return true
	})
}
