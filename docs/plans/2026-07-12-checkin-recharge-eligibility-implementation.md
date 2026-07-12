# Check-in Recharge Eligibility Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let users check in after reaching either the configured cumulative usage threshold or a new cumulative recharge threshold.

**Architecture:** Extend the existing settings-backed check-in configuration and status DTOs with recharge fields, reading progress from `users.total_recharged`. Centralize OR eligibility in one helper used by status, submission, transactional recheck, and already-checked-in responses; then expose the additive fields in the existing Vue administrator and user interfaces.

**Tech Stack:** Go, Ent, Gin, PostgreSQL test database, Vue 3, TypeScript, Vitest, Vue Test Utils, vue-i18n, Vite, Docker Compose.

---

### Task 1: Specify recharge eligibility in backend tests

**Files:**
- Modify: `backend/internal/service/checkin_service_test.go`

**Step 1: Extend the test setting helper**

Store `SettingKeyCheckinMinTotalRechargeUSD` from `CheckinConfig.MinTotalRechargeUSD` alongside the existing usage setting.

**Step 2: Write failing table-driven eligibility tests**

Cover these cases directly through status and check-in behavior:

```go
tests := []struct {
    name              string
    minUsage         float64
    minRecharge      float64
    totalUsage       float64
    totalRecharged   float64
    eligible         bool
}{
    {"usage route met", 5, 10, 5, 0, true},
    {"recharge route met", 5, 10, 0, 10, true},
    {"neither route met", 5, 10, 4, 9, false},
    {"recharge only enabled", 0, 10, 100, 9, false},
    {"usage only remains compatible", 5, 0, 5, 0, true},
    {"both disabled is unrestricted", 0, 0, 0, 0, true},
}
```

Assert status includes `min_total_recharge_usd`, `total_recharge_usd`, and the combined ineligible reason when neither route is met. Add a negative recharge threshold validation test.

**Step 3: Run the focused tests and confirm failure**

Run: `cd backend && go test ./internal/service -run 'TestCheckinService_(RechargeEligibility|MinimumSpend|UpdateConfig)' -count=1`

Expected: FAIL because the recharge setting, config field, status field, and combined helper do not exist.

### Task 2: Implement backend configuration and eligibility

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/checkin_service.go`
- Modify: `backend/internal/handler/admin/checkin_handler.go`

**Step 1: Add the new setting and DTO fields**

Add `SettingKeyCheckinMinTotalRechargeUSD = "checkin_min_total_recharge_usd"`, `MinTotalRechargeUSD` to `CheckinConfig`, and both `MinTotalRechargeUSD` and `TotalRechargeUSD` to `CheckinStatus`. Add the request field with `binding:"gte=0"` in the administrator handler.

**Step 2: Load, validate, and persist the setting**

Include the setting in `GetMultiple`, parse it as a non-negative float, set a zero default, copy it through normalization, and persist it in `UpdateConfig`. Use a dedicated `INVALID_CHECKIN_MIN_TOTAL_RECHARGE_USD` validation error.

**Step 3: Implement centralized OR eligibility**

Use explicit enabled-route logic so zero disables an individual route while two zero thresholds remain unrestricted:

```go
func checkinSpendEligible(totalUsageUSD, minUsageUSD, totalRechargeUSD, minRechargeUSD float64) bool {
    usageEnabled := minUsageUSD > 0
    rechargeEnabled := minRechargeUSD > 0
    if !usageEnabled && !rechargeEnabled {
        return true
    }
    return (usageEnabled && totalUsageUSD >= minUsageUSD) ||
        (rechargeEnabled && totalRechargeUSD >= minRechargeUSD)
}
```

Read `TotalRecharged` from the user entity before status/submission and again through the transaction client. Apply the helper consistently in `GetStatus`, `Checkin`, the transaction recheck, and `alreadyCheckedInResult`. Return a combined `insufficient_usage_or_recharge` reason and corresponding service error.

**Step 4: Run focused backend tests**

Run: `cd backend && go test ./internal/service ./internal/handler/admin -run 'Checkin' -count=1`

Expected: PASS.

**Step 5: Format backend files**

Run: `gofmt -w backend/internal/service/domain_constants.go backend/internal/service/checkin_service.go backend/internal/service/checkin_service_test.go backend/internal/handler/admin/checkin_handler.go`

### Task 3: Specify administrator UI behavior

**Files:**
- Modify: `frontend/src/views/admin/__tests__/CheckinsView.spec.ts`

**Step 1: Extend mocked configuration**

Add `min_total_recharge_usd` to mock responses and assert the value is loaded into a dedicated input.

**Step 2: Write failing save and validation tests**

Assert saving sends both thresholds and a negative/non-finite recharge threshold is rejected without an API call.

**Step 3: Run the focused test and confirm failure**

Run: `cd frontend && pnpm test:run src/views/admin/__tests__/CheckinsView.spec.ts`

Expected: FAIL because the recharge configuration control and API type do not exist.

### Task 4: Implement administrator recharge configuration

**Files:**
- Modify: `frontend/src/api/admin/checkins.ts`
- Modify: `frontend/src/views/admin/CheckinsView.vue`
- Modify: `frontend/src/i18n/locales/zh/restored.ts`
- Modify: `frontend/src/i18n/locales/en/restored.ts`

**Step 1: Extend the TypeScript API contract**

Add `min_total_recharge_usd: number` to `AdminCheckinConfig`.

**Step 2: Add the form field and state flow**

Place a second non-negative USD input beside/below the usage input using the existing input styling. Initialize, load, validate, submit, and refresh `configForm.min_total_recharge_usd` with the rest of the configuration.

**Step 3: Add localized labels and hints**

Explain that usage or recharge may satisfy eligibility and that recharge uses cumulative credited balance.

**Step 4: Run the focused administrator test**

Run: `cd frontend && pnpm test:run src/views/admin/__tests__/CheckinsView.spec.ts`

Expected: PASS.

### Task 5: Specify and implement user eligibility progress

**Files:**
- Modify: `frontend/src/api/checkin.ts`
- Modify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/i18n/locales/zh/restored.ts`
- Modify: `frontend/src/i18n/locales/en/restored.ts`

