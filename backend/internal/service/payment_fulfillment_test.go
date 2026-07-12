//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type paymentFulfillmentTestProvider struct {
	key            string
	supportedTypes []payment.PaymentType
}

func (p paymentFulfillmentTestProvider) Name() string        { return p.key }
func (p paymentFulfillmentTestProvider) ProviderKey() string { return p.key }
func (p paymentFulfillmentTestProvider) SupportedTypes() []payment.PaymentType {
	return p.supportedTypes
}
func (p paymentFulfillmentTestProvider) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	panic("unexpected call")
}
func (p paymentFulfillmentTestProvider) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	panic("unexpected call")
}
func (p paymentFulfillmentTestProvider) VerifyNotification(ctx context.Context, rawBody string, headers map[string]string) (*payment.PaymentNotification, error) {
	panic("unexpected call")
}
func (p paymentFulfillmentTestProvider) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	panic("unexpected call")
}

type paymentFulfillmentAffiliateAccrueCall struct {
	inviterID     int64
	inviteeUserID int64
	amount        float64
	freezeHours   int
	sourceOrderID *int64
}

type paymentFulfillmentAffiliateRepoStub struct {
	inviteeSummary     *AffiliateSummary
	inviterSummary     *AffiliateSummary
	accrueCalls        []paymentFulfillmentAffiliateAccrueCall
	qualifiedCount     int
	reconcileErr       error
	reconcileCalls     int
	lockBusy           bool
	reconcileInviteeFn func(context.Context, int64, float64) (*AffiliateQualification, error)
	reconcileRequired  bool
	generation         int64
	markReconcileErr   error
	markReconcileCalls int
	dirtyEvents        []AffiliateQualificationDirtyEvent
	auditClient        *dbent.Client
}

func (r *paymentFulfillmentAffiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	switch {
	case r.inviteeSummary != nil && r.inviteeSummary.UserID == userID:
		cp := *r.inviteeSummary
		return &cp, nil
	case r.inviterSummary != nil && r.inviterSummary.UserID == userID:
		cp := *r.inviterSummary
		return &cp, nil
	default:
		return &AffiliateSummary{UserID: userID, AffCode: "AFFTEST", CreatedAt: time.Now().Add(-time.Hour)}, nil
	}
}

func (r *paymentFulfillmentAffiliateRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	panic("unexpected GetAffiliateByCode call")
}

func (r *paymentFulfillmentAffiliateRepoStub) BindInviter(context.Context, int64, int64) (bool, error) {
	panic("unexpected BindInviter call")
}

func (r *paymentFulfillmentAffiliateRepoStub) AccrueQuota(_ context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, sourceOrderID *int64) (bool, error) {
	var sourceCopy *int64
	if sourceOrderID != nil {
		v := *sourceOrderID
		sourceCopy = &v
	}
	r.accrueCalls = append(r.accrueCalls, paymentFulfillmentAffiliateAccrueCall{
		inviterID:     inviterID,
		inviteeUserID: inviteeUserID,
		amount:        amount,
		freezeHours:   freezeHours,
		sourceOrderID: sourceCopy,
	})
	return true, nil
}

func (r *paymentFulfillmentAffiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return 0, nil
}

func (r *paymentFulfillmentAffiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	panic("unexpected ThawFrozenQuota call")
}

func (r *paymentFulfillmentAffiliateRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	panic("unexpected TransferQuotaToBalance call")
}

func (r *paymentFulfillmentAffiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	panic("unexpected ListInvitees call")
}

func (r *paymentFulfillmentAffiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	panic("unexpected UpdateUserAffCode call")
}

func (r *paymentFulfillmentAffiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	panic("unexpected ResetUserAffCode call")
}

func (r *paymentFulfillmentAffiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	panic("unexpected SetUserRebateRate call")
}

func (r *paymentFulfillmentAffiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	panic("unexpected BatchSetUserRebateRate call")
}

func (r *paymentFulfillmentAffiliateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	panic("unexpected ListUsersWithCustomSettings call")
}

func (r *paymentFulfillmentAffiliateRepoStub) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	panic("unexpected ListAffiliateInviteRecords call")
}

func (r *paymentFulfillmentAffiliateRepoStub) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	panic("unexpected ListAffiliateRebateRecords call")
}

func (r *paymentFulfillmentAffiliateRepoStub) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	panic("unexpected ListAffiliateTransferRecords call")
}

func (r *paymentFulfillmentAffiliateRepoStub) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	panic("unexpected GetAffiliateUserOverview call")
}

func (r *paymentFulfillmentAffiliateRepoStub) ReconcileInviteeQualification(ctx context.Context, userID int64, threshold float64) (*AffiliateQualification, error) {
	r.reconcileCalls++
	if r.reconcileInviteeFn != nil {
		return r.reconcileInviteeFn(ctx, userID, threshold)
	}
	if r.reconcileErr != nil {
		return nil, r.reconcileErr
	}
	return &AffiliateQualification{}, nil
}

func (r *paymentFulfillmentAffiliateRepoStub) CountQualifiedInvitees(context.Context, int64, float64) (int, error) {
	return r.qualifiedCount, nil
}

func (r *paymentFulfillmentAffiliateRepoStub) ReconcileAllAffiliateQualifications(context.Context, float64, int) error {
	r.reconcileCalls++
	return r.reconcileErr
}

func (r *paymentFulfillmentAffiliateRepoStub) TryWithAffiliateQualificationReconcileLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	if r.lockBusy {
		return false, nil
	}
	return true, fn(ctx)
}

func (r *paymentFulfillmentAffiliateRepoStub) MarkReconcileRequired(context.Context) (AffiliateReconcileToken, error) {
	r.markReconcileCalls++
	if r.markReconcileErr != nil {
		return AffiliateReconcileToken{}, r.markReconcileErr
	}
	wasPending := r.reconcileRequired
	r.generation++
	if r.generation <= 0 {
		r.generation = 1
	}
	r.reconcileRequired = true
	return AffiliateReconcileToken{Generation: r.generation, WasPendingBefore: wasPending}, nil
}

