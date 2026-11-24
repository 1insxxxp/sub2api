package service

import "testing"

func TestIsUpstreamToolSchemaError(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
		want bool
	}{
		{"missing tool parameter", 400, `{"error":{"message":"query is required for function tool"}}`, true},
		{"missing schema property", 400, `{"error":{"message":"invalid function parameters schema"}}`, true},
		{"client model validation", 400, `{"error":{"message":"model is required"}}`, false},
		{"server error", 500, `{"error":{"message":"invalid function parameters schema"}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUpstreamToolSchemaError(tt.code, []byte(tt.body)); got != tt.want {
				t.Fatalf("isUpstreamToolSchemaError() = %v, want %v", got, tt.want)
			}
		})
	}
}
