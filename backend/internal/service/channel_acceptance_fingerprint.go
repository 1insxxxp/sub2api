package service

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strings"
)

// These labels describe the upstream hop seen by the acceptance transport.
// Values from headers, signatures, identifiers and model output never enter
// the fingerprint. A relay can reproduce every response-shape signal.
type acceptanceFingerprint struct {
	endpoint string
	signals  map[string]bool
}

func newAcceptanceFingerprint() *acceptanceFingerprint {
	return &acceptanceFingerprint{signals: make(map[string]bool)}
}

func (f *acceptanceFingerprint) snapshot() *acceptanceFingerprint {
	if f == nil {
		return nil
	}
	return &acceptanceFingerprint{endpoint: f.endpoint, signals: maps.Clone(f.signals)}
}

// The forwarder can return while a detached reader still holds the live
// parser. Finish seals it before we copy the fingerprint under its mutex.
func (p *acceptanceEvidenceParser) reportFingerprint(declared string, result *AcceptanceResult) {
	p.Finish()
	p.mu.Lock()
	snapshot := p.fingerprint.snapshot()
	p.mu.Unlock()
	snapshot.report(declared, result)
}

func (p *acceptanceEvidenceParser) observeFingerprintResponse(r *http.Request, source AcceptanceSource, resp *http.Response, decodedBedrock bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.finished || p.fingerprint == nil {
		return
	}
	p.fingerprint.observeHTTP(r, source, resp.Header)
	media, _, _ := strings.Cut(resp.Header.Get("Content-Type"), ";")
	if !decodedBedrock && resp.Body != nil && strings.EqualFold(strings.TrimSpace(media), "application/vnd.amazon.eventstream") {
		p.fingerprintStream = &acceptanceFingerprintStream{remaining: acceptanceFingerprintStreamLimit}
	}
}

func (f *acceptanceFingerprint) observeHTTP(r *http.Request, source AcceptanceSource, h http.Header) {
	if getHeaderRaw(h, "anthropic-ratelimit-unified-5h-status") != "" ||
		getHeaderRaw(h, "anthropic-ratelimit-unified-7d-status") != "" {
		f.signals["anthropic_quota_windows"] = true
	}
	if getHeaderRaw(h, "x-amzn-bedrock-invocation-latency") != "" {
		f.signals["bedrock_invocation_header"] = true
	}
	if source.Kind != "direct" {
		// This shared Amazon Q endpoint is compatible with Kiro. The
		// declaration still requires independent evidence of the caller.
		parts := strings.Split(acceptanceDirectHost(r), ".")
		if len(parts) == 4 && (parts[0] == "q" || parts[0] == "codewhisperer") &&
			parts[1] != "" && parts[2] == "amazonaws" && parts[3] == "com" && r.URL.Path == "/generateAssistantResponse" {
			f.signals["amazon_assistant_endpoint"] = true
		}
		return
	}
	switch source.Channel {
	case "google_code_assist":
		if strings.HasPrefix(getHeaderRaw(r.Header, "user-agent"), "antigravity/") {
			f.signals["antigravity_client_on_code_assist"] = true
		}
	case "anthropic":
		if strings.HasPrefix(getHeaderRaw(r.Header, "authorization"), "Bearer ") {
			f.signals["anthropic_bearer"] = true
		}
	case "google_developer_api", "vertex", "bedrock":
	default:
		return
	}
	f.endpoint = source.Channel
}

func (f *acceptanceFingerprint) signature(label string, value any) {
	switch strings.TrimSpace(acceptanceString(value)) {
	case "":
	case geminiDummyThoughtSignature:
		f.signals["signature_placeholder"] = true
	default:
		f.signals[label] = true
	}
}

func (f *acceptanceFingerprint) messagesBlock(b map[string]any) {
	if acceptanceString(b["type"]) == "thinking" {
		f.signature("messages_signature", b["signature"])
	}
}

func (f *acceptanceFingerprint) observeFrame(m map[string]any) {
	switch acceptanceString(m["type"]) {
	case "message", "message_start":
		msg := m
		if acceptanceString(m["type"]) == "message_start" {
			msg = acceptanceObject(m["message"])
		}
		if _, ok := msg["content"].([]any); ok {
			f.signals["messages_envelope"] = true
			for _, block := range acceptanceArray(msg["content"]) {
				f.messagesBlock(acceptanceObject(block))
			}
			switch id := acceptanceString(msg["id"]); {
			case strings.HasPrefix(id, "msg_bdrk_"):
				f.signals["bedrock_message_id_shape"] = true
			case strings.HasPrefix(id, "msg_vrtx_"):
				f.signals["vertex_message_id_shape"] = true
			}
		}
	case "content_block_start":
		f.messagesBlock(acceptanceObject(m["content_block"]))
	case "content_block_delta":
		d := acceptanceObject(m["delta"])
		if acceptanceString(d["type"]) == "signature_delta" {
			f.signature("messages_signature", d["signature"])
		}
	}
	if _, ok := acceptanceObject(m["response"])["candidates"].([]any); ok {
		f.signals["google_internal_envelope"] = true
	}
	if c, ok := m["candidates"].([]any); ok {
		for _, candidate := range c {
			content := acceptanceObject(acceptanceObject(candidate)["content"])
			if parts, ok := content["parts"].([]any); ok {
				f.signals["gemini_candidates"] = true
				for _, part := range parts {
					f.signature("gemini_thought_signature", acceptanceObject(part)["thoughtSignature"])
				}
			}
		}
	}
	// A tagged event or one member of the AWS event union contributes at
	// most one signal. Ignore ambiguous objects and unrelated protocol types.
	if typ := acceptanceString(m["type"]); typ != "" {
		f.observeAssistantEvent(typ, m)
		return
	}
	var event string
	for _, name := range []string{"assistantResponseEvent", "contextUsageEvent", "meteringEvent", "toolUseEvent"} {
		if acceptanceObject(m[name]) != nil {
			if event != "" {
				return
			}
			event = name
		}
	}
	if event != "" {
		f.observeAssistantEvent(event, acceptanceObject(m[event]))
	}
}

