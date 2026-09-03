# Lottery Attempt Grants Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an admin workflow for granting lottery attempts to selected users or all non-deleted users with transactional wallet updates and audit records.

**Architecture:** Keep activity attempts and wallet reward attempts separate. Add a per-user `lottery_attempt_grants` record whose generated ID is used as the unique `admin_grant` ledger source, then expose a service capability, admin endpoint, and Vue controls on the existing lottery page. The all-user path resolves non-deleted user IDs inside the same transaction and rolls back the entire batch on failure.

**Tech Stack:** Go, Gin, Ent/PostgreSQL migrations, Vue 3 `<script setup>`, TypeScript, Vitest, existing admin API and i18n infrastructure.

---

### Task 1: Persist administrator grant operations

**Files:**
- Create: `backend/ent/schema/lottery_attempt_grant.go`
- Create: `backend/migrations/239_lottery_attempt_grants.sql`
- Test: `backend/migrations/lottery_attempt_grants_migration_test.go`

**Step 1: Write the failing migration contract test**

Assert the migration creates `lottery_attempt_grants`, enforces positive amounts, links the target and creator users, and expands the ledger source check to include `admin_grant`.

**Step 2: Run the migration test to verify it fails**

Run: `cd backend && go test ./migrations -run TestLotteryAttemptGrantsMigrationContract -count=1`

Expected: FAIL because migration 239 and its contract do not exist.

**Step 3: Add the Ent schema and idempotent SQL migration**

Define the grant row fields (`user_id`, `amount`, `description`, `created_by`, `created_at`) and add the table/index plus the updated ledger check constraint in migration 239. Keep the existing unique `(source_type, source_id)` ledger index unchanged because every grant row has its own source ID.

**Step 4: Generate Ent code**

Run: `cd backend && go generate ./ent`

Expected: generated `lotteryattemptgrant*` files appear and compile.

**Step 5: Run the migration test to verify it passes**

Run: `cd backend && go test ./migrations -run TestLotteryAttemptGrantsMigrationContract -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/ent/schema/lottery_attempt_grant.go backend/ent/lotteryattemptgrant backend/migrations/239_lottery_attempt_grants.sql backend/migrations/lottery_attempt_grants_migration_test.go
git commit -m "feat: persist lottery attempt grants"
```

### Task 2: Add service validation and repository capability

**Files:**
- Modify: `backend/internal/service/lottery.go`
- Modify: `backend/internal/service/lottery_attempt_wallet.go`
- Modify: `backend/internal/repository/lottery_repo.go`
- Test: `backend/internal/service/lottery_attempt_grants_test.go`
- Test: `backend/internal/repository/lottery_attempt_grants_test.go`

**Step 1: Write failing service tests**

Cover: positive selected-user request delegates to the optional repository and returns affected/total values; all-user request is accepted; missing target, mixed target modes, non-positive amount, excessive amount, invalid IDs, and missing creator are rejected with `ErrLotteryConfigurationDenied` (or the dedicated bad-request error chosen in the implementation).

**Step 2: Run service tests to verify they fail**

Run: `cd backend && go test ./internal/service -run TestLotteryAttemptGrant -count=1`

Expected: FAIL because the input/result types and service method do not exist.

**Step 3: Implement minimal service API and validation**

Add `LotteryAttemptGrantInput`, `LotteryAttemptGrantResult`, the `LotteryAdminAttemptRepository` optional capability, the `GrantLotteryAttempts` method, source constant `LotteryAttemptLedgerSourceAdminGrant`, and bounded note/amount validation. Normalize and deduplicate explicit IDs before delegation.

**Step 4: Implement the production repository transaction**

Resolve explicit IDs or all non-deleted users, reject missing explicit targets, then for each target insert a grant row, ensure/create the wallet, increment the balance, and insert an `admin_grant` ledger entry pointing at the grant row ID. Use one transaction and return affected users plus total attempts.

**Step 5: Run service and repository tests to verify they pass**

Run: `cd backend && go test ./internal/service ./internal/repository -run 'TestLotteryAttemptGrant|TestLotteryAdminGrant' -count=1`

Expected: PASS (repository integration tests may skip when PostgreSQL is unavailable).

**Step 6: Commit**

