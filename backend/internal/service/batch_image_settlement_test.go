//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchImageSettlementService_SettlesAndChargesSuccessfulImagesOnly(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_settle")
	job.SuccessCount = 3
	job.FailCount = 2
	job.ItemCount = 5
	job.SessionID = batchImageStringPtr("batch-settlement-session")
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{captureResult: &BatchImageBalanceHoldResult{
		Applied: true, ThresholdExemptCost: 0.5,
	}}
	usageLogs := &batchSettlementUsageLogRepo{}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25},
		UsageLogRepo: usageLogs,
	}

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.Equal(t, 0.75, result.ActualCost)
	require.Equal(t, BatchImageCaptureRequestID(job.BatchID), result.RequestID)
	require.False(t, result.AlreadySettled)
	require.Equal(t, BatchImageJobStatusCompleted, repo.jobs[job.BatchID].Status)
	require.NotNil(t, repo.jobs[job.BatchID].ActualCost)
	require.Equal(t, 0.75, *repo.jobs[job.BatchID].ActualCost)
	require.NotEmpty(t, batchImageDerefString(repo.jobs[job.BatchID].ManifestHash))
	require.NotNil(t, repo.jobs[job.BatchID].SettledAt)
	require.Equal(t, "batch-settlement-session", batchImageDerefString(usageLogs.lastLog.SessionID))
	require.Equal(t, 0.75, usageLogs.lastLog.ActualCost)
	require.Equal(t, 0.5, usageLogs.lastLog.ThresholdExemptCost)
	require.Len(t, billing.captures, 1)
	require.Equal(t, int64(321), billing.captures[0].APIKeyID)
	require.Equal(t, job.UserID, billing.captures[0].UserID)
	require.Equal(t, job.BatchID, billing.captures[0].BatchID)
	require.Equal(t, 0.75, billing.captures[0].ActualAmount)
	require.Equal(t, 1.25, billing.captures[0].HoldAmount)
	require.NotContains(t, fmt.Sprintf("%+v", billing.captures[0]), batchImageTestData)
	require.NotContains(t, fmt.Sprintf("%+v", billing.captures[0]), "gs://")
	require.NotContains(t, fmt.Sprintf("%+v", billing.captures[0]), "prompt")

	duplicate, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.True(t, duplicate.AlreadySettled)
	require.Len(t, billing.captures, 1)
	require.Equal(t, 1, usageLogs.calls)
}

func TestBatchImageSettlementService_UsesOneQuantizedActualCostEverywhere(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_quantized_cost")
	job.SuccessCount = 1
	job.FailCount = 0
	job.ItemCount = 1
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{captureResult: &BatchImageBalanceHoldResult{
		Applied: true, ThresholdExemptCost: 0.00007813,
	}}
	usageLogs := &batchSettlementUsageLogRepo{}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.000078125},
		UsageLogRepo: usageLogs,
	}

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.Equal(t, 0.00007813, result.ActualCost)
	require.Len(t, billing.captures, 1)
	require.Equal(t, result.ActualCost, billing.captures[0].ActualAmount)
	require.Equal(t, result.ActualCost, usageLogs.lastLog.ActualCost)
	require.Equal(t, result.ActualCost, usageLogs.lastLog.ThresholdExemptCost)
	require.NotNil(t, repo.jobs[job.BatchID].ActualCost)
	require.Equal(t, result.ActualCost, *repo.jobs[job.BatchID].ActualCost)
}

type batchSettlementUsageLogRepo struct {
	UsageLogRepository
	lastLog   *UsageLog
	calls     int
	errOnce   error
	errAlways error
	persisted map[string]bool
}

func (r *batchSettlementUsageLogRepo) Create(_ context.Context, log *UsageLog) (bool, error) {
	r.calls++
	r.lastLog = log
	if r.errAlways != nil {
		return false, r.errAlways
	}
	if r.errOnce != nil {
		err := r.errOnce
		r.errOnce = nil
		return false, err
	}
	if r.persisted == nil {
		r.persisted = make(map[string]bool)
	}
	key := fmt.Sprintf("%s:%d", log.RequestID, log.APIKeyID)
	if r.persisted[key] {
		return false, nil
	}
	r.persisted[key] = true
	return true, nil
}

