//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryUpsertResponseOutcomeLinksExistingUsageRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := newUsageLogRepositoryWithSQL(nil, db)
	groupID := int64(40)
	log := &service.UsageLog{
		RequestID: "req-outcome-1",
		APIKeyID:  20,
		UserID:    10,
		AccountID: 30,
		GroupID:   &groupID,
		Outcome: &service.ResponseOutcome{
			HTTPStatus:        200,
			UpstreamStatus:    200,
			HasText:           true,
			OutputBytes:       12,
			EventCount:        2,
			StreamCompleted:   true,
			FinishReason:      "stop",
			DisconnectSource:  service.DisconnectSourceNone,
			UpstreamErrorKind: service.UpstreamErrorNone,
			CollectorVersion:  1,
		},
	}

	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("INSERT INTO usage_response_outcomes")+`.*`+regexp.QuoteMeta("ul.id, $1::varchar(64)")+`.*`+regexp.QuoteMeta("WHERE ul.request_id = $1::varchar(64)")+`.*`+regexp.QuoteMeta("ON CONFLICT (request_id, api_key_id) DO UPDATE")).
		WithArgs(
			log.RequestID, log.APIKeyID, log.UserID, log.AccountID, log.GroupID,
			200, 200, true, false, false, false, int64(12), 2, true, "stop",
			string(service.DisconnectSourceNone), string(service.UpstreamErrorNone), int16(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.upsertResponseOutcome(context.Background(), log)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryUpsertResponseOutcomeRequiresLinkedUsageRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := newUsageLogRepositoryWithSQL(nil, db)
	log := &service.UsageLog{
		RequestID: "req-outcome-missing",
		APIKeyID:  2,
		UserID:    1,
		AccountID: 3,
		Outcome:   &service.ResponseOutcome{CollectorVersion: 1},
	}

	mock.ExpectExec("INSERT INTO usage_response_outcomes").
		WithArgs(
			log.RequestID, log.APIKeyID, log.UserID, log.AccountID, log.GroupID,
			0, 0, false, false, false, false, int64(0), 0, false, "",
			string(service.DisconnectSourceNone), string(service.UpstreamErrorNone), int16(1),
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.upsertResponseOutcome(context.Background(), log)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage log not found")
	require.NoError(t, mock.ExpectationsWereMet())
}
