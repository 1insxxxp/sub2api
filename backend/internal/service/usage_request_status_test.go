package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageRequestStatusRequiresTerminalEvidence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		outcome *ResponseOutcome
		want    UsageRequestStatus
	}{
		{"missing", nil, UsageRequestUnknown},
		{"metadata only", &ResponseOutcome{CollectorVersion: 0, HTTPStatus: 200, StreamCompleted: true}, UsageRequestUnknown},
		{"headers only", &ResponseOutcome{CollectorVersion: 1, HTTPStatus: 200, UpstreamStatus: 200}, UsageRequestUnknown},
		{"partial text", &ResponseOutcome{CollectorVersion: 1, HasText: true, HTTPStatus: 200, UpstreamStatus: 200}, UsageRequestUnknown},
		{"complete text", &ResponseOutcome{CollectorVersion: 1, HasText: true, HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true}, UsageRequestCompleted},
		{"complete tools", &ResponseOutcome{CollectorVersion: 1, HasToolCall: true, HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true}, UsageRequestCompleted},
		{"complete empty", &ResponseOutcome{CollectorVersion: 1, HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true}, UsageRequestEmpty},
		{"client cancelled after headers", &ResponseOutcome{CollectorVersion: 1, HTTPStatus: 200, DisconnectSource: DisconnectSourceClient}, UsageRequestInterrupted},
		{"terminal write failed", &ResponseOutcome{CollectorVersion: 1, HasText: true, StreamCompleted: true, DisconnectSource: DisconnectSourceClient}, UsageRequestInterrupted},
		{"upstream disconnected", &ResponseOutcome{CollectorVersion: 1, HasText: true, DisconnectSource: DisconnectSourceUpstream, UpstreamErrorKind: UpstreamErrorProtocol}, UsageRequestInterrupted},
		{"server deadline", &ResponseOutcome{CollectorVersion: 1, HasText: true, DisconnectSource: DisconnectSourceServer}, UsageRequestInterrupted},
		{"upstream 502", &ResponseOutcome{CollectorVersion: 1, UpstreamStatus: 502, UpstreamErrorKind: UpstreamErrorHTTP5xx}, UsageRequestFailed},
		{"http 500", &ResponseOutcome{CollectorVersion: 1, HTTPStatus: 500}, UsageRequestFailed},
		{"protocol error", &ResponseOutcome{CollectorVersion: 1, UpstreamErrorKind: UpstreamErrorProtocol}, UsageRequestFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ClassifyUsageRequestStatus(tc.outcome))
		})
	}
}