func (r *paymentFulfillmentAffiliateRepoStub) ReadReconcilePendingSnapshot(context.Context) (AffiliateReconcilePendingSnapshot, error) {
	return AffiliateReconcilePendingSnapshot{Required: r.reconcileRequired, Generation: r.generation}, nil
}

func (r *paymentFulfillmentAffiliateRepoStub) ClearReconcileRequiredIfGeneration(_ context.Context, expected int64) (bool, error) {
	if !r.reconcileRequired || r.generation != expected {
		return false, nil
	}
	r.reconcileRequired = false
	return true, nil
}

func (r *paymentFulfillmentAffiliateRepoStub) ListAffiliateQualificationDirtyEvents(ctx context.Context, _ int) ([]AffiliateQualificationDirtyEvent, error) {
	if r.auditClient != nil {
		logs, err := r.auditClient.PaymentAuditLog.Query().Where(paymentauditlog.ActionEQ(AffiliateQualificationDirtyAuditAction)).All(ctx)
		if err != nil {
			return nil, err
		}
		events := make([]AffiliateQualificationDirtyEvent, 0, len(logs))
		for _, logEntry := range logs {
			event := AffiliateQualificationDirtyEvent{OrderID: logEntry.OrderID, Detail: logEntry.Detail}
			if err := json.Unmarshal([]byte(logEntry.Detail), &event); err != nil {
				return nil, err
			}
			events = append(events, event)
		}
		return events, nil
	}
	return append([]AffiliateQualificationDirtyEvent(nil), r.dirtyEvents...), nil
}

func (r *paymentFulfillmentAffiliateRepoStub) DeleteAffiliateQualificationDirtyEvent(ctx context.Context, event AffiliateQualificationDirtyEvent) (bool, error) {
	if r.auditClient != nil {
		deleted, err := r.auditClient.PaymentAuditLog.Delete().Where(
			paymentauditlog.OrderIDEQ(event.OrderID),
			paymentauditlog.ActionEQ(AffiliateQualificationDirtyAuditAction),
			paymentauditlog.DetailEQ(event.Detail),
		).Exec(ctx)
		return deleted == 1, err
	}
	for i, pending := range r.dirtyEvents {
		if pending.OrderID == event.OrderID && pending.Detail == event.Detail {
			r.dirtyEvents = append(r.dirtyEvents[:i], r.dirtyEvents[i+1:]...)
			return true, nil
		}
	}
	return true, nil
}

func (r *paymentFulfillmentAffiliateRepoStub) MarkAffiliateQualificationDirtyEventFailed(context.Context, AffiliateQualificationDirtyEvent, error) error {
	return nil
}

type paymentFulfillmentSettingRepoStub struct {
	values         map[string]string
	setErr         error
	getMultipleErr error
}

func (s *paymentFulfillmentSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *paymentFulfillmentSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if s.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *paymentFulfillmentSettingRepoStub) Set(_ context.Context, key, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *paymentFulfillmentSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if s.getMultipleErr != nil {
		return nil, s.getMultipleErr
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = s.values[key]
	}
	return out, nil
}

func (s *paymentFulfillmentSettingRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	for key, value := range values {
		s.values[key] = value
	}
	return nil
}

