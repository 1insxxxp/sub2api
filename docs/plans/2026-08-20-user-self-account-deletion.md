# User Self Account Deletion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let a signed-in user permanently cancel their own account through a password-confirmed self-service flow.

**Architecture:** Add a user-domain deletion service that verifies the current password, rejects admin self-deletion, deletes owned API keys, soft-deletes the user, and invalidates auth caches. Expose it through `DELETE /api/v1/user/account`, revoke refresh sessions in the handler, and add a restrained danger-zone UI on the profile page.

**Tech Stack:** Go/Gin service and handler tests, Vue 3/Pinia/Vitest frontend tests, existing i18n and profile card patterns.

---

### Task 1: Backend Service Contract

**Files:**
- Modify: `backend/internal/service/user_service.go`
- Modify: `backend/internal/service/user_service_test.go`
- Test: `backend/internal/service/user_service_test.go`

**Step 1: Write failing service tests**

Add tests for:
- wrong password returns `ErrPasswordIncorrect` and does not delete,
- admin user gets a forbidden error and does not delete,
- normal user deletion lists and deletes owned API keys, deletes the user, and invalidates cache by key and user.

**Step 2: Run test to verify it fails**

Run: `go test -tags=unit ./internal/service -run 'TestDeleteOwnAccount' -count=1`

Expected: FAIL because `DeleteOwnAccount` and API-key wiring do not exist.

**Step 3: Implement minimal service code**

Add:
- `ErrAccountDeletionForbidden`,
- a narrow optional API-key deletion dependency or a setter on `UserService`,
- `DeleteOwnAccount(ctx, userID, password string) error`,
- a paged helper to list API keys before deletion.

**Step 4: Run test to verify it passes**

Run: `go test -tags=unit ./internal/service -run 'TestDeleteOwnAccount' -count=1`

Expected: PASS.

### Task 2: Backend Handler And Route

**Files:**
- Modify: `backend/internal/handler/user_handler.go`
- Modify: `backend/internal/handler/user_handler_test.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/cmd/server/wire_gen.go`
- Test: `backend/internal/handler/user_handler_test.go`

**Step 1: Write failing handler tests**

Add tests for:
- missing/empty password returns 400,
- successful delete calls service and revokes refresh sessions through `AuthService`.

**Step 2: Run test to verify it fails**

Run: `go test -tags=unit ./internal/handler -run 'TestUserHandlerDeleteOwnAccount' -count=1`

Expected: FAIL because the handler method does not exist.

**Step 3: Implement minimal handler and route code**

Add:
- `DeleteOwnAccountRequest`,
- `UserHandler.DeleteOwnAccount`,
- `user.DELETE("/account", h.User.DeleteOwnAccount)`,
- production wiring to attach the API-key repository to `UserService`.

**Step 4: Run route/handler checks**

Run:
- `go test -tags=unit ./internal/handler -run 'TestUserHandlerDeleteOwnAccount' -count=1`
- `go test ./internal/server/routes -run 'UserRoutes' -count=1`

Expected: PASS.

### Task 3: Frontend API

**Files:**
- Modify: `frontend/src/api/user.ts`
- Create: `frontend/src/api/__tests__/user.accountDeletion.spec.ts`

**Step 1: Write failing API test**

Assert `deleteOwnAccount('secret')` calls `apiClient.delete('/user/account', { data: { password: 'secret' } })`.

**Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/api/__tests__/user.accountDeletion.spec.ts`

Expected: FAIL because `deleteOwnAccount` does not exist.

**Step 3: Implement API helper**

Export `deleteOwnAccount(password)` and add it to `userAPI`.

**Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run src/api/__tests__/user.accountDeletion.spec.ts`

Expected: PASS.

### Task 4: Frontend Danger-Zone UI

**Files:**
- Create: `frontend/src/components/user/profile/ProfileDangerZoneCard.vue`
- Create: `frontend/src/components/user/profile/__tests__/ProfileDangerZoneCard.spec.ts`
- Modify: `frontend/src/views/user/ProfileView.vue`
- Modify: `frontend/src/views/user/__tests__/ProfileView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`

**Step 1: Write failing component test**

Assert the component:
- does not submit without a password,
- calls `userAPI.deleteOwnAccount(password)`,
- calls `authStore.logout()`,
- redirects to `/login`,
- shows backend error messages on failure.

**Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/components/user/profile/__tests__/ProfileDangerZoneCard.spec.ts`

Expected: FAIL because the component does not exist.

**Step 3: Implement UI**

Create a compact danger card with an explicit confirmation panel, current-password input, cancel button, destructive submit button, loading state, and red accent styling. Add it below passkeys in `ProfileView.vue`.

**Step 4: Run view/component tests**

Run:
- `pnpm exec vitest run src/components/user/profile/__tests__/ProfileDangerZoneCard.spec.ts src/views/user/__tests__/ProfileView.spec.ts`

Expected: PASS.

### Task 5: Full Verification And Commit

**Files:**
- All changed files.

**Step 1: Backend verification**

Run:
- `go test -tags=unit ./internal/service -run 'TestDeleteOwnAccount' -count=1`
- `go test -tags=unit ./internal/handler ./internal/server -run 'DeleteOwnAccount|TestAPIContracts/DELETE_/api/v1/user/account' -count=1`
- `go build ./internal/service ./internal/handler ./internal/server/routes ./cmd/server`

**Step 2: Frontend verification**

Run:
- `pnpm exec vitest run src/api/__tests__/user.accountDeletion.spec.ts src/components/user/profile/__tests__/ProfileDangerZoneCard.spec.ts src/views/user/__tests__/ProfileView.spec.ts`
- `pnpm run typecheck`
- `pnpm run typecheck:tests`

**Step 3: Review diff and commit**

Run:
- `git diff --check`
- `git status --short`
- `git add -f docs/plans/2026-08-20-user-self-account-deletion.md`
- `git add backend frontend`
- `git commit -m "feat: add user self account deletion"`
