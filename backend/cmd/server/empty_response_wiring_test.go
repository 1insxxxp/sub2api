//go:build unit

package main

import (
	"os"
	"strings"
	"testing"
)

func TestApplicationWiresEmptyResponseCompensation(t *testing.T) {
	source, err := os.ReadFile("wire_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []string{
		"repository.NewEmptyResponseClaimRepository(",
		"service.NewEmptyResponseCompensationService(",
		"service.NewEmptyResponseClaimService(",
		"service.NewEmptyResponseClaimAdminService(",
		"handler.ProvideUsageHandler(",
		"handler.ProvideAdminUsageHandler(",
	} {
		t.Run(call, func(t *testing.T) {
			if !strings.Contains(string(source), call) {
				t.Fatalf("application startup is missing %s", call)
			}
		})
	}
}
