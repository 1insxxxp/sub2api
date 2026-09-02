//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsKnownProviderRequestCompatibilityError(t *testing.T) {
	t.Run("invalid tool use format triggers failover", func(t *testing.T) {
		body := []byte(`{"error":{"message":"API error (HTTP 400): {\"message\":\"Invalid tool use format.\",\"reason\":\"REQUEST_BODY_INVALID\"}"}}`)
		require.True(t, isKnownProviderRequestCompatibilityError(body))
	})

	t.Run("generic invalid request does not trigger failover", func(t *testing.T) {
		body := []byte(`{"error":{"message":"max_tokens must be greater than zero","type":"invalid_request_error"}}`)
		require.False(t, isKnownProviderRequestCompatibilityError(body))
	})

	t.Run("unrelated tool validation error does not bypass the configured gate", func(t *testing.T) {
		body := []byte(`{"error":{"message":"tools.0.input_schema is required","type":"invalid_request_error"}}`)
		require.False(t, isKnownProviderRequestCompatibilityError(body))
	})
}

func TestShouldFailoverBadRequest(t *testing.T) {
	knownCompatibilityError := []byte(`{"error":{"message":"API error (HTTP 400): {\"message\":\"Invalid tool use format.\",\"reason\":\"REQUEST_BODY_INVALID\"}"}}`)
	genericToolError := []byte(`{"error":{"message":"tool_use input is invalid","type":"invalid_request_error"}}`)

	t.Run("known compatibility error bypasses disabled gate", func(t *testing.T) {
		svc := &GatewayService{cfg: &config.Config{}}
		require.True(t, svc.shouldFailoverBadRequest(knownCompatibilityError))
	})

	t.Run("known compatibility error works without config", func(t *testing.T) {
		svc := &GatewayService{}
		require.True(t, svc.shouldFailoverBadRequest(knownCompatibilityError))
	})

	t.Run("generic tool error respects disabled gate", func(t *testing.T) {
		svc := &GatewayService{cfg: &config.Config{}}
		require.False(t, svc.shouldFailoverBadRequest(genericToolError))
	})

	t.Run("generic tool error follows enabled gate", func(t *testing.T) {
		svc := &GatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{FailoverOn400: true}}}
		require.True(t, svc.shouldFailoverBadRequest(genericToolError))
	})
}