func (s *paymentFulfillmentSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *paymentFulfillmentSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func ensurePaymentAuditOrderActionUniqueIndex(t *testing.T, ctx context.Context, client *dbent.Client) {
	t.Helper()
	_, err := client.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_audit_logs_order_action_uniq ON payment_audit_logs(order_id, action)")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// resolveRedeemAction — pure idempotency decision logic
// ---------------------------------------------------------------------------

func TestResolveRedeemAction_CodeNotFound(t *testing.T) {
	t.Parallel()
	action := resolveRedeemAction(nil, nil)
	assert.Equal(t, redeemActionCreate, action, "nil code with nil error should create")
}

func TestResolveRedeemAction_LookupError(t *testing.T) {
	t.Parallel()
	action := resolveRedeemAction(nil, errors.New("db connection lost"))
	assert.Equal(t, redeemActionCreate, action, "lookup error should fall back to create")
}

func TestResolveRedeemAction_LookupErrorWithNonNilCode(t *testing.T) {
	t.Parallel()
	// Edge case: both code and error are non-nil (shouldn't happen in practice,
	// but the function should still treat error as authoritative)
	code := &RedeemCode{Status: StatusUnused}
	action := resolveRedeemAction(code, errors.New("partial error"))
	assert.Equal(t, redeemActionCreate, action, "non-nil error should always result in create regardless of code")
}

func TestResolveRedeemAction_CodeExistsAndUsed(t *testing.T) {
	t.Parallel()
	code := &RedeemCode{
		Code:   "test-code-123",
		Status: StatusUsed,
		Type:   RedeemTypeBalance,
		Value:  10.0,
	}
	action := resolveRedeemAction(code, nil)
	assert.Equal(t, redeemActionSkipCompleted, action, "used code should skip to completed")
}

func TestResolveRedeemAction_CodeExistsAndUnused(t *testing.T) {
	t.Parallel()
	code := &RedeemCode{
		Code:   "test-code-456",
		Status: StatusUnused,
		Type:   RedeemTypeBalance,
		Value:  25.0,
	}
	action := resolveRedeemAction(code, nil)
	assert.Equal(t, redeemActionRedeem, action, "unused code should skip creation and proceed to redeem")
}

func TestResolveRedeemAction_CodeExistsWithExpiredStatus(t *testing.T) {
	t.Parallel()
	// A code with a non-standard status (neither "unused" nor "used")
	// should NOT be treated as used, so it falls through to redeemActionRedeem.
	code := &RedeemCode{
		Code:   "expired-code",
		Status: StatusExpired,
	}
	action := resolveRedeemAction(code, nil)
	assert.Equal(t, redeemActionRedeem, action, "expired-status code is not IsUsed(), should redeem")
}

// ---------------------------------------------------------------------------
// Table-driven comprehensive test
// ---------------------------------------------------------------------------

func TestResolveRedeemAction_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     *RedeemCode
		err      error
		expected redeemAction
	}{
		{
			name:     "nil code, nil error — first run",
			code:     nil,
			err:      nil,
			expected: redeemActionCreate,
		},
		{
			name:     "nil code, lookup error — treat as not found",
			code:     nil,
			err:      ErrRedeemCodeNotFound,
			expected: redeemActionCreate,
		},
		{
			name:     "nil code, generic DB error — treat as not found",
			code:     nil,
			err:      errors.New("connection refused"),
			expected: redeemActionCreate,
		},
		{
			name:     "code exists, used — previous run completed redeem",
			code:     &RedeemCode{Status: StatusUsed},
			err:      nil,
			expected: redeemActionSkipCompleted,
		},
		{
			name:     "code exists, unused — previous run created code but crashed before redeem",
			code:     &RedeemCode{Status: StatusUnused},
			err:      nil,
			expected: redeemActionRedeem,
		},
		{
			name:     "code exists but error also set — error takes precedence",
			code:     &RedeemCode{Status: StatusUsed},
			err:      errors.New("unexpected"),
			expected: redeemActionCreate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveRedeemAction(tt.code, tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// ---------------------------------------------------------------------------
// redeemAction enum value sanity
// ---------------------------------------------------------------------------

func TestRedeemAction_DistinctValues(t *testing.T) {
	t.Parallel()
	// Ensure the three actions have distinct values (iota correctness)
	assert.NotEqual(t, redeemActionCreate, redeemActionRedeem)
	assert.NotEqual(t, redeemActionCreate, redeemActionSkipCompleted)
	assert.NotEqual(t, redeemActionRedeem, redeemActionSkipCompleted)
}

// ---------------------------------------------------------------------------
// RedeemCode.IsUsed / CanUse interaction with resolveRedeemAction
// ---------------------------------------------------------------------------

func TestResolveRedeemAction_IsUsedCanUseConsistency(t *testing.T) {
	t.Parallel()

	usedCode := &RedeemCode{Status: StatusUsed}
	unusedCode := &RedeemCode{Status: StatusUnused}

	// Verify our decision function is consistent with the domain model methods
	assert.True(t, usedCode.IsUsed())
	assert.False(t, usedCode.CanUse())
	assert.Equal(t, redeemActionSkipCompleted, resolveRedeemAction(usedCode, nil))

	assert.False(t, unusedCode.IsUsed())
	assert.True(t, unusedCode.CanUse())
	assert.Equal(t, redeemActionRedeem, resolveRedeemAction(unusedCode, nil))
}

func TestExpectedNotificationProviderKeyPrefersOrderInstanceProvider(t *testing.T) {
	t.Parallel()

	registry := payment.NewRegistry()
	registry.Register(paymentFulfillmentTestProvider{
		key:            payment.TypeAlipay,
		supportedTypes: []payment.PaymentType{payment.TypeAlipay},
	})

	assert.Equal(t,
		payment.TypeEasyPay,
		expectedNotificationProviderKey(registry, payment.TypeAlipay, "", payment.TypeEasyPay),
	)
}

func TestExpectedNotificationProviderKeyUsesRegistryMappingForLegacyOrders(t *testing.T) {
	t.Parallel()

	registry := payment.NewRegistry()
	registry.Register(paymentFulfillmentTestProvider{
		key:            payment.TypeEasyPay,
		supportedTypes: []payment.PaymentType{payment.TypeAlipay},
	})

	assert.Equal(t,
		payment.TypeEasyPay,
		expectedNotificationProviderKey(registry, payment.TypeAlipay, "", ""),
	)
}

func TestExpectedNotificationProviderKeyFallsBackToPaymentType(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		payment.TypeWxpay,
		expectedNotificationProviderKey(nil, payment.TypeWxpay, "", ""),
	)
}

func TestExpectedNotificationProviderKeyPrefersOrderSnapshotProviderKey(t *testing.T) {
	t.Parallel()

	registry := payment.NewRegistry()
	registry.Register(paymentFulfillmentTestProvider{
		key:            payment.TypeAlipay,
		supportedTypes: []payment.PaymentType{payment.TypeAlipay},
	})

	assert.Equal(t,
		payment.TypeEasyPay,
		expectedNotificationProviderKey(registry, payment.TypeAlipay, payment.TypeEasyPay, ""),
	)
}

func TestExpectedNotificationProviderKeyForOrderUsesSnapshotProviderKey(t *testing.T) {
	t.Parallel()

	registry := payment.NewRegistry()
	registry.Register(paymentFulfillmentTestProvider{
		key:            payment.TypeAlipay,
		supportedTypes: []payment.PaymentType{payment.TypeAlipay},
	})

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeAlipay,
		ProviderSnapshot: map[string]any{
			"schema_version": 1,
			"provider_key":   payment.TypeEasyPay,
		},
	}

	assert.Equal(t,
		payment.TypeEasyPay,
		expectedNotificationProviderKeyForOrder(registry, order, ""),
	)
}

func TestValidateProviderNotificationMetadataRejectsWxpaySnapshotMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeWxpay,
		ProviderSnapshot: map[string]any{
			"schema_version":  1,
			"merchant_app_id": "wx-app-expected",
			"merchant_id":     "mch-expected",
			"currency":        "CNY",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeWxpay, map[string]string{
		"appid":       "wx-app-other",
		"mchid":       "mch-expected",
		"currency":    "CNY",
		"trade_state": "SUCCESS",
	})
	assert.ErrorContains(t, err, "wxpay appid mismatch")
}

func TestValidateProviderNotificationMetadataAllowsLegacyOrdersWithoutSnapshotFields(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeWxpay,
		ProviderSnapshot: map[string]any{
			"schema_version":       1,
			"provider_instance_id": "9",
			"provider_key":         payment.TypeWxpay,
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeWxpay, map[string]string{
		"appid":       "wx-app-runtime",
		"mchid":       "mch-runtime",
		"currency":    "CNY",
		"trade_state": "SUCCESS",
	})
	assert.NoError(t, err)
}

func TestParseLegacyPaymentOrderID(t *testing.T) {
	t.Parallel()

	oid, ok := parseLegacyPaymentOrderID("sub2_42", &dbent.NotFoundError{})
	assert.True(t, ok)
	assert.EqualValues(t, 42, oid)

	_, ok = parseLegacyPaymentOrderID("42", &dbent.NotFoundError{})
	assert.False(t, ok)

	_, ok = parseLegacyPaymentOrderID("sub2_42", errors.New("db down"))
	assert.False(t, ok)
}

func TestIsValidProviderAmount(t *testing.T) {
	t.Parallel()

	assert.True(t, isValidProviderAmount(0.01))
	assert.False(t, isValidProviderAmount(0))
	assert.False(t, isValidProviderAmount(-1))
	assert.False(t, isValidProviderAmount(math.NaN()))
	assert.False(t, isValidProviderAmount(math.Inf(1)))
}

func TestValidateProviderNotificationMetadataRejectsAlipaySnapshotMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeAlipay,
		ProviderSnapshot: map[string]any{
			"schema_version":  2,
			"merchant_app_id": "alipay-app-expected",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeAlipay, map[string]string{
		"app_id": "alipay-app-other",
	})
	assert.ErrorContains(t, err, "alipay app_id mismatch")
}