func (r *batchSettlementUsageLogRepo) CreateBestEffort(_ context.Context, log *UsageLog) error {
	r.calls++
	r.lastLog = log
	return nil
}

func TestBatchImageSettlementService_ZeroSuccessCanComplete(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_zero")
	job.SuccessCount = 0
	job.FailCount = 4
	job.ItemCount = 4
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.Equal(t, 0.0, result.ActualCost)
	require.Equal(t, BatchImageJobStatusCompleted, repo.jobs[job.BatchID].Status)
	require.Len(t, billing.captures, 1)
	require.Equal(t, 0.0, billing.captures[0].ActualAmount)
}

func TestBatchImageSettlementService_CompletedJobReturnsAlreadySettledWithoutBilling(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_done")
	job.Status = BatchImageJobStatusCompleted
	cost := 0.5
	job.ActualCost = &cost
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.True(t, result.AlreadySettled)
	require.Equal(t, 0.5, result.ActualCost)
	require.Empty(t, billing.captures)
}

type batchImageAlreadySettledOnFinalizeRepo struct {
	*fakeBatchImageRepository
	failureCalls int
}

func (r *batchImageAlreadySettledOnFinalizeRepo) MarkBatchImageJobSettled(_ context.Context, params MarkBatchImageJobSettledParams) error {
	job := r.jobs[params.BatchID]
	job.Status = BatchImageJobStatusCompleted
	job.ActualCost = &params.ActualCost
	return ErrBatchImageAlreadySettled
}

func (r *batchImageAlreadySettledOnFinalizeRepo) SetBatchImageJobSettlementFailed(ctx context.Context, batchID, code, message string) (int, error) {
	r.failureCalls++
	return r.fakeBatchImageRepository.SetBatchImageJobSettlementFailed(ctx, batchID, code, message)
}

func TestBatchImageSettlementService_ConcurrentFinalizeTreatsAlreadySettledAsSuccess(t *testing.T) {
	base := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_concurrent_finalize")
	base.jobs[job.BatchID] = job
	repo := &batchImageAlreadySettledOnFinalizeRepo{fakeBatchImageRepository: base}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: &fakeBatchImageBillingRepo{}, UsageLogRepo: &batchSettlementUsageLogRepo{},
		Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25},
	}

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.True(t, result.AlreadySettled)
	require.Equal(t, BatchImageJobStatusCompleted, base.jobs[job.BatchID].Status)
	require.Zero(t, repo.failureCalls)
}

func TestBatchImageSettlementService_IdempotentAfterBillingCrash(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_crash")
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{alreadyApplied: map[string]bool{BatchImageCaptureRequestID(job.BatchID): true}}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.Equal(t, 0.5, result.ActualCost)
	require.Equal(t, BatchImageJobStatusCompleted, repo.jobs[job.BatchID].Status)
	require.Len(t, billing.captures, 1)
}

func TestBatchImageSettlementService_RetryRestoresCaptureGiftAttributionAfterMarkFailure(t *testing.T) {
	baseRepo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_mark_retry_gift")
	baseRepo.jobs[job.BatchID] = job
	markErr := errors.New("mark settled failed")
	repo := &failOnceMarkBatchImageRepository{fakeBatchImageRepository: baseRepo, err: markErr}
	billing := &fakeBatchImageBillingRepo{captureResult: &BatchImageBalanceHoldResult{ThresholdExemptCost: 0.4}}
	usageLogs := &batchSettlementUsageLogRepo{}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}, UsageLogRepo: usageLogs,
	}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, markErr)
	require.Equal(t, BatchImageJobStatusSettling, baseRepo.jobs[job.BatchID].Status)
	require.Len(t, billing.captures, 1)
	require.Equal(t, 1, usageLogs.calls)
	require.Equal(t, 0.4, usageLogs.lastLog.ThresholdExemptCost)

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.False(t, result.AlreadySettled)
	require.Equal(t, BatchImageJobStatusCompleted, baseRepo.jobs[job.BatchID].Status)
	require.Len(t, billing.captures, 2)
	require.Equal(t, 2, usageLogs.calls)
	require.Equal(t, 0.5, usageLogs.lastLog.ActualCost)
	require.Equal(t, 0.4, usageLogs.lastLog.ThresholdExemptCost)

	duplicate, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.True(t, duplicate.AlreadySettled)
	require.Len(t, billing.captures, 2)
	require.Equal(t, 2, usageLogs.calls)
}

