# Consumption-Linked Check-in Rewards Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a small random check-in reward plus an auditable rebate based on the user's preceding Beijing-calendar-day usage, with configurable caps and consumption-linked streak bonuses.

**Architecture:** Extend the existing check-in JSON configuration and daily record rather than introducing a second reward subsystem. The check-in transaction will aggregate the user's preceding-day `usage_logs.actual_cost`, calculate each reward component with pure helpers, atomically credit the balance, and persist every calculation input/output so retries and support queries reproduce the original result.

**Tech Stack:** Go, Ent, PostgreSQL migrations, Gin, Vue 3, TypeScript, Vitest, Tailwind CSS, vue-i18n.

---

### Task 1: Persist Usage-Linked Reward Details

**Files:**
- Create: `backend/migrations/193_user_checkin_usage_rebate.sql`
- Modify: `backend/ent/schema/user_checkin.go`
- Test: `backend/migrations/user_checkin_usage_rebate_migration_test.go`
- Generated: `backend/ent/usercheckin.go`
- Generated: `backend/ent/usercheckin_create.go`
- Generated: `backend/ent/usercheckin_update.go`
- Generated: `backend/ent/usercheckin/where.go`
- Generated: other Ent files changed by `go generate ./ent`

**Step 1: Write the failing migration contract test**

Assert that migration `193_user_checkin_usage_rebate.sql` exists and contains idempotent decimal columns:

```go
func TestUserCheckinUsageRebateMigration(t *testing.T) {
    raw, err := migrations.FS.ReadFile("193_user_checkin_usage_rebate.sql")
    require.NoError(t, err)
    sql := string(raw)
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS previous_day_usage_amount DECIMAL(20,8) NOT NULL DEFAULT 0")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS usage_rebate_amount DECIMAL(20,8) NOT NULL DEFAULT 0")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reward_cap_adjustment DECIMAL(20,8) NOT NULL DEFAULT 0")
}
```

**Step 2: Run the test to verify it fails**

Run: `cd backend && go test ./migrations -run TestUserCheckinUsageRebateMigration -count=1`

Expected: FAIL because migration 193 does not exist.

**Step 3: Add the idempotent migration and Ent fields**

Use three non-null `DECIMAL(20,8)` columns with zero defaults. Add matching Ent float fields using the same PostgreSQL schema type as the existing reward columns:

```go
field.Float("previous_day_usage_amount").
    SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
    Default(0),
field.Float("usage_rebate_amount").
    SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
    Default(0),
field.Float("reward_cap_adjustment").
    SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
    Default(0),
```

Do not add another usage-log index: `idx_usage_logs_user_created` already covers `(user_id, created_at)`.

**Step 4: Regenerate Ent and run focused tests**

Run: `cd backend && go generate ./ent`

Run: `cd backend && go test ./migrations ./ent/... -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/migrations/193_user_checkin_usage_rebate.sql \
  backend/migrations/user_checkin_usage_rebate_migration_test.go \
  backend/ent/schema/user_checkin.go backend/ent
git commit -m "feat(checkin): persist usage rebate details"
```

### Task 2: Extend And Validate Check-in Configuration

**Files:**
- Modify: `backend/internal/service/checkin_service.go`
- Modify: `backend/internal/handler/admin/checkin_handler.go`
- Test: `backend/internal/service/checkin_service_test.go`
- Test: `backend/internal/server/api_contract_test.go`

**Step 1: Write failing configuration tests**

Cover all of these cases:

```go
func TestCheckinConfig_NormalizesUsageRebateMode(t *testing.T) {
    cfg := validCheckinConfig()
    cfg.UsageRebateEnabled = true
    cfg.UsageRebateRatePercent = 8
    cfg.UsageRebateCap = 8
    cfg.TotalRewardCap = 10
    cfg.StreakRules = []CheckinStreakRule{{Day: 7, BonusRatePercent: 10}}
    normalized, err := normalizeCheckinConfig(cfg)
    require.NoError(t, err)
    require.Equal(t, 8.0, normalized.UsageRebateRatePercent)
}

func TestCheckinConfig_LegacyFixedStreakRemainsValid(t *testing.T) {
    cfg := validCheckinConfig()
    cfg.UsageRebateEnabled = false
    cfg.StreakRules = []CheckinStreakRule{{Day: 7, BonusAmount: 4}}
    _, err := normalizeCheckinConfig(cfg)
    require.NoError(t, err)
}
```

