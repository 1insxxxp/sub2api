# Affiliate Leaderboard Avatars Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Display user-configured avatars in the admin invite leaderboard with a reliable initials fallback.

**Architecture:** Extend the existing aggregate query and DTOs with one optional avatar URL, then render it through a small frontend avatar presentation inside both desktop and mobile leaderboard layouts. Keep ranking and affiliate calculations unchanged.

**Tech Stack:** Go, PostgreSQL, Gin, Vue 3, TypeScript, Vitest, Tailwind CSS

---

### Task 1: Lock the backend contract with tests

**Files:**
- Modify: `backend/internal/repository/affiliate_repo_test.go`
- Modify: `backend/internal/handler/admin/affiliate_workbench_handler_test.go`

**Step 1: Write the failing tests**

Assert that the summary SQL joins `user_avatars`, projects its URL, and that the workbench response contains `inviter_avatar_url`.

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/repository ./internal/handler/admin`

Expected: FAIL because the query and response do not include the avatar field.

### Task 2: Implement the backend avatar projection

**Files:**
- Modify: `backend/internal/service/affiliate_service.go`
- Modify: `backend/internal/repository/affiliate_repo.go`
- Modify: `backend/internal/handler/admin/affiliate_handler.go`

**Step 1: Extend the data models**

Add `InviterAvatarURL` with JSON name `inviter_avatar_url` to the service summary and workbench response item.

**Step 2: Extend the aggregate query**

Left join `user_avatars`, select the URL, add it to the grouped projection, and scan it into the service summary.

**Step 3: Run backend tests**

Run: `cd backend && go test ./internal/repository ./internal/handler/admin`

Expected: PASS.

### Task 3: Lock the frontend behavior with tests

**Files:**
- Modify: `frontend/src/components/admin/workbench/__tests__/AdminAffiliateLeaderboardPanel.spec.ts`

**Step 1: Write the failing test**

Provide one user with an avatar URL and one without it. Assert that the image and initials fallback are rendered in desktop and mobile layouts, and that an image error switches to the fallback.

**Step 2: Run the test to verify it fails**

Run: `cd frontend && pnpm vitest run src/components/admin/workbench/__tests__/AdminAffiliateLeaderboardPanel.spec.ts`

Expected: FAIL because the component does not render avatars.

### Task 4: Implement the responsive avatar UI

**Files:**
- Modify: `frontend/src/api/admin/affiliates.ts`
- Modify: `frontend/src/components/admin/workbench/AdminAffiliateLeaderboardPanel.vue`

**Step 1: Extend the API type**

Add `inviter_avatar_url` to `WorkbenchAffiliateLeaderboardItem`.

**Step 2: Render avatars and fallbacks**

Add fixed-size circular avatar elements to desktop and mobile identity rows. Track failed image IDs and render initials after an error.

**Step 3: Run the component test**

Run: `cd frontend && pnpm vitest run src/components/admin/workbench/__tests__/AdminAffiliateLeaderboardPanel.spec.ts`

Expected: PASS.

### Task 5: Verify the complete change

**Files:**
- Verify all modified files

**Step 1: Run focused backend tests**

Run: `cd backend && go test ./internal/repository ./internal/handler/admin`

Expected: PASS.

**Step 2: Run focused frontend tests**

Run: `cd frontend && pnpm vitest run src/api/__tests__/admin.affiliates.workbench.spec.ts src/components/admin/workbench/__tests__/AdminAffiliateLeaderboardPanel.spec.ts`

Expected: PASS.

**Step 3: Run static and build checks**

Run: `cd frontend && pnpm typecheck && pnpm build`

Expected: PASS.

**Step 4: Check the patch**

Run: `git diff --check && git status --short`

Expected: no whitespace errors and only the intended files are modified.
