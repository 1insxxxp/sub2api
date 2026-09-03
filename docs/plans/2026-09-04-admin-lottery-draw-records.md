# Admin Lottery Draw Records Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Turn the existing basic admin draw history into a discoverable, filterable, paginated, mobile-friendly audit list showing who drew and what they received.

**Architecture:** Extend the existing `/admin/lottery/draws` query contract with a typed filter object passed from handler to service to repository. Keep filtering and pagination in Ent so counts and pages remain accurate, then add compact filters and separate desktop/mobile renderers to the existing admin lottery view.

**Tech Stack:** Go, Gin, Ent, Vue 3, TypeScript, Tailwind CSS, Vitest.

---

### Task 1: Define and validate the backend draw query

**Files:**
- Modify: `backend/internal/service/lottery.go`
- Modify: `backend/internal/handler/admin/lottery_handler.go`
- Test: `backend/internal/handler/admin/lottery_handler_test.go`

1. Add failing handler tests proving pagination and filters are forwarded and invalid enum filters return HTTP 400.
2. Run the targeted handler tests and confirm they fail for the missing query contract.
3. Add `LotteryAdminDrawQuery`, enum validation, and handler parsing for user, prize type, source, and winners-only filters.
4. Run the targeted handler tests and confirm they pass.

### Task 2: Apply filters in the repository

**Files:**
- Modify: `backend/internal/repository/lottery_repo.go`
- Test: `backend/internal/repository/lottery_admin_draws_test.go`

1. Add repository tests with multiple users, prize types, sources, and timestamps.
2. Confirm tests fail because the repository ignores filters.
3. Apply Ent predicates before count/order/pagination and preserve newest-first stable ordering.
4. Confirm repository tests pass.

### Task 3: Extend the frontend API contract

**Files:**
- Modify: `frontend/src/api/admin/lottery.ts`
- Test: `frontend/src/api/__tests__/lottery.spec.ts`

1. Add a failing API test for serialized draw filters.
2. Add the typed query fields and verify the test passes.

### Task 4: Build the responsive records experience

**Files:**
- Modify: `frontend/src/views/admin/LotteryView.vue`
- Modify: `frontend/src/i18n/locales/zh/lottery.ts`
- Modify: `frontend/src/i18n/locales/en/lottery.ts`
- Test: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`

1. Add failing view tests for filter changes, refresh, filtered empty state, and mobile record markup.
2. Add filter state and query loading without page refresh.
3. Add a compact desktop table and mobile stacked record rows with explicit deleted-user and missing-data states.
4. Confirm view tests pass.

### Task 5: Verify and integrate into dev

**Files:**
- Verify all modified files.

1. Run targeted Go and frontend tests.
2. Run backend tests for affected packages, frontend typecheck, and production build.
3. Inspect the final diff and confirm no runtime/generated data is included.
4. Commit the implementation directly on `dev` with a focused feature commit.
