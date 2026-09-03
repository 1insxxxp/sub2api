package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type lotteryAttemptGrantRepositoryStub struct {
	LotteryRepository
	input  LotteryAttemptGrantInput
	result LotteryAttemptGrantResult
}

func (s *lotteryAttemptGrantRepositoryStub) GrantLotteryAttempts(_ context.Context, input LotteryAttemptGrantInput) (LotteryAttemptGrantResult, error) {
	s.input = input
	return s.result, nil
}

func TestLotteryAttemptGrantValidatesAndDelegatesSelectedUsers(t *testing.T) {
	repo := &lotteryAttemptGrantRepositoryStub{result: LotteryAttemptGrantResult{Affected: 2, TotalGranted: 6}}
	svc := NewLotteryService(nil, repo, nil, nil)

	result, err := svc.GrantLotteryAttempts(context.Background(), LotteryAttemptGrantInput{
		UserIDs:     []int64{7, 8, 7},
		Amount:      3,
		Description: "活动补发",
		RequestKey:  "grant-selected-1",
		CreatedBy:   99,
	})

	require.NoError(t, err)
	require.Equal(t, LotteryAttemptGrantResult{Affected: 2, TotalGranted: 6}, *result)
	require.Equal(t, []int64{7, 8}, repo.input.UserIDs)
	require.Equal(t, 3, repo.input.Amount)
	require.Equal(t, "活动补发", repo.input.Description)
	require.Equal(t, "grant-selected-1", repo.input.RequestKey)
	require.Equal(t, int64(99), repo.input.CreatedBy)
}

func TestLotteryAttemptGrantAllowsAllUsersTarget(t *testing.T) {
	repo := &lotteryAttemptGrantRepositoryStub{result: LotteryAttemptGrantResult{Affected: 4, TotalGranted: 8}}
	svc := NewLotteryService(nil, repo, nil, nil)

	result, err := svc.GrantLotteryAttempts(context.Background(), LotteryAttemptGrantInput{
		All:        true,
		Amount:     2,
		RequestKey: "grant-all-1",
		CreatedBy:  99,
	})

	require.NoError(t, err)
	require.Equal(t, 4, result.Affected)
	require.True(t, repo.input.All)
	require.Empty(t, repo.input.UserIDs)
}

func TestLotteryAttemptGrantRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name  string
		input LotteryAttemptGrantInput
	}{
		{name: "missing target", input: LotteryAttemptGrantInput{Amount: 1, RequestKey: "invalid-1", CreatedBy: 1}},
		{name: "mixed targets", input: LotteryAttemptGrantInput{All: true, UserIDs: []int64{1}, Amount: 1, RequestKey: "invalid-2", CreatedBy: 1}},
		{name: "zero amount", input: LotteryAttemptGrantInput{All: true, Amount: 0, RequestKey: "invalid-3", CreatedBy: 1}},
		{name: "negative user", input: LotteryAttemptGrantInput{UserIDs: []int64{-1}, Amount: 1, RequestKey: "invalid-4", CreatedBy: 1}},
		{name: "missing creator", input: LotteryAttemptGrantInput{All: true, Amount: 1, RequestKey: "invalid-5"}},
		{name: "missing request key", input: LotteryAttemptGrantInput{All: true, Amount: 1, CreatedBy: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewLotteryService(nil, &lotteryAttemptGrantRepositoryStub{}, nil, nil)
			_, err := svc.GrantLotteryAttempts(context.Background(), tt.input)
			require.ErrorIs(t, err, ErrLotteryAttemptGrantInvalid)
		})
	}
}
