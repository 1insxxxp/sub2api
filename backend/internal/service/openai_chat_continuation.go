package service

import (
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func stripChatFinishReasonFromSSELine(line string) string {
	payload, ok := extractOpenAISSEDataLine(line)
	if !ok {
		return line
	}
	updated, err := sjson.SetRawBytes([]byte(payload), "choices.0.finish_reason", []byte("null"))
	if err != nil {
		return line
	}
	return "data: " + string(updated)
}

func aggregateCCStreamUsageSSELine(line string, usage OpenAIUsage) string {
	payload, ok := extractOpenAISSEDataLine(line)
	if !ok {
		return line
	}
	inputPath := "usage.prompt_tokens"
	if gjson.Get(payload, "usage.input_tokens").Exists() {
		inputPath = "usage.input_tokens"
	}
	outputPath := "usage.completion_tokens"
	if gjson.Get(payload, "usage.output_tokens").Exists() {
		outputPath = "usage.output_tokens"
	}
	updated, err := sjson.SetBytes([]byte(payload), inputPath, usage.InputTokens)
	if err != nil {
		return line
	}
	updated, err = sjson.SetBytes(updated, outputPath, usage.OutputTokens)
	if err != nil {
		return line
	}
	totalPath := "usage.total_tokens"
	updated, err = sjson.SetBytes(updated, totalPath, usage.InputTokens+usage.OutputTokens)
	if err != nil {
		return line
	}
	return "data: " + string(updated)
}

func buildCCStreamUsageSSELine(usage OpenAIUsage) string {
	return fmt.Sprintf("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d}}", usage.InputTokens, usage.OutputTokens, usage.InputTokens+usage.OutputTokens)
}

func hasCCStreamUsageLine(lines []string) bool {
	for _, line := range lines {
		if payload, ok := extractOpenAISSEDataLine(line); ok && gjson.Get(payload, "usage").Exists() {
			return true
		}
	}
	return false
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
