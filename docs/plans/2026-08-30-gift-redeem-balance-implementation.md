# Gift Redeem Balance Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add administrator-generated gift balance redeem codes whose credit and funded usage remain fully billable/auditable but are excluded from check-in and activity eligibility thresholds.

**Architecture:** Keep `users.balance` as the displayed spendable total and track the exempt subset in `gift_balance` plus `frozen_gift_balance`. Persist an immutable `redeem_codes.threshold_exempt` classification and allocate gift balance first during every usage deduction, recording the allocation in `usage_logs.threshold_exempt_cost`. Only eligibility queries subtract the exempt amount; financial and operational queries remain unchanged.

**Tech Stack:** Go 1.24, Ent, PostgreSQL migrations, Gin, Vue 3, TypeScript, Tailwind CSS, Vitest, Testify/sqlmock.

---

### Task 1: Add backward-compatible database fields

**Files:**
- Create: `backend/migrations/232_gift_redeem_balance_eligibility.sql`
- Create: `backend/migrations/gift_redeem_balance_migration_test.go`
- Modify: `backend/ent/schema/user.go`
- Modify: `backend/ent/schema/redeem_code.go`
- Modify: `backend/ent/schema/usage_log.go`
- Modify: generated files under `backend/ent/`

**Step 1: Write the failing migration test**

Assert that migration 232 adds non-null, zero-default columns for `users.gift_balance`, `users.frozen_gift_balance`, `redeem_codes.threshold_exempt`, and `usage_logs.threshold_exempt_cost`, with nonnegative checks for monetary fields.

```go
func TestMigration232AddsGiftRedeemEligibilityAccounting(t *testing.T) {
    content, err := FS.ReadFile("232_gift_redeem_balance_eligibility.sql")
    require.NoError(t, err)
    sql := string(content)
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS gift_balance")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS frozen_gift_balance")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS threshold_exempt BOOLEAN")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS threshold_exempt_cost")
}
```

**Step 2: Run the migration test and verify it fails**

Run: `cd backend && go test ./migrations -run TestMigration232AddsGiftRedeemEligibilityAccounting -count=1`

Expected: FAIL because the migration file does not exist.

**Step 3: Add the migration and Ent schema fields**

Use `NUMERIC(20,8)` for user wallet fields, `NUMERIC(20,10)` for the usage-log allocation, and safe defaults:

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS gift_balance NUMERIC(20,8) NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS frozen_gift_balance NUMERIC(20,8) NOT NULL DEFAULT 0;
ALTER TABLE redeem_codes ADD COLUMN IF NOT EXISTS threshold_exempt BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS threshold_exempt_cost NUMERIC(20,10) NOT NULL DEFAULT 0;
```

Add idempotent nonnegative constraints using guarded `DO $$ ... $$` blocks. Mirror the fields in Ent with the same defaults and comments.

**Step 4: Regenerate Ent code**

Run: `cd backend && make generate`

Expected: Ent entities, builders, predicates, and mutations include the four fields.

**Step 5: Run schema and migration tests**

Run: `cd backend && go test ./migrations ./ent/schema -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/migrations/232_gift_redeem_balance_eligibility.sql backend/migrations/gift_redeem_balance_migration_test.go backend/ent
git commit -m "feat(db): add gift balance eligibility fields"
```

### Task 2: Propagate gift classification through domain and API models

**Files:**
- Modify: `backend/internal/service/user.go`
- Modify: `backend/internal/service/redeem_code.go`
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/repository/user_repo.go`
- Modify: `backend/internal/repository/redeem_code_repo.go`
- Modify: `backend/internal/repository/usage_log_repo_insert.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Test: `backend/internal/handler/dto/mappers_deleted_user_test.go`
- Test: `backend/internal/repository/usage_log_repo_unit_test.go`

**Step 1: Write failing mapper and persistence tests**

Cover these contracts:

- Admin and workbench-generated redeem DTOs expose `threshold_exempt`.
- Ordinary user redemption history does not expose extra wallet internals.
- Usage-log insert arguments include `threshold_exempt_cost` in every single/batch insert layout.
- Repository entity conversion preserves all new fields.

**Step 2: Run focused tests and verify they fail**

Run: `cd backend && go test -tags=unit ./internal/handler/dto ./internal/repository -run 'Gift|ThresholdExempt' -count=1`

Expected: FAIL because fields/mappings are missing.

**Step 3: Add service fields and repository mappings**

Add:

```go
// User
GiftBalance       float64
FrozenGiftBalance float64

