package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeModelSystemPromptsTrimsInput(t *testing.T) {
	got, err := NormalizeModelSystemPrompts(map[string]string{
		"  gpt-5.4  ": "  Stay in character.  ",
	})

	require.NoError(t, err)
	require.Equal(t, map[string]string{"gpt-5.4": "Stay in character."}, got)
}

func TestNormalizeModelSystemPromptsRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]string
	}{
		{name: "empty model", in: map[string]string{" ": "prompt"}},
		{name: "empty prompt", in: map[string]string{"gpt-5.4": " "}},
		{name: "duplicate normalized model", in: map[string]string{"gpt-5.4": "a", " gpt-5.4 ": "b"}},
		{name: "prompt too long", in: map[string]string{"gpt-5.4": strings.Repeat("x", MaxModelSystemPromptBytes+1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeModelSystemPrompts(tt.in)
			require.Error(t, err)
		})
	}
}

func TestNormalizeModelSystemPromptsReturnsNonNilEmptyMap(t *testing.T) {
	got, err := NormalizeModelSystemPrompts(nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}
