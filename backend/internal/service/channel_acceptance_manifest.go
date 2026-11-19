package service

import (
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
)

func acceptanceValidPrice(p *AcceptancePrice) bool {
	if p == nil || strings.TrimSpace(p.Source) == "" {
		return false
	}
	for _, n := range []float64{p.InputPerMillion, p.OutputPerMillion, p.CachePerMillion, p.CacheWritePerMillion} {
		if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) || n > 1e6 {
			return false
		}
	}
	return true
}

func acceptanceManifest(target AcceptanceSnapshot, extras []string, nonce string) []AcceptanceSample {
	items := make([]AcceptanceSample, 0, 24)
	protocols := target.Protocols
	if len(protocols) == 0 {
		protocols = acceptanceProtocols(target.Platform, target.AccountType, target.UpstreamModel)
	}
	if len(protocols) == 0 {
		return nil
	}
	primary := protocols[0]
	appendCase := func(category, protocol, prompt, expected string, stream, deep bool) {
		if protocol == "systemone" {
			stream = false
		}
		timeout := 180
		if deep {
			timeout = 360
		}
		// A conservative reservation, not a claimed provider token measurement.
		input := len([]byte(prompt))/2 + 512 + target.AdditionalInputEstimate
		if category == "image" || category == "audio" {
			input += 8192
		}
		out := 2048
		// Third-party count routes may incorrectly generate. Reserve the same
		// bounded allowance, release on a valid count response, and stop counting
		// this source after the first unsupported or generation-shaped response.
		inputPrice := target.Price.InputPerMillion
		if category == "cache" {
			inputPrice = math.Max(inputPrice, target.Price.CacheWritePerMillion)
		}
		cost := (float64(input)*inputPrice + float64(out)*target.Price.OutputPerMillion) / 1e6
		items = append(items, AcceptanceSample{ID: uuid.NewString(), Ordinal: len(items), Snapshot: target, Category: category, Protocol: protocol, Stream: stream, Prompt: prompt, Expected: expected, InputSize: input, MaxOutputTokens: 2048, TimeoutSeconds: timeout, EstimatedCostUSD: cost, Status: "pending"})
	}
	prompt := "Return exactly this verification code, with no explanation: " + nonce
	// The primary protocol is also exercised by non-streaming input and
	// streaming replay samples. Share those calls when three protocols exist.
	probeProtocols := protocols
	if len(protocols) == 3 {
		probeProtocols = protocols[1:]
	}
	for _, proto := range probeProtocols {
		for _, stream := range []bool{false, true} {
			if proto == "systemone" && stream {
				continue
			}
			appendCase("protocol", proto, prompt, nonce, stream, false)
		}
	}
	inputPrompts := make([]string, 0, 3)
	for _, size := range []int{256, 2048, 8192} {
		p := "Reference data:\n" + acceptanceText(size) + "\nReturn exactly: " + nonce
		inputPrompts = append(inputPrompts, p)
		for i := 0; i < 2; i++ {
			appendCase("input", primary, p, nonce, false, false)
		}
	}
	for i := 0; i < 3; i++ {
		expected := fmt.Sprintf("%s-check-%d", nonce, i)
		appendCase("repeat", primary, "Return exactly this NEW verification code, do not reuse an earlier answer: "+expected, expected, true, false)
	}
	cachePrompt := "Unique prefix " + nonce + "\n" + acceptanceText(8192) + "\nReturn exactly: " + nonce
	for i := 0; i < 4; i++ {
		appendCase("cache", primary, cachePrompt, nonce, true, false)
	}
	if acceptanceHasCountEndpoint(target) {
		for _, p := range inputPrompts {
			appendCase("count", "count_tokens", p, "", false, false)
		}
	}
	for _, extra := range extras {
		if primary == "systemone" && extra != "long_context" {
			continue
		}
		switch extra {
		case "long_context":
			appendCase(extra, primary, acceptanceText(16000)+"\nHidden verification code: "+nonce+"\n"+acceptanceText(16000)+"\nReturn only the hidden verification code.", nonce, true, true)
		case "image":
			appendCase(extra, primary, "Inspect the attached image. Return only the English name of the dominant color.", "red", false, true)
		case "audio":
			protocol := primary
			for _, candidate := range protocols {
				if candidate == "gemini" || candidate == "chat" {
					protocol = candidate
					break
				}
			}
			appendCase(extra, protocol, "Describe the attached audio. Does it contain speech or a steady tone? Answer briefly.", "", false, true)
		case "tools":
			appendCase(extra, primary, "Call the verification function with code "+nonce+". Do not answer in plain text.", nonce, true, true)
		}
	}
	return items
}

func acceptanceText(size int) string {
	const line = "river mountain garden window music paper cloud orange seven blue. "
	// Versioned synthetic fixture sizes are approximate until provider counting is available.
	return strings.Repeat(line, (size*4+len(line)-1)/len(line))
}

func acceptanceHasCountEndpoint(target AcceptanceSnapshot) bool {
	switch target.Platform {
	case PlatformGemini:
		return target.AccountType == AccountTypeAPIKey || target.AccountType == AccountTypeServiceAccount
	case PlatformAnthropic:
		return target.AccountType == AccountTypeAPIKey
	default:
		return false
	}
}