Also reject NaN/infinity, percentages outside 0-100, enabled mode with non-positive caps, total cap below the minimum random tier, duplicate streak days, and usage-linked streak rules with non-positive percentages.

**Step 2: Run the focused tests to verify failure**

Run: `cd backend && go test ./internal/service -run 'TestCheckinConfig_' -count=1`

Expected: FAIL because the new fields do not exist.

**Step 3: Extend service and handler DTOs**

Add these fields to `CheckinConfig`:

```go
UsageRebateEnabled     bool    `json:"usage_rebate_enabled"`
UsageRebateRatePercent float64 `json:"usage_rebate_rate_percent"`
UsageRebateCap         float64 `json:"usage_rebate_cap"`
TotalRewardCap         float64 `json:"total_reward_cap"`
```

Add `BonusRatePercent float64` to `CheckinStreakRule` while retaining `BonusAmount` for legacy mode. Include the new fields in `mustMarshalCheckinRewardConfig`, `GetConfig`, `UpdateConfig`, and `UpdateCheckinConfigRequest`.

Missing JSON fields must resolve to `UsageRebateEnabled=false`, preserving current production payouts. Set the code defaults for a fresh configuration to the approved values, but never overwrite an existing setting automatically.

**Step 4: Add API contract assertions and run tests**

Assert the admin check-in config response/request includes all new snake-case fields and accepts percentage streak rules.

Run: `cd backend && go test ./internal/service ./internal/server -run 'Checkin|checkin' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/checkin_service.go \
  backend/internal/service/checkin_service_test.go \
  backend/internal/handler/admin/checkin_handler.go \
  backend/internal/server/api_contract_test.go
git commit -m "feat(checkin): configure usage-linked rewards"
```

### Task 3: Calculate Preceding-Day Usage And Reward Components

**Files:**
- Modify: `backend/internal/service/checkin_service.go`
- Test: `backend/internal/service/checkin_service_test.go`

**Step 1: Write failing pure-calculation tests**

Table-test the approved calculation:

```go
tests := []struct {
    name string
    base, usage, rate, rebateCap, streakRate, totalCap float64
    wantRebate, wantStreak, wantTotal, wantCapAdjustment float64
}{
    {"zero usage", .50, 0, 8, 8, 10, 10, 0, 0, .50, 0},
    {"normal usage", .80, 50, 8, 8, 0, 10, 4, 0, 4.80, 0},
    {"rebate cap", .30, 500, 8, 8, 0, 10, 8, 0, 8.30, 32},
    {"streak and total cap", 3, 100, 8, 8, 20, 10, 7, 0, 10, 2.60},
}
```

Use named fields in the real test. Define `reward_cap_adjustment` as total raw reward removed across the rebate cap and final cap, keeping it non-negative.

Write a timezone aggregation test with usage immediately before, inside, and after the preceding Beijing day. Only the inside records may contribute.

**Step 2: Run the focused tests to verify failure**

Run: `cd backend && go test ./internal/service -run 'TestCalculateUsageLinkedCheckinReward|TestPreviousBeijingDayUsage' -count=1`

Expected: FAIL because helpers are absent.

**Step 3: Implement calculation helpers**

Introduce a small internal result type:

```go
type checkinRewardCalculation struct {
    PreviousDayUsage float64
    BaseReward       float64
    UsageRebate      float64
    StreakBonus      float64
    TotalReward      float64
    CapAdjustment    float64
}
```

Implement Beijing boundaries from the check-in date, query `SUM(actual_cost)` using `UserIDEQ`, `CreatedAtGTE`, and `CreatedAtLT`, clamp the aggregate to zero, and round monetary outputs through the existing scaled-money helper. Do not use string date matching in SQL.

When applying the final cap, preserve base first, then rebate, then streak so stored component values sum exactly to `TotalReward`.

**Step 4: Run service tests**

