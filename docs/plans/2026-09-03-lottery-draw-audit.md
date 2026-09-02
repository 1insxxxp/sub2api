# Lottery Draw Audit Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expose paginated lottery draw records, including the user and awarded reward, in the admin lottery page.

**Architecture:** Add an admin-only service/repository query over `lottery_draws`, batch-resolve users including soft-deleted accounts, and return a sanitized admin DTO without the attempt key. Add a paginated table and API client to the existing admin lottery view while preserving public draw/history behavior.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Vue 3, TypeScript, Vitest.

---

### Task 1: Add the failing backend contract test

**Files:**
- Modify: `backend/internal/service/lottery_service_test.go`

**Step 1: Write the failing test**

Add a test that maps a `LotteryDraw` plus a resolved user into an admin record and asserts user email, user ID, prize name, product content, source, and timestamp are preserved while the attempt key is absent from the JSON contract.

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/service -run TestLotteryAdminDraw -count=1`

Expected: FAIL because the admin DTO/mapping does not exist.

### Task 2: Implement backend admin draw listing

**Files:**
- Modify: `backend/internal/service/lottery.go`
- Modify: `backend/internal/repository/lottery_repo.go`
- Modify: `backend/internal/handler/admin/lottery_handler.go`
- Modify: `backend/internal/server/routes/lottery.go`

**Step 1: Add service DTO and repository method**

Define an admin draw response with user identity fields and add a repository method that returns draws in descending creation order with total count. Batch-load user rows with soft-delete bypass so deleted users can still be identified.

**Step 2: Add service and handler methods**

Add `ListAdminDraws`, parse standard page/page-size parameters, and respond with `response.Paginated`.

**Step 3: Run backend tests**

Run: `cd backend && go test ./internal/service ./internal/repository ./internal/handler/admin -count=1`

Expected: PASS.

### Task 3: Add the frontend API contract test

**Files:**
- Modify: `frontend/src/api/admin/lottery.ts`
- Modify: `frontend/src/api/__tests__/lottery.spec.ts`

**Step 1: Write the failing test**

Assert `listDraws({ page: 2, page_size: 10 })` requests `/admin/lottery/draws` with unchanged pagination parameters.

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts`

Expected: FAIL because the API method is missing.

**Step 3: Implement the API method and rerun**

Add the admin draw type and method, then rerun the focused test; expected PASS.

### Task 4: Add the admin records table

**Files:**
- Modify: `frontend/src/views/admin/LotteryView.vue`
- Modify: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/lottery.ts`
- Modify: `frontend/src/i18n/locales/en/lottery.ts`

**Step 1: Write the failing view test**

Mock the draw-list API, mount the view, and assert a record row renders the user email and prize name; assert a page change requests the next page.

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/LotteryView.spec.ts`

Expected: FAIL because the records section and loading state do not exist.

**Step 3: Implement the minimal UI**

Load records alongside config, render a responsive table with the requested fields, add pagination and refresh integration, and add Chinese/English labels.

**Step 4: Run focused and full tests**

Run: `cd frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts src/views/admin/__tests__/LotteryView.spec.ts`

Expected: PASS.

### Task 5: Verify the complete change

**Step 1: Run backend tests**

Run: `cd backend && go test ./...`

**Step 2: Run frontend tests and typecheck/build**

Run: `cd frontend && pnpm test:run && pnpm build`

Expected: PASS with no new type or build errors.

**Step 3: Review the diff**

Run: `git diff --check && git status --short`

Expected: only the lottery audit docs and implementation files are changed.