// RedeemCode
ThresholdExempt bool

// UsageLog
ThresholdExemptCost float64
```

Update `redeemCodeEntityToService`, user entity mapping, redeem create/update builders, DTO base mapping, usage-log select/scan code, and every ordered insert column/argument list in `usage_log_repo_insert.go`.

**Step 4: Run focused tests**

Run: `cd backend && go test -tags=unit ./internal/handler/dto ./internal/repository -run 'Gift|ThresholdExempt|UsageLog' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/repository backend/internal/handler/dto
git commit -m "feat: expose gift balance accounting fields"
```

### Task 3: Add gift option to super-admin code generation

**Files:**
- Modify: `backend/internal/handler/admin/redeem_handler.go`
- Modify: `backend/internal/service/admin_service.go`
- Modify: `backend/internal/service/admin_user.go`
- Test: `backend/internal/handler/admin/redeem_handler_test.go`
- Test: `backend/internal/service/admin_service_redeem_batch_test.go`

**Step 1: Write failing request/service tests**

Test that `threshold_exempt: true` is propagated for positive balance codes and rejected for concurrency, subscription, invitation, zero, or negative-value codes. Verify false remains the default.

**Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/handler/admin ./internal/service -run 'GenerateRedeemCodes.*Threshold|GiftRedeem' -count=1`

Expected: FAIL because the request and service input do not carry the flag.

**Step 3: Implement validation and immutable creation**

Add `ThresholdExempt bool` to `GenerateRedeemCodesRequest` and `GenerateRedeemCodesInput`. Validate:

```go
if input.ThresholdExempt && (input.Type != RedeemTypeBalance || input.Value <= 0) {
    return nil, infraerrors.BadRequest(
        "REDEEM_THRESHOLD_EXEMPT_INVALID",
        "gift credit is only supported for positive balance redeem codes",
    )
}
```

Set the flag when constructing every code. Do not add it to batch-update fields.

**Step 4: Run tests**

Run: `cd backend && go test ./internal/handler/admin ./internal/service -run 'GenerateRedeemCodes|GiftRedeem' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/admin/redeem_handler.go backend/internal/service/admin_service.go backend/internal/service/admin_user.go backend/internal/handler/admin/redeem_handler_test.go backend/internal/service/admin_service_redeem_batch_test.go
git commit -m "feat(admin): classify gift balance redeem codes"
```

### Task 4: Add gift option to secondary-admin balance transfers

**Files:**
- Modify: `backend/internal/handler/redeem_handler.go`
- Modify: `backend/internal/service/redeem_service.go`
- Modify: `backend/internal/repository/user_repo.go`
- Test: `backend/internal/service/redeem_service_balance_transfer_test.go`
- Test: `backend/internal/repository/user_repo_redeem_adjustment_test.go`

**Step 1: Write failing funding-integrity tests**

Test that:

- The workbench request propagates `threshold_exempt`.
- Both ordinary and gift transfer codes can use only `balance - gift_balance` from the creator.
- Insufficient ordinary balance fails even when total balance is sufficient.
- Generated codes retain the selected classification.
- Deleting an unused generated code restores ordinary balance without increasing gift balance.

**Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'BalanceTransfer.*Gift|OrdinaryBalance' -count=1`

Expected: FAIL because transfer generation currently checks total balance only.

**Step 3: Add an atomic ordinary-balance deduction capability**

Use a narrow optional repository interface to avoid changing every `UserRepository` test stub:

```go
type OrdinaryBalanceRepository interface {
    DeductOrdinaryBalance(ctx context.Context, userID int64, amount float64) error
}
```

The SQL must update only when `balance - gift_balance >= amount`, lock atomically through the update, and distinguish user-not-found from insufficient ordinary balance.

**Step 4: Propagate the flag and use ordinary funding**

Add `ThresholdExempt` to `GenerateBalanceTransferRedeemCodeRequest` and `GenerateBalanceTransferCodeInput`. Every generated transfer code carries the flag, while creator deduction and unused-code refund preserve ordinary funding.

**Step 5: Run tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'BalanceTransfer|OrdinaryBalance' -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/internal/handler/redeem_handler.go backend/internal/service/redeem_service.go backend/internal/repository/user_repo.go backend/internal/service/redeem_service_balance_transfer_test.go backend/internal/repository/user_repo_redeem_adjustment_test.go
git commit -m "feat(admin): preserve gift source in balance transfers"
```

