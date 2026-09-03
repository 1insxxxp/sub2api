# Lottery Attempt Balances Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an administrator view that shows each non-deleted user’s current activity attempts, granted wallet attempts, and total available lottery attempts.

**Architecture:** Add an optional lottery repository capability that pages users and batch-loads wallet balances and activity draw counts in the same backend request. Expose it through an authenticated admin endpoint, then add a searchable/paginated table to the existing admin lottery page and refresh it after grants.

**Tech Stack:** Go, Gin, Ent, SQLite/PostgreSQL, Vue 3, TypeScript, Vitest, vue-i18n.

---

### Task 1: Add failing backend service and handler tests

**Files:**
- Modify: `backend/internal/service/lottery_attempt_grants_test.go`
- Modify: `backend/internal/handler/admin/lottery_handler_test.go`

**Step 1: Write the failing tests**

- Add a service stub for the optional attempt-balance repository capability.
- Assert the service normalizes page/page size/search, passes the active activity and daily cutoff to the repository, and returns the paginated balance rows.
- Add an HTTP handler test for `GET /admin/lottery/attempts` asserting page/search parsing and the standard paginated response.

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/service ./internal/handler/admin -run 'Lottery.*Attempt'`

Expected: FAIL because the new service method, repository capability, and handler are not defined.

### Task 2: Implement backend attempt-balance query

**Files:**
- Modify: `backend/internal/service/lottery.go`
- Modify: `backend/internal/repository/lottery_repo.go`
- Modify: `backend/internal/handler/admin/lottery_handler.go`
- Modify: `backend/internal/server/routes/lottery.go`
- Modify: `backend/internal/repository/lottery_attempt_grants_test.go`

**Step 1: Implement service types and capability**

- Define `LotteryAdminAttemptBalance`, `LotteryAdminAttemptBalanceQuery`, and the optional `LotteryAdminAttemptBalanceRepository` interface.
- Add `ListAdminAttemptBalances` on `LotteryService` with bounded pagination, trimmed search, active-activity lookup, and the correct daily/total draw cutoff.

**Step 2: Implement the production repository**

- Page active and disabled users using email/username search, preserving the default soft-delete exclusion.
- Batch query wallet balances by user IDs.
- Batch aggregate activity draw counts by user ID (with the daily cutoff when configured), then calculate activity/reward/total remaining using the same helper as the user draw path.
- Add a repository integration test covering search, pagination, deleted-user exclusion, wallet balance, and daily activity usage.

**Step 3: Add the admin endpoint and route**

- Parse pagination and `search` in `ListAttemptBalances` and return `response.Paginated`.
- Register `GET /admin/lottery/attempts` beside the existing grant endpoint.

**Step 4: Run backend tests**

Run: `cd backend && go test ./internal/service ./internal/repository ./internal/handler/admin`

Expected: PASS.

### Task 3: Add frontend API, translations, and failing UI tests

**Files:**
- Modify: `frontend/src/api/admin/lottery.ts`
- Modify: `frontend/src/views/admin/LotteryView.vue`
- Modify: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/lottery.ts`
- Modify: `frontend/src/i18n/locales/en/lottery.ts`

**Step 1: Write the failing UI/API tests**

- Assert `listAttemptBalances` calls `/admin/lottery/attempts` with page, page size, and search.
- Assert the admin page loads the attempt rows, renders the three balance columns, paginates, and refreshes after a successful grant.

**Step 2: Run tests to verify they fail**

Run: `cd frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts src/views/admin/__tests__/LotteryView.spec.ts`

Expected: FAIL because the API method, state, table, and messages are not present.

### Task 4: Implement the frontend attempt list

**Files:**
- Modify: `frontend/src/api/admin/lottery.ts`
- Modify: `frontend/src/views/admin/LotteryView.vue`
- Modify: `frontend/src/i18n/locales/zh/lottery.ts`
- Modify: `frontend/src/i18n/locales/en/lottery.ts`

**Step 1: Add the API contract**

- Add the balance row/query types and `listAttemptBalances` function to `lotteryAdminAPI`.

**Step 2: Add the admin table**

- Add search, loading/error/empty states, user identity/status, activity remaining, wallet remaining, and total remaining columns.
- Add pagination handlers and include the list in initial load and refresh.
- Reload the first page after a successful grant so the visible counts immediately reflect the change.

**Step 3: Add localized messages**

- Add labels and state messages in Chinese and English.

**Step 4: Run frontend tests and checks**

Run: `cd frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts src/views/admin/__tests__/LotteryView.spec.ts && pnpm run typecheck && pnpm run lint && pnpm run build`

Expected: PASS with no type, lint, or build errors.

### Task 5: Review and commit

**Files:**
- Review all changed backend/frontend files and `docs/plans/2026-09-03-lottery-attempt-balances.md`.

**Step 1: Verify the full focused suite**

Run: `cd backend && go test ./internal/service ./internal/repository ./internal/handler/admin`; then run the frontend checks from Task 4.

**Step 2: Inspect the diff**

Run: `git diff --check && git status --short`.

**Step 3: Commit**

```bash
git add -f docs/plans/2026-09-03-lottery-attempt-balances.md
git add backend frontend
git commit -m "feat: show lottery attempts by user in admin"
```
