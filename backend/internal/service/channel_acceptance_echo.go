package service

import "strings"

// Compare only challenges whose entire answer is known. Avoid retaining extra
// raw text or blaming the relay for formatting chosen by the model itself.
func acceptanceCheckEcho(sample AcceptanceSample, r *AcceptanceResult, raw, client *acceptanceEvidenceParser) {
	if (sample.Category != "protocol" && sample.Category != "repeat") || sample.Protocol == "systemone" || sample.Expected == "" ||
		!raw.completed || !client.completed || raw.truncated || client.truncated || raw.hasTool || client.hasTool ||
		len(raw.excerpt) >= acceptanceExcerptLimit || len(client.excerpt) >= acceptanceExcerptLimit {
		return
	}
	origin, returned := strings.TrimSpace(raw.excerpt), strings.TrimSpace(client.excerpt)
	if origin == sample.Expected && returned != origin {
		acceptanceAddFinding(r, "client_challenge_text_changed", "上游正文精确返回了本轮校验码，客户端正文在转换后发生变化")
	} else if origin != sample.Expected && strings.Contains(origin, sample.Expected) {
		r.Findings = append(r.Findings, AcceptanceFinding{Code: "upstream_challenge_extra_text", Level: "info", Message: "上游原始正文包含本轮校验码及额外文字，可能是模型自行补充，无法据此判定上游注入"})
	}
}