func TestBatchImageSettlementService_RetryRestoresCaptureGiftAttributionAfterUsageLogFailure(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_log_retry_gift")
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{captureResult: &BatchImageBalanceHoldResult{ThresholdExemptCost: 0.4}}
	logErr := errors.New("usage log failed")
	usageLogs := &batchSettlementUsageLogRepo{errOnce: logErr}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}, UsageLogRepo: usageLogs,
	}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, logErr)
	require.Equal(t, BatchImageJobStatusSettling, repo.jobs[job.BatchID].Status)
	require.Equal(t, 1, repo.jobs[job.BatchID].RetryCount)
	require.Equal(t, "SETTLEMENT_USAGE_LOG_FAILED", batchImageDerefString(repo.jobs[job.BatchID].LastErrorCode))
	require.Len(t, billing.captures, 1)
	require.Equal(t, 1, usageLogs.calls)

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.False(t, result.AlreadySettled)
	require.Equal(t, BatchImageJobStatusCompleted, repo.jobs[job.BatchID].Status)
	require.Len(t, billing.captures, 2)
	require.Equal(t, 2, usageLogs.calls)
	require.Equal(t, 0.5, usageLogs.lastLog.ActualCost)
	require.Equal(t, 0.4, usageLogs.lastLog.ThresholdExemptCost)

	duplicate, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.True(t, duplicate.AlreadySettled)
	require.Len(t, billing.captures, 2)
	require.Equal(t, 2, usageLogs.calls)
}

func TestBatchImageSettlementService_MissingUsageLogRepositoryFailsBeforeCapture(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_missing_usage_repo")
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25},
	}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
	require.Empty(t, billing.captures)
	require.Equal(t, BatchImageJobStatusSettling, repo.jobs[job.BatchID].Status)
}

func TestBatchImageSettlementService_UsageLogRetryExhaustionNeverRefundsCapturedHold(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_usage_log_exhausted")
	job.RetryCount = batchImageSettlementMaxRetries - 1
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{captureResult: &BatchImageBalanceHoldResult{ThresholdExemptCost: 0.4}}
	usageLogs := &batchSettlementUsageLogRepo{errAlways: errors.New("usage log unavailable")}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}, UsageLogRepo: usageLogs,
	}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
	require.Equal(t, BatchImageJobStatusFailed, repo.jobs[job.BatchID].Status)
	require.Equal(t, "SETTLEMENT_USAGE_LOG_RETRY_EXHAUSTED", batchImageDerefString(repo.jobs[job.BatchID].LastErrorCode))
	require.Len(t, billing.captures, 1)
	require.Empty(t, billing.releases, "a captured hold is already charged and must never be refunded")
}

func TestBatchImageSettlementService_FinalizeRetryExhaustionNeverRefundsCapturedHold(t *testing.T) {
	baseRepo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_finalize_exhausted")
	job.RetryCount = batchImageSettlementMaxRetries - 1
	baseRepo.jobs[job.BatchID] = job
	repo := &failOnceMarkBatchImageRepository{fakeBatchImageRepository: baseRepo, err: errors.New("mark settled unavailable")}
	billing := &fakeBatchImageBillingRepo{captureResult: &BatchImageBalanceHoldResult{ThresholdExemptCost: 0.4}}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25},
		UsageLogRepo: &batchSettlementUsageLogRepo{},
	}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
	require.Equal(t, BatchImageJobStatusFailed, baseRepo.jobs[job.BatchID].Status)
	require.Equal(t, "SETTLEMENT_FINALIZE_RETRY_EXHAUSTED", batchImageDerefString(baseRepo.jobs[job.BatchID].LastErrorCode))
	require.Len(t, billing.captures, 1)
	require.Empty(t, billing.releases, "a captured hold is already charged and must never be refunded")
}

