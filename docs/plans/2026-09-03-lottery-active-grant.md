# Lottery Active-User Attempt Grants Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow administrators to preview and grant lottery attempts to all non-deleted users who made any API call during the last 7 or 30 days, while preserving existing selected-user and all-user behavior.

**Architecture:** Extend the lottery grant request with a normalized `target` (`selected`, `all`, or `active`) and `active_days` (`7` or `30`). The repository resolves active users from `users` joined through an indexed `usage_logs(user_id, created_at)` existence predicate inside the grant transaction; a read-only preview capability uses the same resolver. The admin Vue view adds an active target selector, window selector, preview count, and submits the new fields through the existing idempotent grant endpoint.

**Tech Stack:** Go, Gin, Ent/PostgreSQL, Vue 3 `<script setup>`, TypeScript, Vitest, Go `testing`/testify.

---

### Task 1: Add target semantics and service-level validation tests

**Files:**
- Modify: `backend/internal/service/lottery.go`
- Test: `backend/internal/service/lottery_attempt_grants_test.go`

**Step 1: Write the failing tests**

Add tests that construct `LotteryAttemptGrantInput` and assert:

- `Target: "active", ActiveDays: 7` is accepted and delegated unchanged.
- `Target: "active", ActiveDays: 30` is accepted and delegated unchanged.
- active target rejects `ActiveDays` values other than 7 or 30.
- active target rejects `All: true` or non-empty `UserIDs`.
- legacy selected/all inputs remain valid after target normalization.

Add a service stub method for an active-user preview capability and test that a valid active preview returns its count while invalid windows return `ErrLotteryAttemptGrantInvalid`.

**Step 2: Run tests to verify they fail**

Run:

```bash
cd backend && go test ./internal/service -run 'TestLotteryAttemptGrant|TestLotteryActiveAttempt' -count=1
```

Expected: FAIL because the target fields/constants and preview method do not exist.

**Step 3: Implement the minimal service contract**

Add constants for the three target values and the two supported active windows. Extend `LotteryAttemptGrantInput` with `Target string` and `ActiveDays int`. Normalize omitted `Target` from legacy fields (`all` → `all`, user IDs → `selected`) before validation. Require exactly one target, require 7/30 for active, and preserve the current amount, idempotency, and max-user checks. Add a `LotteryAdminAttemptPreviewRepository` optional interface and `LotteryService.PreviewLotteryAttemptGrant` that validates and delegates without writing.

**Step 4: Run tests to verify they pass**

Run the same command. Expected: PASS with all existing lottery service tests still green.

**Step 5: Commit**

```bash
git add backend/internal/service/lottery.go backend/internal/service/lottery_attempt_grants_test.go
git commit -m "feat: add lottery grant target semantics"
```

### Task 2: Resolve active users in the repository and cover transaction behavior

**Files:**
- Modify: `backend/internal/repository/lottery_repo.go`
- Test: `backend/internal/repository/lottery_attempt_grants_test.go`

**Step 1: Write the failing tests**

Add Ent integration cases for:

- active user with a usage log inside the selected window is included;
- user whose newest usage log is before the window is excluded;
- user with no usage logs is excluded;
- soft-deleted user with a recent usage log is excluded;
- 7-day and 30-day boundaries use `created_at >= since`;
- preview count matches the IDs that a subsequent active grant would resolve;
- a target set over `LotteryAttemptGrantMaxUsers` fails before any grant, wallet, or ledger rows are created.

Use the existing in-memory/PostgreSQL test client helpers and create usage-log rows directly; no production data is changed.

**Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/repository -run 'TestLotteryRepository.*Active|TestLotteryRepository.*Grant' -count=1
```

Expected: FAIL because active target resolution and preview are not implemented.

**Step 3: Implement the minimal repository behavior**

Add a shared resolver that queries `client.User.Query().Where(user.DeletedAtIsNil(), user.HasUsageLogsWith(usagelog.CreatedAtGTE(since)))`, orders by ID, and returns IDs. Use it for both preview and `resolveLotteryAttemptGrantUsers`. For active grants, derive `since` from the request’s `ActiveDays` and the current clock supplied by the service/handler context; do not persist a snapshot. Keep the existing transaction, idempotency, wallet, ledger, and all-user paths unchanged.

**Step 4: Run tests to verify they pass**

Run the same repository command and then:

```bash
cd backend && go test ./internal/repository -run Lottery -count=1
```

Expected: PASS, including rollback and idempotency coverage.

**Step 5: Commit**

```bash
git add backend/internal/repository/lottery_repo.go backend/internal/repository/lottery_attempt_grants_test.go
git commit -m "feat: resolve active lottery grant users"
```

### Task 3: Expose preview and active grant fields through the admin HTTP API

**Files:**
- Modify: `backend/internal/handler/admin/lottery_handler.go`
- Modify: `backend/internal/server/routes/lottery.go`
- Test: `backend/internal/handler/admin/lottery_handler_test.go`

**Step 1: Write the failing handler tests**

Add tests that:

- `POST /admin/lottery/attempts/preview` accepts `{target:"active",active_days:7}` and returns `{count: ...}`;
- `POST /admin/lottery/attempts/grant` passes `Target` and `ActiveDays` to the service/repository stub;
- an invalid active window returns HTTP 400;
- existing legacy all-user and selected-user JSON requests continue to pass.

**Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/handler/admin -run 'TestLotteryAdminHandler.*Attempt' -count=1
```