func TestValidateProviderNotificationMetadataRejectsEasyPaySnapshotMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeAlipay,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"merchant_id":    "pid-expected",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeEasyPay, map[string]string{
		"pid": "pid-other",
	})
	assert.ErrorContains(t, err, "easypay pid mismatch")
}

func TestValidateProviderNotificationMetadataRejectsAirwallexSnapshotMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeAirwallex,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"merchant_id":    "acct_expected",
			"currency":       "CNY",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeAirwallex, map[string]string{
		"account_id": "acct_other",
		"currency":   "CNY",
		"status":     "SUCCEEDED",
	})
	assert.ErrorContains(t, err, "airwallex account_id mismatch")

	err = validateProviderNotificationMetadata(order, payment.TypeAirwallex, map[string]string{
		"account_id": "acct_expected",
		"currency":   "USD",
		"status":     "SUCCEEDED",
	})
	assert.ErrorContains(t, err, "airwallex currency mismatch")
}

func TestValidateProviderNotificationMetadataRejectsStripeCurrencyMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeStripe,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"currency":       "HKD",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeStripe, map[string]string{
		"currency": "USD",
	})
	assert.ErrorContains(t, err, "stripe currency mismatch")
}

func TestPaymentAmountToleranceForThreeDecimalCurrency(t *testing.T) {
	t.Parallel()

	assert.Equal(t, amountToleranceCNY, paymentAmountToleranceForCurrency("CNY"))
	assert.Equal(t, amountToleranceCNY, paymentAmountToleranceForCurrency("JPY"))
	assert.InDelta(t, 0.0005, paymentAmountToleranceForCurrency("KWD"), 1e-12)
}

func TestRetryFulfillmentRejectsFreshRechargingLease(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusRecharging, time.Now())

	svc := &PaymentService{entClient: client}
	err := svc.RetryFulfillment(ctx, order.ID)
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))

	reloaded, getErr := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, getErr)
	require.Equal(t, OrderStatusRecharging, reloaded.Status)
}

func TestAlreadyProcessedRecoversStaleRechargingLease(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	order := createPaymentFulfillmentSubscriptionOrder(
		t,
		ctx,
		client,
		OrderStatusRecharging,
		time.Now().Add(-paymentFulfillmentLeaseDuration-time.Minute),
	)
	_, err := client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("SUBSCRIPTION_ASSIGNED").
		SetDetail(`{"groupID":7,"validityDays":30}`).
		SetOperator("system").
		Save(ctx)
	require.NoError(t, err)

	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       groupRepo,
		subscriptionSvc: NewSubscriptionService(groupRepo, userSubRepoNoop{}, nil, nil, nil),
	}

	require.NoError(t, svc.alreadyProcessed(ctx, order))
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
}

func TestFulfillmentLeaseVersionRejectsStaleWorker(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	staleAt := time.Now().Add(-paymentFulfillmentLeaseDuration - time.Minute)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusRecharging, staleAt)
	svc := &PaymentService{entClient: client}

	firstLease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
	require.NoError(t, err)
	require.NotNil(t, firstLease)

	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetUpdatedAt(staleAt).Save(ctx)
	require.NoError(t, err)
	time.Sleep(time.Millisecond)
	staleOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	secondLease, err := svc.acquirePaymentFulfillmentLease(ctx, staleOrder)
	require.NoError(t, err)
	require.NotNil(t, secondLease)
	require.False(t, firstLease.version.Equal(secondLease.version))

	err = svc.markCompleted(ctx, order, firstLease, "SUBSCRIPTION_SUCCESS")
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))
	svc.markFailed(ctx, order.ID, firstLease, errors.New("stale worker failure"))

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRecharging, reloaded.Status)
	require.NoError(t, svc.markCompleted(ctx, order, secondLease, "SUBSCRIPTION_SUCCESS"))
}

func TestExecuteBalanceFulfillmentRecoversAfterRedeemWithoutCreditingAgain(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	staleAt := time.Now().Add(-paymentFulfillmentLeaseDuration - time.Minute)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusRecharging, staleAt)
	order, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetOrderType(payment.OrderTypeBalance).
		ClearPlanID().
		ClearSubscriptionGroupID().
		ClearSubscriptionDays().
		SetUpdatedAt(staleAt).
		Save(ctx)
	require.NoError(t, err)

	redeemRepo := &redeemCodeRepoStub{codesByCode: map[string]*RedeemCode{
		order.RechargeCode: {
			ID:     101,
			Code:   order.RechargeCode,
			Type:   RedeemTypeBalance,
			Value:  order.Amount,
			Status: StatusUsed,
		},
	}}
	svc := &PaymentService{
		entClient:     client,
		redeemService: &RedeemService{redeemRepo: redeemRepo},
	}

	require.NoError(t, svc.ExecuteBalanceFulfillment(ctx, order.ID))
	require.Empty(t, redeemRepo.useCalls, "an already-used order code must not be redeemed again")
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
}