func TestBatchImageSettlementService_PreviouslyExhaustedUsageLogFailureNeverRefunds(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_usage_log_pre_exhausted")
	job.RetryCount = batchImageSettlementMaxRetries
	job.LastErrorCode = batchImageStringPtr("SETTLEMENT_USAGE_LOG_FAILED")
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{
		Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25},
		UsageLogRepo: &batchSettlementUsageLogRepo{},
	}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
	require.Equal(t, BatchImageJobStatusFailed, repo.jobs[job.BatchID].Status)
	require.Equal(t, "SETTLEMENT_USAGE_LOG_RETRY_EXHAUSTED", batchImageDerefString(repo.jobs[job.BatchID].LastErrorCode))
	require.Empty(t, billing.captures)
	require.Empty(t, billing.releases)
}

type failOnceMarkBatchImageRepository struct {
	*fakeBatchImageRepository
	err error
}

func (r *failOnceMarkBatchImageRepository) MarkBatchImageJobSettled(ctx context.Context, params MarkBatchImageJobSettledParams) error {
	if r.err != nil {
		err := r.err
		r.err = nil
		return err
	}
	return r.fakeBatchImageRepository.MarkBatchImageJobSettled(ctx, params)
}

func TestBatchImageSettlementService_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*BatchImageJob)
		pricing BatchImagePricingResolver
		want    error
	}{
		{name: "invalid_status", mutate: func(j *BatchImageJob) { j.Status = BatchImageJobStatusRunning }, want: ErrBatchImageSettlementInvalidStatus},
		{name: "negative_success_count", mutate: func(j *BatchImageJob) { j.SuccessCount = -1 }, want: ErrBatchImageSettlementInvalidCounts},
		{name: "negative_fail_count", mutate: func(j *BatchImageJob) { j.FailCount = -1 }, want: ErrBatchImageSettlementInvalidCounts},
		{name: "counts_exceed_item_count", mutate: func(j *BatchImageJob) { j.SuccessCount = 2; j.FailCount = 2; j.ItemCount = 3 }, want: ErrBatchImageSettlementInvalidCounts},
		{name: "missing_api_key", mutate: func(j *BatchImageJob) { j.APIKeyID = nil }, want: ErrBatchImageSettlementMissingAPIKeyID},
		{name: "missing_account", mutate: func(j *BatchImageJob) { j.AccountID = nil }, want: ErrBatchImageSettlementMissingAccountID},
		{name: "pricing_missing", pricing: &fakeBatchImagePricingResolver{err: ErrBatchImageSettlementPricingMissing}, want: ErrBatchImageSettlementPricingMissing},
		{name: "manifest_conflict", mutate: func(j *BatchImageJob) { v := "different"; j.ManifestHash = &v }, want: ErrBatchImageSettlementManifestConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeBatchImageRepository()
			job := testSettlingBatchImageJob("imgbatch_" + tt.name)
			if tt.mutate != nil {
				tt.mutate(job)
			}
			repo.jobs[job.BatchID] = job
			pricing := tt.pricing
			if pricing == nil {
				pricing = &fakeBatchImagePricingResolver{unitPrice: 0.25}
			}
			billing := &fakeBatchImageBillingRepo{}
			svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, Pricing: pricing}

			_, err := svc.Settle(context.Background(), job.BatchID)
			require.ErrorIs(t, err, tt.want)
			require.Empty(t, billing.captures)
			require.NotEqual(t, BatchImageJobStatusCompleted, repo.jobs[job.BatchID].Status)
		})
	}
}