func (f *acceptanceFingerprint) observeAssistantEvent(name string, payload map[string]any) {
	switch name {
	case "assistantResponseEvent":
		if _, ok := payload["content"].(string); ok {
			f.signals["kiro_assistant_event"] = true
		}
	case "toolUseEvent":
		if acceptanceString(payload["toolUseId"]) != "" && acceptanceString(payload["name"]) != "" {
			f.signals["kiro_tool_use_event"] = true
		}
	case "contextUsageEvent":
		if _, ok := payload["contextUsagePercentage"].(json.Number); ok {
			f.signals["kiro_context_usage_event"] = true
		}
	case "meteringEvent":
		if _, ok := payload["usage"].(json.Number); ok && acceptanceString(payload["unit"]) != "" {
			f.signals["kiro_metering_event"] = true
		}
	}
}

// Count distinct event shapes within this response. An AWS endpoint, a
// Bedrock envelope, repeated deltas or a text claim cannot supply a vote.
func (f *acceptanceFingerprint) kiroEventKinds() int {
	count := 0
	for _, key := range []string{"kiro_assistant_event", "kiro_context_usage_event", "kiro_metering_event", "kiro_tool_use_event"} {
		if f.signals[key] {
			count++
		}
	}
	return count
}

var acceptanceFingerprintLabels = map[string]string{
	"unknown": "未知", "vertex": "Vertex AI", "google_developer_api": "Gemini Developer API",
	"google_code_assist": "Google Code Assist", "antigravity": "Antigravity", "anthropic": "Anthropic",
	"claude_max": "Claude Max", "kiro": "Kiro/Amazon Q", "bedrock": "Amazon Bedrock",
}

func (f *acceptanceFingerprint) compare(declared string, kiroKinds int) string {
	family := declared
	switch declared {
	case "unknown":
		if kiroKinds >= 2 {
			return "source_candidate_inferred"
		}
		return "source_inconclusive"
	case "claude_max":
		family = "anthropic"
	case "antigravity":
		family = "google_code_assist"
	}
	if f.endpoint != "" && f.endpoint != family || kiroKinds >= 2 && declared != "kiro" {
		return "source_mismatch"
	}
	if f.endpoint == declared {
		return "source_endpoint_match"
	}
	switch declared {
	case "kiro":
		if f.signals["amazon_assistant_endpoint"] || kiroKinds > 0 {
			return "source_compatible"
		}
	case "antigravity":
		if f.signals["antigravity_client_on_code_assist"] {
			return "source_compatible"
		}
	case "claude_max":
		if f.signals["anthropic_bearer"] && f.signals["anthropic_quota_windows"] {
			return "source_compatible"
		}
	case "bedrock":
		if f.signals["bedrock_message_id_shape"] || f.signals["bedrock_invocation_header"] {
			return "source_compatible"
		}
	case "vertex":
		if f.signals["vertex_message_id_shape"] {
			return "source_compatible"
		}
	case "anthropic":
		if f.signals["messages_envelope"] || f.signals["messages_signature"] {
			return "source_compatible"
		}
	case "google_code_assist":
		if f.signals["google_internal_envelope"] {
			return "source_compatible"
		}
	}
	return "source_inconclusive"
}

func (f *acceptanceFingerprint) report(declared string, r *AcceptanceResult) {
	label, ok := acceptanceFingerprintLabels[declared]
	if !ok {
		return
	}
	kinds := f.kiroEventKinds()
	code := f.compare(declared, kinds)
	present := f.signals["messages_signature"] || f.signals["gemini_thought_signature"]
	evidence := map[string]any{
		"declared_source": declared, "endpoint": f.endpoint, "signals": slices.Sorted(maps.Keys(f.signals)), "verdict": code,
		"signature_present": present, "signature_verified": false,
	}
	message := ""
	if kinds >= 2 {
		evidence["inferred_candidate"] = "kiro"
		message = "多种上游事件结构指向 Kiro/Amazon Q。"
	}
	r.EffectiveParameters["source_fingerprint"] = evidence
	level := "info"
	switch code {
	case "source_endpoint_match":
		message += "已连接声明的 " + label + " 端点。"
	case "source_mismatch":
		level = "warning"
		if f.endpoint != "" {
			message += "连接端点为 " + acceptanceFingerprintLabels[f.endpoint] + "。"
		}
		message += "请复核声明来源 " + label + "。"
		if r.Status == "passed" || r.Status == "inconclusive" {
			r.Status = "anomaly"
		}
	case "source_compatible":
		if kinds < 2 {
			message += "接口特征与 " + label + " 相容，具体来源待核实。"
		}
	case "source_inconclusive":
		if f.endpoint != "" {
			message += "连接端点为 " + acceptanceFingerprintLabels[f.endpoint] + "。"
		}
		if declared == "unknown" {
			message += "现有证据不足以推断供应来源。"
		} else {
			message += "现有证据不足以确认 " + label + "。"
		}
	}
	if present {
		message += "签名字段存在，未经密码学验证。"
	}
	r.Findings = append(r.Findings, AcceptanceFinding{Code: code, Level: level, Message: message})
}