func TestExecuteSubscriptionFulfillmentRecoversCommittedAssignmentWithoutExtendingAgain(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	staleAt := time.Now().Add(-paymentFulfillmentLeaseDuration - time.Minute)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusRecharging, staleAt)

	expiresAt := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        99,
		UserID:    order.UserID,
		GroupID:   *order.SubscriptionGroupID,
		StartsAt:  time.Now().Add(-time.Hour),
		ExpiresAt: expiresAt,
		Status:    SubscriptionStatusActive,
		Notes:     "manual note\n" + paymentSubscriptionOrderNote(order.ID) + "\nretained note",
	})
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       groupRepo,
		subscriptionSvc: NewSubscriptionService(groupRepo, subRepo, nil, nil, nil),
	}

	require.NoError(t, svc.ExecuteSubscriptionFulfillment(ctx, order.ID))
	assertPaymentSubscriptionExpiry(t, subRepo, order, expiresAt)

	assignmentAuditCount, err := client.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
			paymentauditlog.ActionEQ("SUBSCRIPTION_ASSIGNED"),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, assignmentAuditCount)

	// Simulate another stale recovery attempt after completion. The durable audit
	// must make replay a no-op for the subscription entitlement.
	_, err = client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusRecharging).
		SetUpdatedAt(staleAt).
		ClearCompletedAt().
		Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.ExecuteSubscriptionFulfillment(ctx, order.ID))
	assertPaymentSubscriptionExpiry(t, subRepo, order, expiresAt)

	assignmentAuditCount, err = client.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
			paymentauditlog.ActionEQ("SUBSCRIPTION_ASSIGNED"),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, assignmentAuditCount)
}

func TestHasPaymentSubscriptionOrderNoteRequiresIndependentExactLine(t *testing.T) {
	t.Parallel()
	require.True(t, hasPaymentSubscriptionOrderNote("before\r\npayment order 42\r\nafter", "payment order 42"))
	require.False(t, hasPaymentSubscriptionOrderNote("payment order 420", "payment order 42"))
	require.False(t, hasPaymentSubscriptionOrderNote("prefix payment order 42 suffix", "payment order 42"))
}

func createPaymentFulfillmentSubscriptionOrder(
	t *testing.T,
	ctx context.Context,
	client *dbent.Client,
	status string,
	updatedAt time.Time,
) *dbent.PaymentOrder {
	t.Helper()
	user, err := client.User.Create().
		SetEmail("fulfillment-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com").
		SetPasswordHash("hash").
		SetUsername("payment-fulfillment-user").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(80).
		SetPayAmount(80).
		SetFeeRate(0).
		SetRechargeCode("PAY-SUB-" + strconv.FormatInt(time.Now().UnixNano(), 10)).
		SetOutTradeNo("sub2_fulfillment_" + strconv.FormatInt(time.Now().UnixNano(), 10)).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-fulfillment").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(100).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(status).
		SetPaidAt(time.Now().Add(-time.Hour)).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetUpdatedAt(updatedAt).
		Save(ctx)
	require.NoError(t, err)
	return order
}

func assertPaymentSubscriptionExpiry(t *testing.T, repo *subscriptionUserSubRepoStub, order *dbent.PaymentOrder, expected time.Time) {
	t.Helper()
	sub, err := repo.GetByUserIDAndGroupID(context.Background(), order.UserID, *order.SubscriptionGroupID)
	require.NoError(t, err)
	require.True(t, sub.ExpiresAt.Equal(expected), "subscription expiry changed from %s to %s", expected, sub.ExpiresAt)
}

func TestApplyAffiliateRebateFallsBackWhenPendingReconcileFails(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	user, err := client.User.Create().SetEmail("affiliate-fallback@example.com").SetPasswordHash("hash").SetUsername("affiliate-fallback").Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(100).SetPayAmount(100).SetFeeRate(0).
		SetRechargeCode("PAY-AFFILIATE-FALLBACK").SetOutTradeNo("sub2_affiliate_fallback").
		SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("trade-affiliate-fallback").SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRecharging).SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").SetSrcHost("api.example.com").Save(ctx)
	require.NoError(t, err)

	inviterID := int64(9010)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		inviteeSummary:    &AffiliateSummary{UserID: user.ID, InviterID: &inviterID, CreatedAt: time.Now().Add(-time.Hour)},
		inviterSummary:    &AffiliateSummary{UserID: inviterID},
		reconcileErr:      errors.New("pending qualification reconcile failed"),
		reconcileRequired: true,
		generation:        1,
	}
	settings := &paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:               "true",
		SettingKeyAffiliateRebateRate:            "8",
		SettingKeyAffiliateTierReconcileRequired: "true",
		SettingKeyAffiliateQualificationAmount:   "50",
		SettingKeyAffiliateBronzeInvitees:        "3",
		SettingKeyAffiliateBronzeRate:            "10",
		SettingKeyAffiliateSilverInvitees:        "10",
		SettingKeyAffiliateSilverRate:            "12",
		SettingKeyAffiliateGoldInvitees:          "30",
		SettingKeyAffiliateGoldRate:              "15",
	}}
	svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(affiliateRepo, NewSettingService(settings, nil), nil, nil)}

	err = svc.applyAffiliateRebateForOrder(ctx, order)

	require.NoError(t, err)
	require.Len(t, affiliateRepo.accrueCalls, 1)
	require.Equal(t, 8.0, affiliateRepo.accrueCalls[0].amount)
	require.True(t, affiliateRepo.reconcileRequired)
	require.Equal(t, int64(2), affiliateRepo.generation)
	fallbacks, err := client.PaymentAuditLog.Query().Where(
		paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
		paymentauditlog.ActionEQ("AFFILIATE_TIER_FALLBACK"),
	).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, fallbacks)
}