func TestBatchImageSettlementService_CostExceedingHoldDoesNotCharge(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_cost_over_hold")
	job.SuccessCount = 2
	job.FailCount = 0
	job.ItemCount = 2
	holdAmount := 0.5
	job.HoldAmount = &holdAmount
	job.EstimatedCost = holdAmount
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.50}}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementCostExceedsHold)
	require.Empty(t, billing.captures)
	require.Equal(t, BatchImageJobStatusSettling, repo.jobs[job.BatchID].Status)
	require.Equal(t, "SETTLEMENT_COST_EXCEEDS_HOLD", batchImageDerefString(repo.jobs[job.BatchID].LastErrorCode))
}

func TestBatchImageSettlementService_UsesSubmittedPricingSnapshot(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_snapshot")
	job.SuccessCount = 2
	job.FailCount = 0
	job.ItemCount = 2
	job.PricingSnapshotVersion = 1
	job.BaseUnitPrice = 0.25
	job.GroupRateMultiplier = 1
	job.AccountRateMultiplier = 1
	job.BatchDiscountMultiplier = 1
	job.HoldMultiplier = 1.1
	job.BillableUnitPrice = 0.25
	job.HoldUnitPrice = 0.275
	holdAmount := 0.55
	job.HoldAmount = &holdAmount
	job.EstimatedCost = 0.5
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.50}}

	result, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.InDelta(t, 0.5, result.ActualCost, 1e-12)
	require.Len(t, billing.captures, 1)
	require.InDelta(t, 0.5, billing.captures[0].ActualAmount, 1e-12)
	require.InDelta(t, 0.55, billing.captures[0].HoldAmount, 1e-12)
}

func TestBatchImageSettlementService_BillingFailureLeavesSettlingAndRecordsError(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_billing_fail")
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{err: errors.New("temporary billing timeout with gs://hidden-output")}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
	require.Equal(t, BatchImageJobStatusSettling, repo.jobs[job.BatchID].Status)
	require.Equal(t, "SETTLEMENT_BILLING_FAILED", batchImageDerefString(repo.jobs[job.BatchID].LastErrorCode))
	require.Contains(t, batchImageDerefString(repo.jobs[job.BatchID].LastErrorMessage), "temporary billing timeout")
	require.NotNil(t, billing.captures[0])
}

func TestBatchImagePipelineProcessor_SettlesQueuedSettlingJob(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_pipeline")
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	settlement := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}
	processor := &BatchImagePipelineProcessor{
		ProviderProcessor: &BatchImageProviderProcessor{Repo: repo, ProviderRegistry: NewBatchImageProviderRegistry(&fakeProcessorProvider{}), AccountResolver: &fakeBatchImageAccountResolver{account: &Account{}}},
		SettlementService: settlement,
	}

	result, err := processor.Process(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.True(t, result.Terminal)
	require.Equal(t, BatchImageJobStatusCompleted, repo.jobs[job.BatchID].Status)
	require.Len(t, billing.captures, 1)
}

