//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestUsageTokenProvenanceNotValidChecksStillConstrainNewWrites(t *testing.T) {
	ctx := context.Background()
	for _, column := range []string{"input_tokens_source", "output_tokens_source"} {
		constraint := "usage_response_outcomes_" + column + "_check"
		var validated bool
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT convalidated FROM pg_constraint WHERE conrelid = 'usage_response_outcomes'::regclass AND conname = $1`, constraint).Scan(&validated))
		require.False(t, validated, "migration must not scan historical outcomes while holding its DDL lock")
		for _, operation := range []string{"insert", "update"} {
			t.Run(column+"/"+operation, func(t *testing.T) {
				tx := testEntTx(t)
				client := tx.Client()
				user := mustCreateUser(t, client, &service.User{Email: "provenance-check@example.invalid"})
				key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-provenance-check", Name: "provenance check"})
				account := mustCreateAccount(t, client, &service.Account{Name: "provenance check"})
				_, err := tx.ExecContext(ctx, `INSERT INTO usage_response_outcomes (request_id, api_key_id, user_id, account_id) VALUES ('source-check-default', $1, $2, $3)`, key.ID, user.ID, account.ID)
				require.NoError(t, err, "omitted source fields retain valid legacy unknown defaults")
				rows, err := tx.QueryContext(ctx, `SELECT input_tokens_source, output_tokens_source FROM usage_response_outcomes WHERE request_id = 'source-check-default' AND api_key_id = $1`, key.ID)
				require.NoError(t, err)
				require.True(t, rows.Next())
				var inputSource, outputSource string
				require.NoError(t, rows.Scan(&inputSource, &outputSource))
				require.NoError(t, rows.Close())
				require.Equal(t, "unknown", inputSource)
				require.Equal(t, "unknown", outputSource)
				if operation == "insert" {
					_, err = tx.ExecContext(ctx, `INSERT INTO usage_response_outcomes (request_id, api_key_id, user_id, account_id, `+column+`) VALUES ('source-check-invalid', $1, $2, $3, 'invalid-source')`, key.ID, user.ID, account.ID)
				} else {
					_, err = tx.ExecContext(ctx, `UPDATE usage_response_outcomes SET `+column+` = 'invalid-source' WHERE request_id = 'source-check-default' AND api_key_id = $1`, key.ID)
				}
				var pgErr *pq.Error
				require.ErrorAs(t, err, &pgErr)
				require.Equal(t, pq.ErrorCode("23514"), pgErr.Code)
				require.Equal(t, constraint, pgErr.Constraint)
			})
		}
	}
}

func TestUsageTokenProvenanceReadPathsAndResponseEvidence(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)
	user := mustCreateUser(t, client, &service.User{Email: "provenance@example.invalid"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-provenance", Name: "provenance"})
	account := mustCreateAccount(t, client, &service.Account{Name: "provenance"})
	log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: "source-only-provenance", Model: "gemini-test", InputTokensSource: service.UsageTokenSourceUpstream, OutputTokensSource: service.UsageTokenSourceUnreported, CreatedAt: time.Now().UTC()}
	_, err := repo.Create(ctx, log)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertResponseOutcome(ctx, log))
	page := pagination.PaginationParams{Page: 1, PageSize: 10}
	start, end := log.CreatedAt.Add(-time.Second), log.CreatedAt.Add(time.Second)
	getters := map[string]func() ([]service.UsageLog, error){
		"detail": func() ([]service.UsageLog, error) {
			v, e := repo.GetByID(ctx, log.ID)
			if e != nil {
				return nil, e
			}
			return []service.UsageLog{*v}, nil
		},
		"filters": func() ([]service.UsageLog, error) {
			v, _, e := repo.ListWithFilters(ctx, page, usagestats.UsageLogFilters{UserID: user.ID})
			return v, e
		},
		"user":    func() ([]service.UsageLog, error) { v, _, e := repo.ListByUser(ctx, user.ID, page); return v, e },
		"key":     func() ([]service.UsageLog, error) { v, _, e := repo.ListByAPIKey(ctx, key.ID, page); return v, e },
		"account": func() ([]service.UsageLog, error) { v, _, e := repo.ListByAccount(ctx, account.ID, page); return v, e },
		"user time": func() ([]service.UsageLog, error) {
			v, _, e := repo.ListByUserAndTimeRange(ctx, user.ID, start, end)
			return v, e
		},
		"key time": func() ([]service.UsageLog, error) {
			v, _, e := repo.ListByAPIKeyAndTimeRange(ctx, key.ID, start, end)
			return v, e
		},
		"account time": func() ([]service.UsageLog, error) {
			v, _, e := repo.ListByAccountAndTimeRange(ctx, account.ID, start, end)
			return v, e
		},
		"model time": func() ([]service.UsageLog, error) {
			v, _, e := repo.ListByModelAndTimeRange(ctx, log.Model, start, end)
			return v, e
		},
	}
	for name, get := range getters {
		t.Run(name, func(t *testing.T) {
			logs, e := get()
			require.NoError(t, e)
			require.Len(t, logs, 1)
			require.Equal(t, service.UsageTokenSourceUpstream, logs[0].InputTokensSource)
			require.Equal(t, service.UsageTokenSourceUnreported, logs[0].OutputTokensSource)
			require.Nil(t, logs[0].Outcome)
		})
	}
	// Retries may add a real outcome, but an older metadata-only write must never erase it.
	log.Outcome = &service.ResponseOutcome{HTTPStatus: 200, UpstreamStatus: 200, HasText: true, StreamCompleted: true, CollectorVersion: 1}
	require.NoError(t, repo.UpsertResponseOutcome(ctx, log))
	log.Outcome = nil
	require.NoError(t, repo.UpsertResponseOutcome(ctx, log))
	got, err := repo.GetByID(ctx, log.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Outcome, "metadata-only retries must retain existing response evidence")
	require.True(t, got.Outcome.HasText)
	require.True(t, got.Outcome.StreamCompleted)
}

func TestUsageTokenProvenanceDoesNotChangeSuccessfulSyncAnalytics(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)
	user := mustCreateUser(t, client, &service.User{Email: "provenance-analytics@example.invalid"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-provenance-analytics", Name: "provenance"})
	account := mustCreateAccount(t, client, &service.Account{Name: "provenance"})
	group := mustCreateGroup(t, client, &service.Group{Name: "provenance analytics", Platform: service.PlatformGemini})
	now := time.Now().UTC()
	log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &group.ID, RequestID: "source-only-analytics", Model: "gemini-test", OutputTokens: 7, ActualCost: 0.7, InputTokensSource: service.UsageTokenSourceUpstream, OutputTokensSource: service.UsageTokenSourceUpstream, CreatedAt: now, Stream: false}
	_, err := repo.Create(ctx, log)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertResponseOutcome(ctx, log))
	t.Run("heatmap", func(t *testing.T) {
		days, err := repo.GetUserActivityHeatmap(ctx, now.Add(-time.Second), now.Add(time.Second), usagestats.UsageLogFilters{UserID: user.ID})
		require.NoError(t, err)
		require.Len(t, days, 1)
		require.EqualValues(t, 1, days[0].SuccessRequests)
		require.EqualValues(t, 7, days[0].OutputTokens)
		require.EqualValues(t, 7, days[0].TotalTokens)
		require.InDelta(t, 0.7, days[0].BilledCost, 1e-10)
	})
	t.Run("model health", func(t *testing.T) {
		scopes, err := json.Marshal([]service.ModelStatusScope{{GroupID: group.ID, Platform: service.PlatformGemini, Model: log.Model}})
		require.NoError(t, err)
		rows, err := tx.QueryContext(ctx, modelStatusAggregateSQL, string(scopes), now.Add(time.Second), service.ModelStatusRecentLimit)
		require.NoError(t, err)
		defer rows.Close()
		require.True(t, rows.Next())
		var groupID, total, success, failure, empty, unknown, ttftSamples, durationSamples int64
		var platform, model string
		var ttft, duration sql.NullFloat64
		var recent, buckets []byte
		var needsMore bool
		require.NoError(t, rows.Scan(&groupID, &platform, &model, &total, &success, &failure, &empty, &unknown, &ttft, &duration, &ttftSamples, &durationSamples, &recent, &buckets, &needsMore))
		require.EqualValues(t, 1, total)
		require.EqualValues(t, 1, success)
		require.Zero(t, empty)
		require.Zero(t, failure)
	})
	t.Run("claim evidence", func(t *testing.T) {
		rows, err := tx.QueryContext(ctx, `SELECT id, collector_version FROM usage_response_outcomes WHERE request_id = $1 AND api_key_id = $2`, log.RequestID, key.ID)
		require.NoError(t, err)
		require.True(t, rows.Next())
		var metadataID int64
		var collectorVersion int
		require.NoError(t, rows.Scan(&metadataID, &collectorVersion))
		require.NoError(t, rows.Close())
		require.Zero(t, collectorVersion, "source-only metadata must not become collected response evidence")

		evaluation, err := newEmptyResponseClaimRepositoryWithSQL(tx).LoadEvaluation(ctx, user.ID, log.ID)
		require.NoError(t, err)
		// The metadata row supplies provenance and its reference, while only a
		// positive collector version may supply actual response evidence.
		require.NotNil(t, evaluation.OutcomeID)
		require.Equal(t, metadataID, *evaluation.OutcomeID)
		require.Equal(t, service.UsageTokenSourceUpstream, evaluation.Usage.InputTokensSource)
		require.Equal(t, service.UsageTokenSourceUpstream, evaluation.Usage.OutputTokensSource)
		require.Nil(t, evaluation.Outcome)
	})
}