Run: `cd backend && go test ./internal/service -run Checkin -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/checkin_service.go backend/internal/service/checkin_service_test.go
git commit -m "feat(checkin): calculate previous-day usage rebate"
```

### Task 4: Integrate The Calculation Into Atomic Check-in Fulfillment

**Files:**
- Modify: `backend/internal/service/checkin_service.go`
- Test: `backend/internal/service/checkin_service_test.go`

**Step 1: Write failing transaction and idempotency tests**

Add tests proving:

- the credited balance equals base + persisted rebate + persisted streak bonus;
- record fields contain preceding-day usage, rebate, and cap adjustment;
- a retry returns the stored values even if new usage logs are inserted afterward;
- a usage aggregation error rolls back without a balance update, check-in row, or redeem history row;
- two concurrent submissions result in one credit and one stored record.

**Step 2: Run tests and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestCheckinService_Checkin_(UsageRebate|IdempotentUsageSnapshot|UsageQueryRollback|Concurrent)' -count=1`

Expected: FAIL on missing persisted values or incorrect credits.

**Step 3: Integrate within the existing transaction**

After eligibility and streak counters are resolved, calculate the usage-linked reward through the transaction client. Persist with:

```go
SetPreviousDayUsageAmount(calc.PreviousDayUsage).
SetBaseRewardAmount(calc.BaseReward).
SetUsageRebateAmount(calc.UsageRebate).
SetBonusRewardAmount(calc.StreakBonus).
SetRewardCapAdjustment(calc.CapAdjustment).
SetTotalRewardAmount(calc.TotalReward).
SetRewardAmount(calc.TotalReward)
```

Update `CheckinStatus` and `CheckinRecord` with:

```go
PreviousDayUsageAmount float64 `json:"previous_day_usage_amount"`
UsageRebateAmount      float64 `json:"usage_rebate_amount"`
RewardCapAdjustment    float64 `json:"reward_cap_adjustment"`
EstimatedUsageRebate   float64 `json:"estimated_usage_rebate,omitempty"`
```

`GetStatus` computes the current preceding-day estimate before check-in; after check-in and on retries it returns the immutable stored values. Update `checkinRecordFromEntity` and `alreadyCheckedInResult` accordingly.

**Step 4: Run all backend check-in tests**

Run: `cd backend && go test ./internal/service ./internal/handler/... ./internal/server -run 'Checkin|checkin' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/checkin_service.go backend/internal/service/checkin_service_test.go
git commit -m "feat(checkin): credit and audit usage rebates"
```

### Task 5: Expose Admin Configuration, Preview, And Record Details

**Files:**
- Modify: `frontend/src/api/admin/checkins.ts`
- Modify: `frontend/src/api/__tests__/admin.checkins.spec.ts`
- Modify: `frontend/src/views/admin/CheckinsView.vue`
- Modify: `frontend/src/views/admin/__tests__/CheckinsView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/restored.ts`
- Modify: `frontend/src/i18n/locales/en/restored.ts`

**Step 1: Write failing admin UI tests**

Test that the form loads and submits:

```ts
{
  usage_rebate_enabled: true,
  usage_rebate_rate_percent: 8,
  usage_rebate_cap: 8,
  total_reward_cap: 10,
  streak_rules: [{ day: 7, bonus_rate_percent: 10 }]
}
```

Assert validation rejects negative/non-finite values, rate over 100, a total cap below the smallest random tier, and an enabled mode with zero caps. Assert preview examples display rewards for `$0`, `$10`, `$20`, `$50`, and `$100` preceding-day usage.

Assert the record detail displays base reward, preceding-day usage, usage rebate, streak bonus, cap adjustment, and total.

**Step 2: Run tests to verify failure**

Run: `cd frontend && npm run test -- --run src/api/__tests__/admin.checkins.spec.ts src/views/admin/__tests__/CheckinsView.spec.ts`

Expected: FAIL because the API types and controls are absent.

**Step 3: Implement types and admin controls**

Extend `AdminCheckinConfig`, `CheckinStreakRule`, and `AdminCheckinRecord`. Add a restrained configuration section using a toggle for usage rebate mode and numeric inputs for percentage/caps. In usage-linked mode, render streak rules as percentage fields; in legacy mode preserve fixed amount fields.