func TestBatchImagePipelineProcessor_RequeuesTransientSettlementFailure(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_pipeline_retry")
	repo.jobs[job.BatchID] = job
	settlement := &BatchImageSettlementService{Repo: repo, BillingRepo: &fakeBatchImageBillingRepo{err: errors.New("temporary")}, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}
	processor := &BatchImagePipelineProcessor{
		ProviderProcessor: &BatchImageProviderProcessor{Repo: repo, ProviderRegistry: NewBatchImageProviderRegistry(&fakeProcessorProvider{}), AccountResolver: &fakeBatchImageAccountResolver{account: &Account{}}},
		SettlementService: settlement,
	}

	result, err := processor.Process(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.False(t, result.Terminal)
	require.Equal(t, batchImageSettlementRetryDelay, result.RequeueAfter)
	require.Equal(t, BatchImageJobStatusSettling, repo.jobs[job.BatchID].Status)
}

func TestBatchImagePipelineProcessor_FailsAndReleasesAfterSettlementRetryLimit(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_pipeline_retry_exhausted")
	job.RetryCount = batchImageSettlementMaxRetries - 1
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{captureErr: errors.New("temporary billing timeout")}
	settlement := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}
	processor := &BatchImagePipelineProcessor{
		ProviderProcessor: &BatchImageProviderProcessor{Repo: repo, ProviderRegistry: NewBatchImageProviderRegistry(&fakeProcessorProvider{}), AccountResolver: &fakeBatchImageAccountResolver{account: &Account{}}},
		SettlementService: settlement,
	}

	result, err := processor.Process(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.True(t, result.Terminal)
	require.Equal(t, BatchImageJobStatusFailed, repo.jobs[job.BatchID].Status)
	require.Equal(t, "SETTLEMENT_BILLING_RETRY_EXHAUSTED", batchImageDerefString(repo.jobs[job.BatchID].LastErrorCode))
	require.Len(t, billing.captures, 1)
	require.Len(t, billing.releases, 1)
	require.Equal(t, BatchImageReleaseRequestID(job.BatchID), billing.releases[0].RequestID)
}

func TestBatchImageSettlementRetryExhaustedReleaseIsIdempotentAfterTransitionFailure(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_retry_exhausted_transition_fail")
	job.RetryCount = batchImageSettlementMaxRetries
	job.LastErrorCode = batchImageStringPtr("SETTLEMENT_BILLING_FAILED")
	repo.jobs[job.BatchID] = job
	repo.transitionErr = errors.New("temporary transition failure")
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorContains(t, err, "temporary transition failure")
	require.Equal(t, BatchImageJobStatusSettling, repo.jobs[job.BatchID].Status)
	require.Len(t, billing.releases, 1)
	require.Len(t, billing.seen, 1)

	repo.transitionErr = nil
	_, err = svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
	require.Equal(t, BatchImageJobStatusFailed, repo.jobs[job.BatchID].Status)
	require.Len(t, billing.releases, 2)
	require.Equal(t, billing.releases[0].RequestID, billing.releases[1].RequestID)
	require.Len(t, billing.seen, 1)
}

func TestBatchImageSettlementService_CostExceedsHoldExhaustsAndReleases(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_over_hold_exhausted")
	job.SuccessCount = 2
	job.FailCount = 0
	job.ItemCount = 2
	holdAmount := 0.5
	job.HoldAmount = &holdAmount
	job.EstimatedCost = holdAmount
	requestHash := "request-hash-over-hold"
	job.RequestHash = &requestHash
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.50}}

	// 前 N-1 次：记录失败并返回错误（等待 worker 重试）。
	for i := 0; i < batchImageSettlementMaxRetries-1; i++ {
		_, err := svc.Settle(context.Background(), job.BatchID)
		require.ErrorIs(t, err, ErrBatchImageSettlementCostExceedsHold)
		require.Equal(t, BatchImageJobStatusSettling, repo.jobs[job.BatchID].Status)
	}
	// 达到上限：必须走耗尽出口释放冻结并转 failed，而不是无限 requeue。
	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
	require.Equal(t, BatchImageJobStatusFailed, repo.jobs[job.BatchID].Status)
	require.Empty(t, billing.captures)
	require.Len(t, billing.releases, 1)
	require.Equal(t, BatchImageReleaseRequestID(job.BatchID), billing.releases[0].RequestID)
	// 释放指纹必须与 processor/Cancel/recovery 一致地使用 RequestHash，
	// 否则共享同一 request id 的后续释放会命中指纹冲突（毒消息）。
	require.Equal(t, requestHash, billing.releases[0].RequestPayloadHash)
}