func TestApplyAffiliateRebateFallsBackWhenQualificationReconcileLockIsBusy(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	user, err := client.User.Create().SetEmail("affiliate-lock-busy@example.com").SetPasswordHash("hash").SetUsername("affiliate-lock-busy").Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(100).SetPayAmount(100).SetFeeRate(0).SetRechargeCode("PAY-AFFILIATE-LOCK-BUSY").SetOutTradeNo("sub2_affiliate_lock_busy").
		SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("trade-affiliate-lock-busy").SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRecharging).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("api.example.com").Save(ctx)
	require.NoError(t, err)

	inviterID := int64(9011)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		qualifiedCount:    30,
		lockBusy:          true,
		reconcileRequired: true,
		generation:        1,
		inviteeSummary:    &AffiliateSummary{UserID: user.ID, InviterID: &inviterID, CreatedAt: time.Now().Add(-time.Hour)},
		inviterSummary:    &AffiliateSummary{UserID: inviterID},
	}
	settings := &paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:               "true",
		SettingKeyAffiliateRebateRate:            "8",
		SettingKeyAffiliateTierReconcileRequired: "true",
		SettingKeyAffiliateQualificationAmount:   "50",
		SettingKeyAffiliateBronzeInvitees:        "3",
		SettingKeyAffiliateBronzeRate:            "10",
		SettingKeyAffiliateSilverInvitees:        "10",
		SettingKeyAffiliateSilverRate:            "12",
		SettingKeyAffiliateGoldInvitees:          "30",
		SettingKeyAffiliateGoldRate:              "15",
	}}
	svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(affiliateRepo, NewSettingService(settings, nil), nil, nil)}

	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.Len(t, affiliateRepo.accrueCalls, 1)
	require.Equal(t, 8.0, affiliateRepo.accrueCalls[0].amount)
	require.True(t, affiliateRepo.reconcileRequired)
	require.Equal(t, int64(1), affiliateRepo.generation)
}

func TestApplyAffiliateRebateStrictFailureAuditsMarkerWriteFailure(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	user, err := client.User.Create().SetEmail("affiliate-strict-failure@example.com").SetPasswordHash("hash").SetUsername("affiliate-strict-failure").Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(100).SetPayAmount(100).SetFeeRate(0).SetRechargeCode("PAY-AFFILIATE-STRICT-FAILURE").SetOutTradeNo("sub2_affiliate_strict_failure").
		SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("trade-affiliate-strict-failure").SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRecharging).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("api.example.com").Save(ctx)
	require.NoError(t, err)
	inviterID := int64(9012)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		qualifiedCount:   30,
		inviteeSummary:   &AffiliateSummary{UserID: user.ID, InviterID: &inviterID, CreatedAt: time.Now().Add(-time.Hour)},
		inviterSummary:   &AffiliateSummary{UserID: inviterID},
		markReconcileErr: errors.New("generation bump failed"),
	}
	settings := &paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:             "true",
		SettingKeyAffiliateRebateRate:          "8",
		SettingKeyAffiliateQualificationAmount: "invalid",
	}}
	svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(affiliateRepo, NewSettingService(settings, nil), nil, nil)}

	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.Len(t, affiliateRepo.accrueCalls, 1)
	require.Equal(t, 8.0, affiliateRepo.accrueCalls[0].amount)
	fallback, err := client.PaymentAuditLog.Query().Where(
		paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
		paymentauditlog.ActionEQ("AFFILIATE_TIER_FALLBACK"),
	).Only(ctx)
	require.NoError(t, err)
	require.Contains(t, fallback.Detail, "bump affiliate qualification reconcile generation")
}

func TestPaymentTierThresholdOrderUsesOldRateAndNextOrderUsesNewRate(t *testing.T) {
	tests := []struct {
		name    string
		before  int
		after   int
		oldRate float64
		newRate float64
	}{
		{name: "bronze", before: 2, after: 3, oldRate: 8, newRate: 10},
		{name: "silver", before: 9, after: 10, oldRate: 10, newRate: 12},
		{name: "gold", before: 29, after: 30, oldRate: 12, newRate: 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
			user, err := client.User.Create().SetEmail("tier-" + tt.name + "@example.com").SetPasswordHash("hash").SetUsername("tier-" + tt.name).Save(ctx)
			require.NoError(t, err)
			inviterID := int64(9100 + tt.after)
			repo := &paymentFulfillmentAffiliateRepoStub{
				qualifiedCount: tt.before,
				inviteeSummary: &AffiliateSummary{UserID: user.ID, InviterID: &inviterID, CreatedAt: time.Now().Add(-time.Hour)},
				inviterSummary: &AffiliateSummary{UserID: inviterID},
			}
			var firstOrderID int64
			repo.reconcileInviteeFn = func(ctx context.Context, userID int64, threshold float64) (*AffiliateQualification, error) {
				if firstOrderID > 0 {
					first, getErr := client.PaymentOrder.Get(ctx, firstOrderID)
					require.NoError(t, getErr)
					if first.Status == OrderStatusCompleted {
						repo.qualifiedCount = tt.after
					}
				}
				return &AffiliateQualification{InviteeUserID: userID, QualifyingPaymentAmount: threshold}, nil
			}
			settings := &paymentFulfillmentSettingRepoStub{values: map[string]string{
				SettingKeyAffiliateEnabled:             "true",
				SettingKeyAffiliateRebateRate:          "8",
				SettingKeyAffiliateQualificationAmount: "50",
				SettingKeyAffiliateBronzeInvitees:      "3",
				SettingKeyAffiliateBronzeRate:          "10",
				SettingKeyAffiliateSilverInvitees:      "10",
				SettingKeyAffiliateSilverRate:          "12",
				SettingKeyAffiliateGoldInvitees:        "30",
				SettingKeyAffiliateGoldRate:            "15",
			}}
			svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)}
			createOrder := func(suffix string) *dbent.PaymentOrder {
				order, createErr := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
					SetAmount(100).SetPayAmount(100).SetFeeRate(0).SetRechargeCode("TIER-" + tt.name + "-" + suffix).SetOutTradeNo("sub2_tier_" + tt.name + "_" + suffix).
					SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("trade-tier-" + tt.name + "-" + suffix).SetOrderType(payment.OrderTypeBalance).
					SetStatus(OrderStatusPaid).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("api.example.com").Save(ctx)
				require.NoError(t, createErr)
				return order
			}

			first := createOrder("first")
			firstOrderID = first.ID
			firstLease, err := svc.acquirePaymentFulfillmentLease(ctx, first)
			require.NoError(t, err)
			require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, first))
			require.Equal(t, tt.oldRate, repo.accrueCalls[0].amount)
			require.NoError(t, svc.markCompleted(ctx, first, firstLease, "TIER_FIRST_COMPLETED"))
			require.Equal(t, tt.after, repo.qualifiedCount)

			second := createOrder("second")
			_, err = svc.acquirePaymentFulfillmentLease(ctx, second)
			require.NoError(t, err)
			require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, second))
			require.Len(t, repo.accrueCalls, 2)
			require.Equal(t, tt.newRate, repo.accrueCalls[1].amount)
		})
	}
}