Calculate preview rows client-side with the exact backend order and show the random expected value separately from deterministic usage rebate. Do not nest the section inside an existing card; use the established unframed admin form section pattern.

**Step 4: Run focused tests and type checking**

Run: `cd frontend && npm run test -- --run src/api/__tests__/admin.checkins.spec.ts src/views/admin/__tests__/CheckinsView.spec.ts`

Run: `cd frontend && npx vue-tsc -b --pretty false`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/checkins.ts frontend/src/api/__tests__/admin.checkins.spec.ts \
  frontend/src/views/admin/CheckinsView.vue frontend/src/views/admin/__tests__/CheckinsView.spec.ts \
  frontend/src/i18n/locales/zh/restored.ts frontend/src/i18n/locales/en/restored.ts
git commit -m "feat(checkin): manage usage rebate settings"
```

### Task 6: Show Yesterday's Usage And Reward Breakdown To Users

**Files:**
- Modify: `frontend/src/api/checkin.ts`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/restored.ts`
- Modify: `frontend/src/i18n/locales/en/restored.ts`

**Step 1: Write failing user experience tests**

Before check-in, mock status values and assert the popover shows preceding-day usage and estimated deterministic rebate while leaving the random reward unknown. After submission, assert this breakdown:

```text
Random reward $0.80
Yesterday's usage $50.00
Usage rebate $4.00
Streak bonus $0.00
Credited today $4.80
```

Also test zero usage, a capped rebate, recent-record details, and a 320px viewport/source-level responsive invariant preventing fixed-width truncation.

**Step 2: Run the test to verify failure**

Run: `cd frontend && npm run test -- --run src/components/layout/__tests__/AppHeader.spec.ts`

Expected: FAIL because the new fields are not rendered.

**Step 3: Extend API types and update the popover**

Add the persisted and estimate fields to `CheckinStatus` and `CheckinRecord`. Replace the two-column base/streak-only presentation with a compact breakdown that uses wrapping labels and tabular monetary values. Keep the control usable by keyboard and preserve the current loading, disabled, and already-checked-in states.

Update the success toast to use `total_reward_amount`, not the legacy `reward_amount` fallback unless the total is absent.

**Step 4: Run focused frontend verification**

Run: `cd frontend && npm run test -- --run src/components/layout/__tests__/AppHeader.spec.ts`

Run: `cd frontend && npx vue-tsc -b --pretty false`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/checkin.ts frontend/src/components/layout/AppHeader.vue \
  frontend/src/components/layout/__tests__/AppHeader.spec.ts \
  frontend/src/i18n/locales/zh/restored.ts frontend/src/i18n/locales/en/restored.ts
git commit -m "feat(checkin): explain consumption-linked rewards"
```

### Task 7: Full Verification And Rollout Readiness

**Files:**
- Modify if needed: `docs/plans/2026-08-02-consumption-linked-checkin-rewards-design.md`
- Test: all files changed above

**Step 1: Run backend verification**

Run: `cd backend && go test ./...`

Expected: PASS.

Run: `cd backend && go vet ./...`

Expected: PASS.

**Step 2: Run frontend verification**

Run: `cd frontend && npm run test -- --run`

Expected: PASS.

Run: `cd frontend && npm run build`

Expected: PASS.

**Step 3: Inspect migration and generated-code scope**

Run: `git diff --check`

Run: `git status --short`

Expected: no whitespace errors; only intentional files are modified. Preserve the unrelated root `package.json` and `package-lock.json` as user-owned untracked files.

**Step 4: Re-run the production cost model read-only**

Use the approved 30-day SQL model with random expected value `$0.61`, usage rate 8%, usage cap `$8`, and total cap `$10`. Record projected totals in the deployment handoff; do not change production settings during this task.

**Step 5: Close verification without bundling corrections**

Expected: no new commit is needed. If a verification command fails, return to the task that owns the failing behavior, add a regression test there, and commit only that task's exact files before repeating this verification task. Do not use `git add -A` or include the unrelated root package files.

Production activation is a separate, explicit operation after local verification. Save the approved random tiers and `8% / $8 / $10` settings through the admin API only after deployment approval, then compare actual rebate-to-attributed-usage cost for seven days.
