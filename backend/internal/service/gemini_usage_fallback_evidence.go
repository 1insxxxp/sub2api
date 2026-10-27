package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

const (
	geminiFallbackMaxSemanticBytes = 4 << 20
	geminiFallbackMaxStreams       = 4096
)

// GeminiUsageFallback contains only scalar output evidence and the short-lived
// input count request. Provider field presence remains distinct from estimates.
type GeminiUsageFallback struct {
	Input              *GeminiInputUsageFallback
	CandidateTokens    int
	ThoughtTokens      int
	UsageFields        uint8
	MeasuredCandidates int
	MeasuredThoughts   int
	UpstreamAccepted   bool
	OutputTruncated    bool
}

// Output text stays on the live attempt only. Async billing receives token
// counts, never generated text, tool arguments or opaque response payloads.
type geminiFallbackCandidate struct {
	text, thought   strings.Builder
	tools           []*geminiFallbackTool
	activeTools     map[string]*geminiFallbackTool
	identifiedTools map[string]*geminiFallbackTool
}

type geminiFallbackTool struct {
	name     string
	nameHash [32]byte
	args     strings.Builder
}

// reconcileGeminiFallbackText matches computeGeminiTextDelta, while appending
// incremental chunks without repeatedly copying the entire generated response.
func (a *GeminiUpstreamAttempt) reconcileFallbackText(target *strings.Builder, incoming string) {
	incoming = strings.TrimSuffix(incoming, "\u0000")
	if incoming == "" {
		return
	}
	seen := target.String()
	switch {
	case strings.HasPrefix(incoming, seen):
		incoming = incoming[len(seen):]
	case strings.HasPrefix(seen, incoming):
		// Duplicate/cumulative rewind, matching the streaming adapters.
		return
	}
	target.WriteString(a.boundedFallbackText(incoming))
}

func (a *GeminiUpstreamAttempt) boundedFallbackText(text string) string {
	remaining := geminiFallbackMaxSemanticBytes - a.fallbackOutputBytes
	if len(text) > remaining {
		a.fallbackOutputTruncated = true
		text = text[:max(remaining, 0)]
		for len(text) > 0 && !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	a.fallbackOutputBytes += len(text)
	return text
}

func (a *GeminiUpstreamAttempt) allowFallbackStream() bool {
	if a.fallbackStreamCount >= geminiFallbackMaxStreams {
		a.fallbackOutputTruncated = true
		return false
	}
	a.fallbackStreamCount++
	return true
}

func (a *GeminiUpstreamAttempt) observeFallbackOutput(raw []byte) {
	root := gjson.ParseBytes(raw)
	if wrapped := root.Get("response"); wrapped.IsObject() {
		root = wrapped
	}
	root.Get("candidates").ForEach(func(position, candidate gjson.Result) bool {
		toolPosition := 0
		index := position.Int()
		if explicit := candidate.Get("index"); explicit.Type == gjson.Number && explicit.Int() >= 0 {
			index = explicit.Int()
		}
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			text := part.Get("text")
			call := part.Get("functionCall")
			if (text.Type != gjson.String || text.String() == "") && !call.IsObject() {
				return true
			}
			if a.fallbackCandidates == nil {
				a.fallbackCandidates = make(map[int64]*geminiFallbackCandidate)
			}
			output := a.fallbackCandidates[index]
			if output == nil {
				if !a.allowFallbackStream() {
					return true
				}
				output = &geminiFallbackCandidate{}
				a.fallbackCandidates[index] = output
			}
			if text.Type == gjson.String && text.String() != "" {
				a.upstreamAccepted = true
				target := &output.text
				if part.Get("thought").Bool() {
					target = &output.thought
				}
				seen, incoming := target.String(), strings.TrimSuffix(text.String(), "\u0000")
				// Cumulative frames may repeat text before the same tool call.
				// Only a new incremental text boundary closes anonymous tools;
				// explicit IDs remain stable for the entire candidate.
				if incoming != "" && !strings.HasPrefix(seen, incoming) && (seen == "" || !strings.HasPrefix(incoming, seen)) {
					output.activeTools = nil
				}
				a.reconcileFallbackText(target, incoming)
			}
			if call.IsObject() {
				name, id := call.Get("name").String(), call.Get("id").String()
				args := geminiFallbackToolArguments(call.Get("args"))
				key := strconv.Itoa(toolPosition)
				toolPosition++
				if id != "" {
					digest := sha256.Sum256([]byte(id))
					key = "id:" + hex.EncodeToString(digest[:])
				}
				if name == "" && args == "" {
					return true
				}
				a.upstreamAccepted = true
				nameHash := sha256.Sum256([]byte(name))
				tool := output.activeTools[key]
				if id != "" {
					tool = output.identifiedTools[key]
				}
				if tool == nil || tool.nameHash != nameHash {
					if !a.allowFallbackStream() {
						return true
					}
					tool = &geminiFallbackTool{name: strings.Clone(a.boundedFallbackText(name)), nameHash: nameHash}
					output.tools = append(output.tools, tool)
					if id != "" {
						if output.identifiedTools == nil {
							output.identifiedTools = make(map[string]*geminiFallbackTool)
						}
						output.identifiedTools[key] = tool
					} else {
						if output.activeTools == nil {
							output.activeTools = make(map[string]*geminiFallbackTool)
						}
						output.activeTools[key] = tool
					}
				}
				if parsedArgs := call.Get("args"); parsedArgs.IsObject() || parsedArgs.IsArray() {
					// Structured args are complete snapshots; JSON closing braces
					// make prefix-based string reconciliation unsuitable here.
					a.fallbackOutputBytes -= tool.args.Len()
					tool.args.Reset()
					tool.args.WriteString(a.boundedFallbackText(args))
				} else {
					a.reconcileFallbackText(&tool.args, args)
				}
			}
			return true
		})
		return true
	})
}

func geminiFallbackToolArguments(args gjson.Result) string {
	if args.Type == gjson.String {
		return args.String()
	}
	if !args.Exists() || args.Type == gjson.Null {
		return ""
	}
	var compact bytes.Buffer
	if json.Compact(&compact, []byte(args.Raw)) != nil {
		return ""
	}
	return compact.String()
}

func (a *GeminiUpstreamAttempt) snapshotUsageFallback() *GeminiUsageFallback {
	if a == nil {
		return nil
	}
	fallback := &GeminiUsageFallback{
		UsageFields: a.event.UsageFields, MeasuredCandidates: int(a.candidates),
		MeasuredThoughts: int(a.event.ThoughtTokens), UpstreamAccepted: a.upstreamAccepted,
		OutputTruncated: a.fallbackOutputTruncated,
	}
	if a.inputFallback != nil {
		if a.event.UsageFields&1 != 0 {
			// Even an explicitly measured zero makes the count request needless.
			a.inputFallback.CountRequest = nil
		}
		input := *a.inputFallback
		fallback.Input = &input
	}
	if a.event.UsageFields&2 != 0 {
		return fallback // Reported candidate totals make output estimates unused.
	}
	codec, err := openAIInputTokensCodecForModel(a.model)
	if err != nil {
		return fallback
	}
	count := func(text string) int {
		n, err := codec.Count(text)
		if err != nil {
			return 0
		}
		return n
	}
	for _, output := range a.fallbackCandidates {
		fallback.CandidateTokens += count(output.text.String())
		fallback.ThoughtTokens += count(output.thought.String())
		for _, tool := range output.tools {
			fallback.CandidateTokens += count(tool.name) + count(tool.args.String())
		}
	}
	return fallback
}