**Step 1: Write failing component tests**

Extend status fixtures with recharge fields. Assert both enabled progress rows render, a recharge-satisfied user gets an enabled check-in button, disabled criteria are omitted, and the combined locked tooltip/message contains both current values and thresholds.

**Step 2: Run the focused test and confirm failure**

Run: `cd frontend && pnpm test:run src/components/layout/__tests__/AppHeader.spec.ts`

Expected: FAIL because the API fields and recharge progress rendering do not exist.

**Step 3: Extend the user status contract and component state**

Add `min_total_recharge_usd` and `total_recharge_usd` to `CheckinStatus`. Represent usage and recharge as separate enabled criteria with independent percentages; render only criteria with positive thresholds. When both thresholds are zero, retain the no-threshold message.

**Step 4: Update localized user messaging**

Use OR wording for the eligibility title, locked state, satisfied state, and tooltip. Keep the component responsive within the existing popover width.

**Step 5: Run both focused frontend tests**

Run: `cd frontend && pnpm test:run src/components/layout/__tests__/AppHeader.spec.ts src/views/admin/__tests__/CheckinsView.spec.ts`

Expected: PASS.

### Task 6: Verify the complete change

**Files:**
- Modify only if verification exposes an issue in the files above.

**Step 1: Run backend package tests**

Run: `cd backend && go test ./internal/service ./internal/handler/admin ./internal/handler/user -count=1`

Expected: PASS.

**Step 2: Run frontend tests and static checks**

Run: `cd frontend && pnpm test:run`

Run: `cd frontend && pnpm typecheck`

Run: `cd frontend && pnpm build`

Expected: all commands PASS.

**Step 3: Inspect the final diff**

Run: `git diff --check && git status --short && git diff --stat`

Expected: no whitespace errors and only planned files changed.

**Step 4: Verify live local development**

Confirm the existing Vite development server receives the changed frontend modules through HMR. Rebuild and restart only the `sub2api-dev` backend container using the repository's existing local Compose workflow; do not restart PostgreSQL or Redis.

Verify the backend version endpoint, health endpoint, and authenticated check-in status response include both recharge fields and remain healthy.

**Step 5: Commit the implementation**

```bash
git add backend/internal/service/domain_constants.go \
  backend/internal/service/checkin_service.go \
  backend/internal/service/checkin_service_test.go \
  backend/internal/handler/admin/checkin_handler.go \
  frontend/src/api/admin/checkins.ts \
  frontend/src/api/checkin.ts \
  frontend/src/views/admin/CheckinsView.vue \
  frontend/src/views/admin/__tests__/CheckinsView.spec.ts \
  frontend/src/components/layout/AppHeader.vue \
  frontend/src/components/layout/__tests__/AppHeader.spec.ts \
  frontend/src/i18n/locales/zh/restored.ts \
  frontend/src/i18n/locales/en/restored.ts
git commit -m "feat: allow recharge-based daily check-in"
```
