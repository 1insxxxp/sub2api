package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseUsageRequestType(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		input   string
		want    RequestType
		wantErr bool
	}

	cases := []testCase{
		{name: "unknown", input: "unknown", want: RequestTypeUnknown},
		{name: "sync", input: "sync", want: RequestTypeSync},
		{name: "stream", input: "stream", want: RequestTypeStream},
		{name: "ws_v2", input: "ws_v2", want: RequestTypeWSV2},
		{name: "case_insensitive", input: "WS_V2", want: RequestTypeWSV2},
		{name: "trim_spaces", input: "  stream  ", want: RequestTypeStream},
		{name: "invalid", input: "xxx", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseUsageRequestType(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRequestTypeNormalizeAndString(t *testing.T) {
	t.Parallel()

	require.Equal(t, RequestTypeUnknown, RequestType(99).Normalize())
	require.Equal(t, "unknown", RequestType(99).String())
	require.Equal(t, "sync", RequestTypeSync.String())
	require.Equal(t, "stream", RequestTypeStream.String())
	require.Equal(t, "ws_v2", RequestTypeWSV2.String())
}

func TestRequestTypeFromLegacy(t *testing.T) {
	t.Parallel()

	require.Equal(t, RequestTypeWSV2, RequestTypeFromLegacy(false, true))
	require.Equal(t, RequestTypeStream, RequestTypeFromLegacy(true, false))
	require.Equal(t, RequestTypeSync, RequestTypeFromLegacy(false, false))
}

func TestApplyLegacyRequestFields(t *testing.T) {
	t.Parallel()

	stream, ws := ApplyLegacyRequestFields(RequestTypeSync, true, true)
	require.False(t, stream)
	require.False(t, ws)

	stream, ws = ApplyLegacyRequestFields(RequestTypeStream, false, true)
	require.True(t, stream)
	require.False(t, ws)

	stream, ws = ApplyLegacyRequestFields(RequestTypeWSV2, false, false)
	require.True(t, stream)
	require.True(t, ws)

	stream, ws = ApplyLegacyRequestFields(RequestTypeUnknown, true, false)
	require.True(t, stream)
	require.False(t, ws)
}

func TestUsageLogSyncRequestTypeAndLegacyFields(t *testing.T) {
	t.Parallel()

	log := &UsageLog{RequestType: RequestTypeWSV2, Stream: false, OpenAIWSMode: false}
	log.SyncRequestTypeAndLegacyFields()

	require.Equal(t, RequestTypeWSV2, log.RequestType)
	require.True(t, log.Stream)
	require.True(t, log.OpenAIWSMode)
}

func TestUsageLogEffectiveRequestTypeFallback(t *testing.T) {
	t.Parallel()

	log := &UsageLog{RequestType: RequestTypeUnknown, Stream: true, OpenAIWSMode: true}
	require.Equal(t, RequestTypeWSV2, log.EffectiveRequestType())
}

func TestUsageLogEffectiveRequestTypeNilReceiver(t *testing.T) {
	t.Parallel()

	var log *UsageLog
	require.Equal(t, RequestTypeUnknown, log.EffectiveRequestType())
}

func TestUsageLogSyncRequestTypeAndLegacyFieldsNilReceiver(t *testing.T) {
	t.Parallel()

	var log *UsageLog
	log.SyncRequestTypeAndLegacyFields()
}

func TestProjectUsageLogCompensationAllowsVisibleLowTokenOutput(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	log := &UsageLog{
		ActualCost:   1.25,
		OutputTokens: EmptyResponseClaimLowOutputTokenLimit,
		CreatedAt:    now.Add(-6 * 24 * time.Hour),
		Group:        &Group{EmptyResponseCompensationEnabled: true},
		Outcome: &ResponseOutcome{
			HTTPStatus:       200,
			UpstreamStatus:   200,
			HasText:          true,
			StreamCompleted:  true,
			CollectorVersion: ResponseOutcomeCollectorVersion,
		},
	}

	projectUsageLogCompensation(log, now)

	require.True(t, log.CompensationEligible)
	require.Equal(t, UsageCompensationEligible, log.CompensationEligibility)
	require.Equal(t, EmptyResponseReasonLowOutput, log.CompensationReasonCode)
}

func TestProjectUsageLogCompensationExpiresAfterSevenDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	log := &UsageLog{
		ActualCost:   1.25,
		OutputTokens: 0,
		CreatedAt:    now.Add(-EmptyResponseClaimWindow - time.Second),
		Group:        &Group{EmptyResponseCompensationEnabled: true},
		Outcome: &ResponseOutcome{
			HTTPStatus:       200,
			UpstreamStatus:   200,
			StreamCompleted:  true,
			CollectorVersion: ResponseOutcomeCollectorVersion,
		},
	}

	projectUsageLogCompensation(log, now)

	require.False(t, log.CompensationEligible)
	require.Equal(t, UsageCompensationUnavailable, log.CompensationEligibility)
	require.Equal(t, EmptyResponseReasonWindowExpired, log.CompensationReasonCode)
}