Expected: FAIL because the preview route and request fields are missing.

**Step 3: Implement the minimal HTTP layer**

Extend `GrantLotteryAttemptsRequest` with `Target` and `ActiveDays`. Add a preview request/response handler that authenticates through the existing admin middleware, normalizes legacy fields, and invokes `PreviewLotteryAttemptGrant`. Register `POST /admin/lottery/attempts/preview`. Use the existing error envelope and idempotency handling for grants; never create an idempotency record or mutate state during preview.

**Step 4: Run tests to verify they pass**

```bash
cd backend && go test ./internal/handler/admin -run 'TestLotteryAdminHandler.*Attempt' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/admin/lottery_handler.go backend/internal/server/routes/lottery.go backend/internal/handler/admin/lottery_handler_test.go
git commit -m "feat: add lottery active-user preview API"
```

### Task 4: Add frontend API types and contract tests

**Files:**
- Modify: `frontend/src/api/admin/lottery.ts`
- Test: `frontend/src/api/__tests__/lottery.spec.ts`

**Step 1: Write the failing tests**

Add API tests asserting:

- `previewAttemptGrant({ target: 'active', active_days: 7 })` sends `POST /admin/lottery/attempts/preview` and returns the count;
- `grantAttempts` includes `target` and `active_days` when supplied;
- selected/all request tests retain their current payload and idempotency header behavior.

**Step 2: Run tests to verify they fail**

```bash
cd frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts
```

Expected: FAIL because the preview function and request fields are absent.

**Step 3: Implement the minimal API client changes**

Add `LotteryAttemptGrantTarget`, `LotteryAttemptGrantRequest.target`, optional `active_days`, `LotteryAttemptGrantPreviewRequest`, and `LotteryAttemptGrantPreviewResult`. Implement and export `previewAttemptGrant`, and include it in `lotteryAdminAPI`.

**Step 4: Run tests to verify they pass**

Run the same Vitest command. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/lottery.ts frontend/src/api/__tests__/lottery.spec.ts
git commit -m "feat: add lottery grant preview client"
```

### Task 5: Add active-user controls, preview count, and UI tests

**Files:**
- Modify: `frontend/src/views/admin/LotteryView.vue`
- Modify: `frontend/src/i18n/locales/zh/lottery.ts`
- Modify: `frontend/src/i18n/locales/en/lottery.ts`
- Test: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`

**Step 1: Write the failing component tests**

Add tests that:

- selecting the active target reveals the 7/30-day selector;
- changing the window calls the preview API and displays the returned user count;
- preview failure disables submission and shows the existing app error mechanism;
- submitting active target sends `target: 'active'`, `active_days`, amount, description, and request key;
- selected/all flows still submit their original payloads.

**Step 2: Run tests to verify they fail**

```bash
cd frontend && pnpm vitest run src/views/admin/__tests__/LotteryView.spec.ts
```

Expected: FAIL because the controls and preview state do not exist.

**Step 3: Implement the minimal UI**

Add a `grantTarget` value of `active`, an `activeDays` value defaulting to 7, preview loading/error/count state, and a debounced or event-driven preview call when the active target/window changes. Render translated labels for “最近活跃用户”, “最近 7 天”, “最近 30 天”, and the matched-user count. Disable the submit button while preview is loading or failed, and clear active preview state after a successful grant. Keep the existing selected-user search and all-user behavior unchanged.

**Step 4: Run tests to verify they pass**

```bash
cd frontend && pnpm vitest run src/views/admin/__tests__/LotteryView.spec.ts src/i18n/__tests__/lotteryLocales.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/LotteryView.vue frontend/src/i18n/locales/zh/lottery.ts frontend/src/i18n/locales/en/lottery.ts frontend/src/views/admin/__tests__/LotteryView.spec.ts
git commit -m "feat: grant lottery attempts to active users"
```

### Task 6: Run the full verification suite and review the diff

**Files:**
- Test only: existing backend and frontend suites

**Step 1: Run targeted backend tests**

```bash
cd backend && go test ./internal/service ./internal/repository ./internal/handler/admin -count=1
```

Expected: PASS.

**Step 2: Run targeted frontend tests**

```bash
cd frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts src/views/admin/__tests__/LotteryView.spec.ts src/i18n/__tests__/lotteryLocales.spec.ts
```

Expected: PASS.

**Step 3: Run formatting and type/build checks**

```bash
cd backend && gofmt -w internal/service/lottery.go internal/repository/lottery_repo.go internal/handler/admin/lottery_handler.go && go test ./... -count=1
cd ../frontend && pnpm lint && pnpm build
```

Expected: no formatting changes left, all Go tests pass, lint and production build succeed.

**Step 4: Review and commit any final fixes**

```bash
git diff --check
git status --short
```

Commit only the verified implementation changes with a focused message if any final fix was needed.

