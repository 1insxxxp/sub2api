//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEmptyResponseClaimProvenancePaginationMatchesEligibility(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	logs := newUsageLogRepositoryWithSQL(client, tx)
	claims := newEmptyResponseClaimRepositoryWithSQL(tx)
	user := mustCreateUser(t, client, &service.User{Email: "claim-provenance-page@example.invalid"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-claim-provenance-page", Name: "claim provenance"})
	account := mustCreateAccount(t, client, &service.Account{Name: "claim provenance"})
	group := mustCreateGroup(t, client, &service.Group{Name: "claim provenance", Platform: service.PlatformOpenAI, EmptyResponseCompensationEnabled: true})
	_, err := tx.ExecContext(ctx, `UPDATE groups SET empty_response_compensation_enabled = TRUE WHERE id = $1`, group.ID)
	require.NoError(t, err)
	now := time.Now().UTC()
	pure := func() *service.ResponseOutcome {
		return &service.ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, StreamCompleted: true, CollectorVersion: 1}
	}
	text := pure()
	text.HasText = true
	reasoning := pure()
	reasoning.HasReasoning = true
	canceled := pure()
	canceled.DisconnectSource = service.DisconnectSourceClient
	failed := pure()
	failed.UpstreamStatus = 500
	metadataOnly := pure()
	metadataOnly.CollectorVersion = 0
	cases := []struct {
		name    string
		source  service.UsageTokenSource
		output  int
		outcome *service.ResponseOutcome
		claim   string
		visible bool
	}{
		{"missing metadata", service.UsageTokenSourceUnreported, 0, nil, "", false},
		{"estimated zero", service.UsageTokenSourceEstimated, 0, nil, "", false},
		{"missing with visible text", service.UsageTokenSourceUnreported, 0, text, "", false},
		{"measured zero", service.UsageTokenSourceUpstream, 0, nil, "", true},
		{"legacy unknown zero", service.UsageTokenSourceUnknown, 0, nil, "", true},
		{"estimated nonzero", service.UsageTokenSourceEstimated, 7, nil, "", true},
		{"missing with pure empty", service.UsageTokenSourceUnreported, 0, pure(), "", true},
		{"estimated zero pure empty", service.UsageTokenSourceEstimated, 0, pure(), "", true},
		{"missing existing visible claim", service.UsageTokenSourceUnreported, 0, nil, service.EmptyResponseReasonLowOutput, true},
		{"missing existing hidden claim", service.UsageTokenSourceUnreported, 0, nil, service.EmptyResponseReasonMissingEvidence, false},
		{"missing with reasoning", service.UsageTokenSourceUnreported, 0, reasoning, "", false},
		{"missing client canceled", service.UsageTokenSourceUnreported, 0, canceled, "", false},
		{"missing upstream failure", service.UsageTokenSourceUnreported, 0, failed, "", false},
		{"metadata only is not empty evidence", service.UsageTokenSourceUnreported, 0, metadataOnly, "", false},
	}
	var expectedIDs []int64
	for i, tc := range cases {
		log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &group.ID,
			RequestID: fmt.Sprintf("claim-provenance-page-%d", i), Model: "claim-test", InputTokens: 100,
			OutputTokens: tc.output, ActualCost: 1, InputTokensSource: service.UsageTokenSourceUpstream,
			OutputTokensSource: tc.source, Outcome: tc.outcome, CreatedAt: now.Add(-time.Duration(i+1) * time.Second)}
		_, err := logs.Create(ctx, log)
		require.NoError(t, err, tc.name)
		require.NoError(t, logs.UpsertResponseOutcome(ctx, log), tc.name)
		if tc.outcome != nil && tc.outcome.CollectorVersion == 0 {
			// Upsert treats a non-nil Outcome as a current collector and promotes
			// version zero. Seed the persisted metadata-only edge explicitly.
			_, err = tx.ExecContext(ctx, `UPDATE usage_response_outcomes SET collector_version = 0 WHERE usage_log_id = $1`, log.ID)
			require.NoError(t, err, tc.name)
			evaluation, err := claims.LoadEvaluation(ctx, user.ID, log.ID)
			require.NoError(t, err, tc.name)
			require.Nil(t, evaluation.Outcome, "fixture must remain metadata-only despite its completion flags")
		}
		if tc.claim != "" {
			_, err = tx.ExecContext(ctx, `INSERT INTO empty_response_claims (usage_log_id, user_id, api_key_id, account_id, group_id, status, reason_code, original_actual_cost) VALUES ($1,$2,$3,$4,$5,'approved',$6,1)`, log.ID, user.ID, key.ID, account.ID, group.ID, tc.claim)
			require.NoError(t, err, tc.name)
		}
		if tc.visible {
			expectedIDs = append(expectedIDs, log.ID)
		}
	}
	var actualIDs []int64
	for page := 1; page <= len(expectedIDs)+1; page++ {
		candidates, info, err := claims.ListRecentEvaluations(ctx, user.ID, now.Add(-time.Hour), now, pagination.PaginationParams{Page: page, PageSize: 1})
		require.NoError(t, err)
		require.EqualValues(t, len(expectedIDs), info.Total, "count must exclude rows the service cannot show")
		require.Equal(t, len(expectedIDs), info.Pages)
		if page <= len(expectedIDs) {
			require.Len(t, candidates, 1, "eligible entries must not be shifted behind missing measurement rows")
		} else {
			require.Empty(t, candidates)
		}
		for _, candidate := range candidates {
			actualIDs = append(actualIDs, candidate.Evaluation.Usage.ID)
			if candidate.ClaimID == nil {
				decision := service.EvaluateEmptyResponseClaim(now, candidate.Evaluation.Usage, candidate.Evaluation.Outcome, candidate.Evaluation.Group, 0)
				require.Equal(t, service.EmptyResponseClaimApproved, decision.Status, "repository must not paginate an invisible candidate")
			}
		}
	}
	require.Equal(t, expectedIDs, actualIDs)
}