func TestMarkCompletedKeepsBothPaymentTypesCompletedWhenAffiliateRefreshFails(t *testing.T) {
	for _, orderType := range []string{payment.OrderTypeBalance, payment.OrderTypeSubscription} {
		t.Run(orderType, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
			order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
			if orderType == payment.OrderTypeBalance {
				var err error
				order, err = client.PaymentOrder.UpdateOneID(order.ID).SetOrderType(payment.OrderTypeBalance).ClearPlanID().ClearSubscriptionGroupID().ClearSubscriptionDays().Save(ctx)
				require.NoError(t, err)
			}
			var err error
			order, err = client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			settings := &paymentFulfillmentSettingRepoStub{values: map[string]string{
				SettingKeyAffiliateQualificationAmount: "50",
				SettingKeyAffiliateRebateRate:          "8",
				SettingKeyAffiliateBronzeInvitees:      "3",
				SettingKeyAffiliateBronzeRate:          "10",
				SettingKeyAffiliateSilverInvitees:      "10",
				SettingKeyAffiliateSilverRate:          "12",
				SettingKeyAffiliateGoldInvitees:        "30",
				SettingKeyAffiliateGoldRate:            "15",
			}}
			affiliateRepo := &paymentFulfillmentAffiliateRepoStub{reconcileErr: errors.New("refresh failed")}
			svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(affiliateRepo, NewSettingService(settings, nil), nil, nil)}
			lease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
			require.NoError(t, err)
			require.NotNil(t, lease)

			require.NoError(t, svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS"))
			reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusCompleted, reloaded.Status)
			require.True(t, affiliateRepo.reconcileRequired)
			require.Equal(t, int64(1), affiliateRepo.generation)
		})
	}
}

func TestMarkCompletedPersistsDirtyOutboxWhenGenerationMarkerFails(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	repo := &paymentFulfillmentAffiliateRepoStub{
		reconcileErr:     errors.New("local reconcile unavailable"),
		markReconcileErr: errors.New("generation unavailable"),
	}
	svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(repo, nil, nil, nil)}
	lease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
	require.NoError(t, err)

	err = svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS")

	require.NoError(t, err)
	reloaded, reloadErr := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, reloadErr)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	dirty, auditErr := client.PaymentAuditLog.Query().Where(
		paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
		paymentauditlog.ActionEQ("AFFILIATE_QUALIFICATION_DIRTY"),
	).Count(ctx)
	require.NoError(t, auditErr)
	require.Equal(t, 1, dirty)
}

func TestMarkCompletedDirtyOutboxRecoversAfterCommitWithoutLocalReconcile(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	svc := &PaymentService{entClient: client}
	lease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
	require.NoError(t, err)

	require.NoError(t, svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS"))
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	dirtyBefore, err := client.PaymentAuditLog.Query().Where(paymentauditlog.ActionEQ(AffiliateQualificationDirtyAuditAction)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, dirtyBefore)

	repo := &paymentFulfillmentAffiliateRepoStub{auditClient: client}
	recoverySvc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(repo, nil, nil, nil)}
	require.NoError(t, recoverySvc.ExecuteSubscriptionFulfillment(ctx, order.ID))
	require.Equal(t, 1, repo.reconcileCalls)
	dirtyAfter, err := client.PaymentAuditLog.Query().Where(paymentauditlog.ActionEQ(AffiliateQualificationDirtyAuditAction)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, dirtyAfter)
	require.False(t, repo.reconcileRequired, "draining an outbox event must not create or clear a global marker")
}

func TestMarkCompletedRollsBackStatusWhenDirtyOutboxCannotPersist(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	_, err := client.ExecContext(ctx, `
CREATE TRIGGER fail_affiliate_dirty_audit
BEFORE INSERT ON payment_audit_logs
WHEN NEW.action = 'AFFILIATE_QUALIFICATION_DIRTY'
BEGIN
    SELECT RAISE(FAIL, 'injected dirty audit failure');
END`)
	require.NoError(t, err)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	svc := &PaymentService{entClient: client}
	lease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
	require.NoError(t, err)

	err = svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS")

	require.ErrorContains(t, err, "persist affiliate qualification dirty audit")
	reloaded, reloadErr := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, reloadErr)
	require.Equal(t, OrderStatusRecharging, reloaded.Status)
}

func TestAffiliateQualificationDirtyOutboxUpsertsLatestTerminalEvent(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	svc := &PaymentService{entClient: client}
	lease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
	require.NoError(t, err)
	require.NoError(t, svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS"))
	completed, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)

	plan := &RefundPlan{OrderID: order.ID, Order: completed, RefundAmount: completed.Amount / 2, Reason: "partial"}
	_, err = svc.markRefundOk(ctx, plan)
	require.NoError(t, err)
	_, err = svc.markRefundOk(ctx, plan)
	require.NoError(t, err)

	logs, err := client.PaymentAuditLog.Query().Where(
		paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
		paymentauditlog.ActionEQ(AffiliateQualificationDirtyAuditAction),
	).All(ctx)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	var event AffiliateQualificationDirtyEvent
	require.NoError(t, json.Unmarshal([]byte(logs[0].Detail), &event))
	require.Equal(t, "refund_completed", event.EventType)
	require.Equal(t, OrderStatusPartiallyRefunded, event.OrderStatus)
}

