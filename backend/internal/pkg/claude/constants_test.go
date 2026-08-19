package claude

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolCapableBetaHeadersIncludeFineGrainedToolStreaming(t *testing.T) {
	requireBetaToken(t, MessageBetaHeaderWithTools, BetaFineGrainedToolStreaming)
	require.Contains(t, FullClaudeCodeMimicryBetas(), BetaFineGrainedToolStreaming)
}

func requireBetaToken(t *testing.T, header string, token string) {
	t.Helper()
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == token {
			return
		}
	}
	require.Failf(t, "missing beta token", "header %q does not contain %q", header, token)
}
