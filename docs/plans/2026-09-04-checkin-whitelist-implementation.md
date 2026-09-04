# Check-in Whitelist Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an admin-managed email/username whitelist that bypasses all check-in qualification thresholds while preserving global enablement, blacklist, and once-per-day protections.

**Architecture:** Extend `service.CheckinConfig` and the existing admin check-in config API with a bounded, normalized string list persisted through the settings repository. Add a pure matching helper and use it in `currentCheckinPolicyState`, which is shared by status and check-in execution. Extend the existing check-in admin view and user status display without introducing a new table or migration.

**Tech Stack:** Go, Gin, Ent-backed service tests, Vue 3, TypeScript, Vitest, existing i18n and API clients.

---

### Task 1: Define whitelist normalization and service behavior tests

**Files:**
- Modify: `backend/internal/service/checkin_service_test.go`
- Modify: `backend/internal/service/checkin_service.go`

**Step 1: Write failing unit tests**

Add table-driven tests for a pure whitelist matcher covering email, username, case-insensitive matching, surrounding whitespace, duplicates/blank entries, and non-matches. Add service status/check-in policy cases proving a match bypasses daily usage and both spend/recharge thresholds, while a blacklisted match remains ineligible.

**Step 2: Run targeted tests**

Run: `go test ./internal/service -run 'TestCheckin.*Whitelist|Test.*Checkin.*Whitelist'`

Expected: FAIL because the whitelist fields/helper do not exist.

**Step 3: Implement minimal service model and matcher**

Add `Whitelist []string` to `CheckinConfig`, `WhitelistExempt bool` to `CheckinStatus` and policy state, plus a normalization/matching helper that trims and folds case. Keep the helper independent of persistence so it is directly testable.

**Step 4: Run tests again**

Run: `go test ./internal/service -run 'TestCheckin.*Whitelist|Test.*Checkin.*Whitelist'`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/checkin_service.go backend/internal/service/checkin_service_test.go
git commit -m "feat: add check-in whitelist policy"
```

### Task 2: Persist whitelist and expose it through the admin API

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/checkin_service.go`
- Modify: `backend/internal/handler/admin/checkin_handler.go`
- Modify: `frontend/src/api/admin/checkins.ts`
- Test: `backend/internal/service/checkin_service_test.go`
- Test: `frontend/src/api/__tests__/admin.checkins.spec.ts`

**Step 1: Extend failing persistence/API tests**

Verify `GetConfig` reads the new setting, `UpdateConfig` writes normalized values, and the admin request/response round-trips the whitelist.

**Step 2: Run targeted tests to verify failure**

Run: `go test ./internal/service ./internal/handler/admin -run 'Checkin.*Config|Checkin.*Whitelist'` and `pnpm vitest run src/api/__tests__/admin.checkins.spec.ts --pool=forks --poolOptions.forks.singleFork=true`

Expected: FAIL on missing setting/field expectations.

**Step 3: Implement persistence and DTO changes**

Add a `SettingKeyCheckinWhitelist` constant, read/write the list as JSON, enforce a bounded number and entry length, and return a bad request for invalid admin payload entries. Preserve an empty list for old installations.

**Step 4: Run targeted tests**

Run the same Go and Vitest commands; expected PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/domain_constants.go backend/internal/service/checkin_service.go backend/internal/handler/admin/checkin_handler.go frontend/src/api/admin/checkins.ts frontend/src/api/__tests__/admin.checkins.spec.ts
git commit -m "feat: persist check-in whitelist settings"
```

### Task 3: Add admin configuration UI and user status messaging

**Files:**
- Modify: `frontend/src/views/admin/CheckinsView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin.ts` and corresponding locale files
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/CheckinsView.spec.ts`

**Step 1: Add failing UI tests**

Cover loading the whitelist into the editor, saving normalized values, showing validation for blank-only/over-limit input as appropriate, and rendering the user-facing exemption state instead of threshold-blocked messaging.

**Step 2: Run targeted frontend tests**

Run: `pnpm vitest run src/views/admin/__tests__/CheckinsView.spec.ts src/components/layout/__tests__/AppHeader.spec.ts --pool=forks --poolOptions.forks.singleFork=true`

Expected: FAIL because the editor, API fields, and status badge are absent.

**Step 3: Implement the UI**

Add a multiline email/username editor in the existing base-rules area, include the list in load/save payloads, and show a concise “白名单免门槛” state when `whitelist_exempt` is true. Keep the controls responsive and use existing buttons, inputs, and i18n conventions.

**Step 4: Run targeted frontend tests**

Run the same Vitest command; expected PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/CheckinsView.vue frontend/src/i18n frontend/src/components/layout/AppHeader.vue frontend/src/components/layout/__tests__/AppHeader.spec.ts frontend/src/views/admin/__tests__/CheckinsView.spec.ts
git commit -m "feat: manage check-in whitelist in admin UI"
```

### Task 4: Run verification and review integration

**Files:**
- Test: relevant backend and frontend suites

**Step 1: Run backend formatting and tests**

Run: `gofmt -w` on changed Go files, then `go test ./internal/service ./internal/handler/admin`.

**Step 2: Run frontend lint and tests**

Run: `pnpm eslint` for changed frontend files and the targeted Vitest suites.

**Step 3: Inspect the final diff**

Run: `git diff --check`, `git status --short`, and review that no unrelated files changed.

**Step 4: Commit any verification-only fixes**

Use a focused commit if needed; do not deploy from a dirty worktree.