func TestMarkCompletedStaleLocalTokenKeepsConcurrentDirtyGeneration(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	repo := &paymentFulfillmentAffiliateRepoStub{}
	repo.reconcileInviteeFn = func(ctx context.Context, userID int64, threshold float64) (*AffiliateQualification, error) {
		_, err := repo.MarkReconcileRequired(ctx)
		return &AffiliateQualification{InviteeUserID: userID, QualifyingPaymentAmount: threshold}, err
	}
	svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(repo, nil, nil, nil)}
	lease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
	require.NoError(t, err)

	require.NoError(t, svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS"))
	require.True(t, repo.reconcileRequired)
	require.Equal(t, int64(1), repo.generation)
	require.NoError(t, svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS"))
	require.Equal(t, int64(1), repo.generation, "completed retry must not bump a new generation")
}

func TestMarkCompletedLocalClearRespectsPriorPendingState(t *testing.T) {
	for _, tt := range []struct {
		name         string
		wasPending   bool
		wantRequired bool
	}{
		{name: "clean_before_terminal", wasPending: false, wantRequired: false},
		{name: "pending_before_terminal", wasPending: true, wantRequired: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
			repo := &paymentFulfillmentAffiliateRepoStub{reconcileRequired: tt.wasPending, generation: 10}
			svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(repo, nil, nil, nil)}
			lease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
			require.NoError(t, err)

			require.NoError(t, svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS"))
			require.Equal(t, int64(10), repo.generation)
			require.Equal(t, tt.wantRequired, repo.reconcileRequired)
			require.Equal(t, 1, repo.reconcileCalls)
			require.NoError(t, svc.markCompleted(ctx, order, lease, "PAYMENT_SUCCESS"))
			require.Equal(t, int64(10), repo.generation, "completed retry must not bump generation")
		})
	}
}

func TestExecuteSubscriptionFulfillmentAppliesAffiliateRebate(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	user, err := client.User.Create().
		SetEmail("subscription-affiliate@example.com").
		SetPasswordHash("hash").
		SetUsername("subscription-affiliate-user").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(9.99).
		SetPayAmount(71.36).
		SetFeeRate(0).
		SetRechargeCode("PAY-SUB-AFFILIATE").
		SetOutTradeNo("sub2_subscription_affiliate").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-sub-affiliate").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(99).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(OrderStatusPaid).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	inviterID := int64(9001)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		inviteeSummary: &AffiliateSummary{
			UserID:    user.ID,
			AffCode:   "INVITEE",
			InviterID: &inviterID,
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
		inviterSummary: &AffiliateSummary{
			UserID:    inviterID,
			AffCode:   "INVITER",
			CreatedAt: time.Now().Add(-48 * time.Hour),
		},
	}
	settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:           "true",
		SettingKeyAffiliateRebateRate:        "15",
		SettingKeyAffiliateRebateFreezeHours: "0",
	}}, nil)
	subRepo := newSubscriptionUserSubRepoStub()
	subscriptionSvc := NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)
	svc := &PaymentService{
		entClient:        client,
		groupRepo:        &subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}},
		subscriptionSvc:  subscriptionSvc,
		affiliateService: NewAffiliateService(affiliateRepo, settingSvc, nil, nil),
	}

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Len(t, affiliateRepo.accrueCalls, 1)
	require.Equal(t, inviterID, affiliateRepo.accrueCalls[0].inviterID)
	require.Equal(t, user.ID, affiliateRepo.accrueCalls[0].inviteeUserID)
	require.InDelta(t, 1.4985, affiliateRepo.accrueCalls[0].amount, 0.00000001)
	require.NotNil(t, affiliateRepo.accrueCalls[0].sourceOrderID)
	require.Equal(t, order.ID, *affiliateRepo.accrueCalls[0].sourceOrderID)
	require.Equal(t, 1, subRepo.createCalls)

	applied, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("AFFILIATE_REBATE_APPLIED")).
		Only(ctx)
	require.NoError(t, err)
	require.Contains(t, applied.Detail, `"baseAmount":9.99`)
	require.Contains(t, applied.Detail, `"rebateAmount":1.4985`)
}

func TestExecuteSubscriptionFulfillmentDoesNotDuplicateWorkAfterLegacySuccessAudit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	user, err := client.User.Create().
		SetEmail("subscription-affiliate-idempotent@example.com").
		SetPasswordHash("hash").
		SetUsername("subscription-affiliate-idempotent-user").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(80).
		SetPayAmount(80).
		SetFeeRate(0).
		SetRechargeCode("PAY-SUB-AFFILIATE-IDEMPOTENT").
		SetOutTradeNo("sub2_subscription_affiliate_idempotent").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-sub-affiliate-idempotent").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(100).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(30).
		SetStatus(OrderStatusPaid).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("SUBSCRIPTION_SUCCESS").
		SetDetail(`{"groupID":7,"validityDays":30}`).
		SetOperator("system").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("AFFILIATE_REBATE_APPLIED").
		SetDetail(`{"baseAmount":80,"rebateAmount":16}`).
		SetOperator("system").
		Save(ctx)
	require.NoError(t, err)

	inviterID := int64(9001)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		inviteeSummary: &AffiliateSummary{
			UserID:    user.ID,
			AffCode:   "INVITEE",
			InviterID: &inviterID,
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
		inviterSummary: &AffiliateSummary{
			UserID:    inviterID,
			AffCode:   "INVITER",
			CreatedAt: time.Now().Add(-48 * time.Hour),
		},
	}
	settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateRate: "20",
	}}, nil)
	subRepo := newSubscriptionUserSubRepoStub()
	subscriptionSvc := NewSubscriptionService(&subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}, subRepo, nil, nil, nil)
	svc := &PaymentService{
		entClient:        client,
		groupRepo:        &subscriptionGroupRepoStub{group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription}},
		subscriptionSvc:  subscriptionSvc,
		affiliateService: NewAffiliateService(affiliateRepo, settingSvc, nil, nil),
	}

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Empty(t, affiliateRepo.accrueCalls)
	require.Zero(t, subRepo.createCalls)
}

var _ AffiliateRepository = (*paymentFulfillmentAffiliateRepoStub)(nil)
var _ SettingRepository = (*paymentFulfillmentSettingRepoStub)(nil)