```bash
git add backend/internal/service/lottery.go backend/internal/service/lottery_attempt_wallet.go backend/internal/repository/lottery_repo.go backend/internal/service/lottery_attempt_grants_test.go backend/internal/repository/lottery_attempt_grants_test.go
git commit -m "feat: grant lottery attempts transactionally"
```

### Task 3: Expose the admin endpoint

**Files:**
- Modify: `backend/internal/handler/admin/lottery_handler.go`
- Modify: `backend/internal/server/routes/lottery.go`
- Test: `backend/internal/handler/admin/lottery_handler_test.go`

**Step 1: Write failing handler tests**

Set an authenticated `AuthSubject`, POST a selected-user grant payload, assert the service capability receives the creator ID and the response contains `affected` and `total_granted`; add a bad-payload test for no target/amount.

**Step 2: Run the handler tests to verify they fail**

Run: `cd backend && go test ./internal/handler/admin -run TestLotteryAdminHandlerGrantsAttempts -count=1`

Expected: FAIL because the handler method and route do not exist.

**Step 3: Implement handler and route**

Bind the JSON request, require the existing auth subject, call `GrantLotteryAttempts`, map service errors through `response.ErrorFrom`, and register `POST /admin/lottery/attempts/grant` under the existing admin lottery group.

**Step 4: Run handler tests to verify they pass**

Run: `cd backend && go test ./internal/handler/admin -run 'TestLotteryAdminHandlerGrantsAttempts|TestLotteryAdminHandlerRejectsInvalidGrant' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/admin/lottery_handler.go backend/internal/server/routes/lottery.go backend/internal/handler/admin/lottery_handler_test.go
git commit -m "feat: expose lottery attempt grant endpoint"
```

### Task 4: Add frontend grant controls and API contract

**Files:**
- Modify: `frontend/src/api/admin/lottery.ts`
- Modify: `frontend/src/views/admin/LotteryView.vue`
- Modify: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/api/__tests__/lottery.spec.ts`
- Modify: `frontend/src/locales/zh-CN.ts`
- Modify: `frontend/src/locales/en-US.ts`

**Step 1: Write failing API and view tests**

Assert `grantAttempts` posts the selected/all payload to `/admin/lottery/attempts/grant`. Mount the view with mocked users and grant calls, select a user from search results, submit the amount, then verify the payload and success text; also cover the all-user target.

**Step 2: Run frontend tests to verify they fail**

Run: `cd frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts src/views/admin/__tests__/LotteryView.spec.ts`

Expected: FAIL because the API method and controls do not exist.

**Step 3: Implement the API contract and UI**

Add request/result types and `grantAttempts`. In `LotteryView.vue`, add target radio/select state, debounced `adminAPI.users.list` search, selected-user chips, amount/note fields, loading/error handling, and a submit action that resets the form and reports affected users. Keep all-user mode explicit and prevent submission with no selected target or invalid amount.

**Step 4: Add localized copy**

Add Chinese and English labels for the grant section, target choices, search, amount, note, validation, progress, success, and failure states.

**Step 5: Run frontend tests to verify they pass**

Run: `cd frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts src/views/admin/__tests__/LotteryView.spec.ts`

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/api/admin/lottery.ts frontend/src/views/admin/LotteryView.vue frontend/src/views/admin/__tests__/LotteryView.spec.ts frontend/src/api/__tests__/lottery.spec.ts frontend/src/locales/zh-CN.ts frontend/src/locales/en-US.ts
git commit -m "feat: add lottery attempt grant controls"
```

### Task 5: Verify the integrated feature

**Files:**
- Modify only if verification finds defects.

**Step 1: Run focused backend tests**

Run: `cd backend && go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/server/routes -run 'LotteryAttemptGrant|LotteryAdminGrant|LotteryAdminHandler' -count=1`

**Step 2: Run frontend quality checks**

Run: `cd frontend && pnpm typecheck && pnpm exec eslint src/api/admin/lottery.ts src/views/admin/LotteryView.vue && pnpm build`

**Step 3: Check patch hygiene**

Run: `git diff --check`

Expected: all commands pass; unrelated pre-existing failures, if any, are reported separately rather than masked.

**Step 4: Commit any verification fixes**

```bash
git add <verified-fix-files>
git commit -m "fix: polish lottery attempt grants"
```