func TestBatchImageSettlementService_InvalidCountsExhaustsAndReleases(t *testing.T) {
	repo := newFakeBatchImageRepository()
	job := testSettlingBatchImageJob("imgbatch_bad_counts_exhausted")
	job.SuccessCount = 2
	job.FailCount = 2
	job.ItemCount = 3
	requestHash := "request-hash-bad-counts"
	job.RequestHash = &requestHash
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}

	for i := 0; i < batchImageSettlementMaxRetries-1; i++ {
		_, err := svc.Settle(context.Background(), job.BatchID)
		require.ErrorIs(t, err, ErrBatchImageSettlementInvalidCounts)
	}
	_, err := svc.Settle(context.Background(), job.BatchID)
	require.ErrorIs(t, err, ErrBatchImageSettlementBillingFailed)
	require.Equal(t, BatchImageJobStatusFailed, repo.jobs[job.BatchID].Status)
	require.Empty(t, billing.captures)
	require.Len(t, billing.releases, 1)
	require.Equal(t, requestHash, billing.releases[0].RequestPayloadHash)
}

func TestReleaseBatchImageBalanceHold_TreatsFingerprintConflictAsReleased(t *testing.T) {
	job := testSettlingBatchImageJob("imgbatch_release_conflict")
	// 历史版本用 manifestHash 释放过一次：同一 request id 再以 RequestHash
	// 释放会命中指纹冲突。资金已归还，必须视为幂等成功而非毒消息。
	billing := &fakeBatchImageBillingRepo{releaseErr: ErrUsageBillingRequestConflict}
	err := releaseBatchImageBalanceHold(context.Background(), billing, job, "request-hash")
	require.NoError(t, err)
	require.Len(t, billing.releases, 1)
}

func TestBatchImageSettlementManifestHash(t *testing.T) {
	job := testSettlingBatchImageJob("imgbatch_hash")
	first := BuildBatchImageSettlementManifestHash(job)
	job.CreatedAt = job.CreatedAt.AddDate(0, 0, 1)
	job.UpdatedAt = job.UpdatedAt.AddDate(0, 0, 1)
	require.Equal(t, first, BuildBatchImageSettlementManifestHash(job))

	job.SuccessCount++
	require.NotEqual(t, first, BuildBatchImageSettlementManifestHash(job))

	job.SuccessCount--
	promptOrBase64 := first + " prompt " + batchImageTestData
	require.NotContains(t, BuildBatchImageSettlementManifestHash(job), promptOrBase64)
}

func TestBatchImageSettlementBillingRequestIDs(t *testing.T) {
	repo := newFakeBatchImageRepository()
	first := testSettlingBatchImageJob("imgbatch_unique_1")
	second := testSettlingBatchImageJob("imgbatch_unique_2")
	repo.jobs[first.BatchID] = first
	repo.jobs[second.BatchID] = second
	billing := &fakeBatchImageBillingRepo{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, UsageLogRepo: &batchSettlementUsageLogRepo{}, Pricing: &fakeBatchImagePricingResolver{unitPrice: 0.25}}

	_, err := svc.Settle(context.Background(), first.BatchID)
	require.NoError(t, err)
	_, err = svc.Settle(context.Background(), first.BatchID)
	require.NoError(t, err)
	_, err = svc.Settle(context.Background(), second.BatchID)
	require.NoError(t, err)

	require.Len(t, billing.captures, 2)
	require.Equal(t, BatchImageCaptureRequestID(first.BatchID), billing.captures[0].RequestID)
	require.Equal(t, BatchImageCaptureRequestID(second.BatchID), billing.captures[1].RequestID)
	require.NotEqual(t, billing.captures[0].RequestID, billing.captures[1].RequestID)
	require.Len(t, billing.seen, 2)
}

func testSettlingBatchImageJob(batchID string) *BatchImageJob {
	apiKeyID := int64(321)
	accountID := int64(654)
	providerJobName := "providers/job"
	outputRef := "files/output"
	holdAmount := 1.25
	holdID := BatchImageHoldRequestID(batchID)
	return &BatchImageJob{
		BatchID:           batchID,
		UserID:            123,
		APIKeyID:          &apiKeyID,
		AccountID:         &accountID,
		Provider:          BatchImageProviderGeminiAPI,
		Model:             "gemini-image",
		Status:            BatchImageJobStatusSettling,
		ProviderJobName:   &providerJobName,
		ProviderOutputRef: &outputRef,
		ItemCount:         3,
		SuccessCount:      2,
		FailCount:         1,
		EstimatedCost:     holdAmount,
		HoldAmount:        &holdAmount,
		HoldID:            &holdID,
	}
}