### Task 5: Credit gift codes without increasing cumulative recharge

**Files:**
- Modify: `backend/internal/service/redeem_service.go`
- Modify: `backend/internal/repository/user_repo.go`
- Modify: `backend/internal/repository/redeem_code_repo.go`
- Test: `backend/internal/service/redeem_service_redeem_test.go`
- Test: `backend/internal/repository/redeem_code_repo_integration_test.go`
- Test: `backend/internal/repository/user_repo_redeem_adjustment_test.go`

**Step 1: Write failing redemption tests**

Verify an ordinary USD 10 code adds USD 10 to balance and cumulative recharge, while a gift USD 10 code adds USD 10 to balance and gift balance but leaves cumulative recharge unchanged. Include transaction rollback on failure.

Also verify `SumPositiveBalanceByUser` excludes gift codes.

**Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'Redeem.*Gift|SumPositiveBalance.*Gift' -count=1`

Expected: FAIL because positive redemption always calls `UpdateBalance`.

**Step 3: Add atomic gift crediting**

Introduce a narrow repository capability:

```go
type GiftBalanceRepository interface {
    CreditGiftBalance(ctx context.Context, userID int64, amount float64) error
}
```

Implement one SQL update that adds to `balance` and `gift_balance` without touching `total_recharged`. In `Redeem`, dispatch to it only for a valid positive balance code with `ThresholdExempt` set.

Update the aggregate query with `redeemcode.ThresholdExemptEQ(false)`.

**Step 4: Run tests**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'Redeem|SumPositiveBalance' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/redeem_service.go backend/internal/repository/user_repo.go backend/internal/repository/redeem_code_repo.go backend/internal/service/redeem_service_redeem_test.go backend/internal/repository/redeem_code_repo_integration_test.go backend/internal/repository/user_repo_redeem_adjustment_test.go
git commit -m "feat: credit gift codes outside cumulative recharge"
```

### Task 6: Allocate gift balance in the unified request billing transaction