type fakeBatchImagePricingResolver struct {
	unitPrice     float64
	missingModels map[string]bool
	err           error
}

func (r *fakeBatchImagePricingResolver) BatchImageUnitPrice(_ context.Context, job *BatchImageJob) (float64, error) {
	if r.err != nil {
		return 0, r.err
	}
	if job != nil && r.missingModels[job.Model] {
		return 0, ErrBatchImageSettlementPricingMissing
	}
	return r.unitPrice, nil
}

type fakeBatchImageBillingRepo struct {
	commands       []*UsageBillingCommand
	reserves       []*BatchImageBalanceHoldCommand
	captures       []*BatchImageBalanceHoldCommand
	releases       []*BatchImageBalanceHoldCommand
	seen           map[string]struct{}
	alreadyApplied map[string]bool
	err            error
	reserveErr     error
	captureErr     error
	releaseErr     error
	captureResult  *BatchImageBalanceHoldResult
	captureResults map[string]float64
}

func (r *fakeBatchImageBillingRepo) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	if r.seen == nil {
		r.seen = make(map[string]struct{})
	}
	if r.err != nil {
		r.commands = append(r.commands, cmd)
		return nil, r.err
	}
	if cmd != nil {
		cmd.Normalize()
		if _, ok := r.seen[cmd.RequestID]; ok || r.alreadyApplied[cmd.RequestID] {
			r.commands = append(r.commands, cmd)
			return &UsageBillingApplyResult{Applied: false}, nil
		}
		r.seen[cmd.RequestID] = struct{}{}
	}
	r.commands = append(r.commands, cmd)
	return &UsageBillingApplyResult{Applied: true}, nil
}

func (r *fakeBatchImageBillingRepo) ReserveBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	if r.reserveErr != nil {
		r.reserves = append(r.reserves, cmd)
		return nil, r.reserveErr
	}
	return r.applyHold(cmd, &r.reserves)
}

func (r *fakeBatchImageBillingRepo) CaptureBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	if r.captureErr != nil {
		r.captures = append(r.captures, cmd)
		return nil, r.captureErr
	}
	result, err := r.applyHold(cmd, &r.captures)
	if err != nil || result == nil {
		return result, err
	}
	if result.Applied && r.captureResult != nil {
		copy := *r.captureResult
		copy.Applied = true
		if r.captureResults == nil {
			r.captureResults = make(map[string]float64)
		}
		if cmd != nil {
			r.captureResults[cmd.RequestID] = copy.ThresholdExemptCost
		}
		return &copy, nil
	}
	if !result.Applied && cmd != nil {
		result.ThresholdExemptCost = r.captureResults[cmd.RequestID]
	}
	return result, err
}

func (r *fakeBatchImageBillingRepo) ReleaseBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	if r.releaseErr != nil {
		r.releases = append(r.releases, cmd)
		return nil, r.releaseErr
	}
	return r.applyHold(cmd, &r.releases)
}

func (r *fakeBatchImageBillingRepo) applyHold(cmd *BatchImageBalanceHoldCommand, calls *[]*BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	if r.seen == nil {
		r.seen = make(map[string]struct{})
	}
	if r.err != nil {
		*calls = append(*calls, cmd)
		return nil, r.err
	}
	if cmd != nil {
		cmd.Normalize()
		if _, ok := r.seen[cmd.RequestID]; ok || r.alreadyApplied[cmd.RequestID] {
			*calls = append(*calls, cmd)
			return &BatchImageBalanceHoldResult{Applied: false}, nil
		}
		r.seen[cmd.RequestID] = struct{}{}
	}
	*calls = append(*calls, cmd)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}

var _ UsageBillingRepository = (*fakeBatchImageBillingRepo)(nil)
var _ BatchImagePricingResolver = (*fakeBatchImagePricingResolver)(nil)
var _ = strings.TrimSpace