**Files:**
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/service/gateway_usage_billing.go`
- Test: `backend/internal/repository/usage_billing_repo_unit_test.go`
- Test: `backend/internal/repository/usage_billing_repo_integration_test.go`
- Test: `backend/internal/service/gateway_usage_billing_test.go`

**Step 1: Write failing allocation tests**

Cover full gift, mixed, no gift, overdraft, and concurrent deduction cases. For cost USD 12 against USD 10 gift balance, assert:

```go
require.InDelta(t, 10, result.ThresholdExemptCost, 1e-8)
require.InDelta(t, 0, storedUser.GiftBalance, 1e-8)
require.InDelta(t, 12, usageLog.ActualCost, 1e-8)
require.InDelta(t, 10, usageLog.ThresholdExemptCost, 1e-8)
```

**Step 2: Run tests and verify they fail**

Run: `cd backend && go test -tags=unit ./internal/repository ./internal/service -run 'UsageBilling.*Gift|Gateway.*ThresholdExempt' -count=1`

Expected: FAIL because the billing result has no allocation field.

**Step 3: Extend the billing result**

Add `ThresholdExemptCost float64` to `UsageBillingApplyResult`. Quantize it to the existing eight-decimal billing scale.

**Step 4: Replace balance deduction with a locked allocation update**

Use a CTE/`FOR UPDATE` statement that reads the old wallet state and returns new total balance, sufficiency, and `LEAST(old_gift_balance, cost)` in one transaction. Update `gift_balance` to `GREATEST(old_gift_balance - cost, 0)`.

Do not alter subscription billing.

**Step 5: Attach allocation before persisting the usage log**

After `repo.Apply`, assign:

```go
usageLog.ThresholdExemptCost = result.ThresholdExemptCost
```

The write remains best effort, but billing and allocation remain atomic in the database transaction.

**Step 6: Run tests**

Run: `cd backend && go test -tags=unit ./internal/repository ./internal/service -run 'UsageBilling|Gateway' -count=1`

Expected: PASS.

**Step 7: Commit**

```bash
git add backend/internal/service/usage_billing.go backend/internal/repository/usage_billing_repo.go backend/internal/service/gateway_usage_billing.go backend/internal/repository/usage_billing_repo_unit_test.go backend/internal/repository/usage_billing_repo_integration_test.go backend/internal/service/gateway_usage_billing_test.go
git commit -m "feat(billing): allocate gift balance per request"
```

### Task 7: Preserve gift attribution in legacy and batch-image billing

**Files:**
- Modify: `backend/internal/repository/user_repo.go`
- Modify: `backend/internal/service/gateway_usage_billing.go`
- Modify: `backend/internal/service/usage_service.go`
- Modify: `backend/internal/repository/usage_billing_repo.go`
- Modify: `backend/internal/service/usage_billing.go`
- Modify: `backend/internal/service/batch_image_billing_hold.go`
- Modify: `backend/internal/service/batch_image_settlement.go`
- Test: `backend/internal/repository/usage_billing_repo_unit_test.go`
- Test: `backend/internal/service/gateway_usage_billing_fallback_test.go`
- Test: `backend/internal/service/batch_image_settlement_test.go`

**Step 1: Write failing fallback and hold tests**

Cover:

- Legacy balance deduction returns the gift-funded portion and writes it to the usage log.
- Reservation moves gift credit into `frozen_gift_balance`.
- Capture consumes frozen gift first and returns the exempt amount.
- Release returns unused frozen gift to available gift balance.
- Mixed concurrent holds preserve aggregate wallet invariants.

**Step 2: Run tests and verify they fail**

Run: `cd backend && go test -tags=unit ./internal/repository ./internal/service -run 'Legacy.*Gift|BatchImage.*Gift' -count=1`

Expected: FAIL because current hold/capture results contain only total balances.

**Step 3: Add gift-aware legacy deduction**

Add an optional `DeductBalanceWithGiftAllocation` repository capability returning `{NewBalance, ThresholdExemptCost}`. Use it in degraded `postUsageBilling` and `UsageService`; retain the old method only for compatibility with unrelated callers/tests.

**Step 4: Extend batch-image settlement results**

Add `ThresholdExemptCost` to `BatchImageBalanceHoldResult`. Reserve, capture, and release update `gift_balance`, `frozen_gift_balance`, `balance`, and `frozen_balance` in the same transaction.

Change `captureBatchImageBalanceHold` to return the result and pass its exempt amount to `BatchImageSettlementService.recordUsageLog`.

**Step 5: Run tests**

Run: `cd backend && go test -tags=unit ./internal/repository ./internal/service -run 'Legacy|BatchImage' -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/internal/repository/user_repo.go backend/internal/service/gateway_usage_billing.go backend/internal/service/usage_service.go backend/internal/repository/usage_billing_repo.go backend/internal/service/usage_billing.go backend/internal/service/batch_image_billing_hold.go backend/internal/service/batch_image_settlement.go backend/internal/repository/usage_billing_repo_unit_test.go backend/internal/service/gateway_usage_billing_fallback_test.go backend/internal/service/batch_image_settlement_test.go
git commit -m "feat(billing): preserve gift attribution across fallback paths"
```

### Task 8: Exclude gift-funded usage from eligibility only

**Files:**
- Modify: `backend/internal/service/checkin_service.go`
- Test: `backend/internal/service/checkin_service_test.go`

**Step 1: Write failing eligibility tests**

Create usage rows where `actual_cost` is greater than `threshold_exempt_cost`. Verify:

- Total cumulative eligibility usage sums `GREATEST(actual_cost - threshold_exempt_cost, 0)`.
- A fully gift-funded user does not satisfy `min_total_usage_usd`.
- A mixed user satisfies only after the ordinary-funded portion reaches the threshold.
- Previous-day usage rebate behavior remains unchanged because the requirement applies to eligibility thresholds, not the separate rebate calculation.

**Step 2: Run tests and verify they fail**

Run: `cd backend && go test ./internal/service -run 'Checkin.*Gift|TotalUsage.*ThresholdExempt' -count=1`

Expected: FAIL because cumulative usage currently sums all `actual_cost`.

**Step 3: Add a reusable eligibility aggregate**

Use an Ent selector expression equivalent to:

```sql
COALESCE(SUM(GREATEST(actual_cost - threshold_exempt_cost, 0)), 0)
```

Apply it to `totalUsageUSDWithClient` only. Keep administrator, finance, rankings, and previous-day rebate aggregates on full `actual_cost`.

**Step 4: Run tests**

Run: `cd backend && go test ./internal/service -run 'Checkin' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/checkin_service.go backend/internal/service/checkin_service_test.go
git commit -m "feat(checkin): exclude gift-funded eligibility usage"
```

### Task 9: Add responsive administrator controls and badges

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/redeem.ts`
- Modify: `frontend/src/api/redeem.ts`
- Modify: `frontend/src/views/admin/RedeemView.vue`
- Modify: `frontend/src/views/admin/AdminWorkbenchView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/en/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`
- Test: `frontend/src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts`
- Test: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`
- Test: `frontend/src/api/__tests__/redeem.spec.ts`

**Step 1: Write failing UI/API tests**

Assert that:

- The super-admin toggle appears only when type is `balance`.
- Both forms default the toggle to off and reset it after generation.
- API payloads serialize `threshold_exempt`.
- Generated-code rows show a gift badge when true.
- Mobile markup uses full-width labels, stable spacing, and no horizontal overflow.

**Step 2: Run tests and verify they fail**

Run: `cd frontend && pnpm test:run src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts src/views/admin/__tests__/AdminWorkbenchView.spec.ts src/api/__tests__/redeem.spec.ts`

Expected: FAIL because types, controls, and payload fields are missing.

**Step 3: Add API types and serialization**

Add `threshold_exempt?: boolean` to `RedeemCode`, `GenerateRedeemCodesRequest`, and `GenerateBalanceTransferCodeRequest`. Add a final boolean argument/options object to admin generation without changing existing defaults.

**Step 4: Build the responsive controls**

Use the existing `admin-form-section`/choice styling rather than introducing a new card. Label it `赠送额度（不计活动门槛）`, include concise help text, and keep the checkbox/toggle aligned at narrow widths. Add a compact badge to both generated-code lists.

**Step 5: Run UI tests and type checks**

Run: `cd frontend && pnpm test:run src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts src/views/admin/__tests__/AdminWorkbenchView.spec.ts src/api/__tests__/redeem.spec.ts`

Run: `cd frontend && pnpm typecheck`

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src
git commit -m "feat(admin): add gift redeem code controls"
```

### Task 10: Verify invariants and the end-to-end workflow

**Files:**
- Modify as needed: tests touched in Tasks 1-9

**Step 1: Run backend formatting and generated-code checks**

Run: `cd backend && gofmt -w ent/schema/*.go internal/service/*.go internal/repository/*.go internal/handler/admin/*.go internal/handler/dto/*.go migrations/*.go`

Run: `cd backend && make generate && git diff --check`

Expected: no formatting or uncommitted generated drift beyond intentional changes.

**Step 2: Run focused backend suites**

Run: `cd backend && go test ./migrations ./internal/service ./internal/repository ./internal/handler/admin ./internal/handler/dto -count=1`

Run: `cd backend && go test -tags=unit ./internal/service ./internal/repository -count=1`

Expected: PASS.

**Step 3: Run frontend verification**

Run: `cd frontend && pnpm test:run`

Run: `cd frontend && pnpm typecheck && pnpm build`

Expected: PASS.

**Step 4: Perform database-backed smoke tests**

With a local migrated database:

1. Generate and redeem an ordinary USD 10 balance code; verify balance and `total_recharged` both increase by USD 10.
2. Generate and redeem a gift USD 10 balance code; verify balance and `gift_balance` increase while `total_recharged` does not.
3. Submit a USD 12 balance-billed request with USD 10 gift balance; verify usage `actual_cost = 12`, `threshold_exempt_cost = 10`, and eligibility usage increases by USD 2.
4. Verify administrator usage totals still increase by the full USD 12.
5. Verify a batch image hold/capture/release preserves total and frozen gift balances.

**Step 5: Inspect the responsive UI**

Run the local frontend and inspect both pages at 390x844 and desktop widths. Confirm conditional visibility, readable help text, badge alignment, and no horizontal scrolling.

**Step 6: Commit final test adjustments**

```bash
git add backend frontend
git commit -m "test: verify gift balance eligibility accounting"
```

**Step 7: Review deployment readiness**

Run: `git status --short && git log --oneline -10`

Expected: clean worktree, migration 232 included, and all feature commits present. Deploy the database migration and backend before exposing the frontend toggle.
