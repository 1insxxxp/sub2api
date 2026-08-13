# Custom Group Stale Source Repair Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent new custom-group source breakage and make existing stale source mappings visible and repairable without changing billing or automatically remapping user models.

**Architecture:** Add a repository-backed reference guard before admin group deletion, annotate custom-group model DTOs with current source availability, and render stale mappings explicitly in the existing custom-group manager. Gateway resolution remains fail-closed and existing stale rows remain untouched until the owner removes or replaces them.

**Tech Stack:** Go, Ent/PostgreSQL, Gin application errors, Vue 3, TypeScript, Vitest, Vue Test Utils.

---

### Task 1: Protect referenced source groups from deletion

**Files:**
- Modify: `backend/internal/service/group_service.go`
- Modify: `backend/internal/service/admin_group.go`
- Modify: `backend/internal/repository/group_repo.go`
- Test: `backend/internal/service/admin_service_group_test.go`

**Step 1: Write the failing service tests**

Add repository-stub controls and tests proving that `DeleteGroup`:

- returns HTTP 409 reason `CUSTOM_GROUP_SOURCE_IN_USE` with a safe `reference_count` metadata value when active custom groups reference the source;
- does not call `DeleteCascade` in that case;
- still deletes normally when the reference count is zero;
- propagates count-query errors without deleting.

**Step 2: Run the focused tests and confirm RED**

Run: `cd backend && go test -tags=unit ./internal/service -run 'TestAdminService_DeleteGroup.*CustomGroup' -count=1`

Expected: FAIL because the repository contract and guard do not exist.

**Step 3: Implement the minimal guard**

- Add `CountCustomGroupModelReferences(ctx, sourceGroupID)` to the admin repository contract.
- Count only mappings owned by non-deleted custom groups.
- Add a stable 409 application error with `reference_count` metadata.
- Execute the guard before collecting API keys or calling `DeleteCascade`.

**Step 4: Run focused tests and confirm GREEN**

Run the focused command from Step 2.

### Task 2: Annotate stale source mappings in custom-group responses

**Files:**
- Modify: `backend/internal/service/user_custom_group.go`
- Modify: `backend/internal/service/user_custom_group_service.go`
- Test: `backend/internal/service/user_custom_group_service_test.go`

**Step 1: Write failing response-annotation tests**

Cover list/get results for:

- deleted or missing source group => `source_available=false`, `source_issue=source_group_unavailable`;
- inactive/composite source => unavailable;
- source no longer bindable by the user => `source_group_not_allowed`;
- valid source => available with no issue.

**Step 2: Run focused tests and confirm RED**

Run: `cd backend && go test -tags=unit ./internal/service -run 'TestUserCustomGroupService.*SourceAvailability' -count=1`

Expected: FAIL because availability fields and annotation are absent.

**Step 3: Implement minimal annotation**

- Add backward-compatible response-only fields `source_available` and `source_issue`.
- Annotate list/get/create/update responses with the current user and loaded source group.
- Preserve stored mappings and aliases; do not auto-delete or auto-remap.

**Step 4: Run focused tests and confirm GREEN**

Run the focused command from Step 2.

### Task 3: Make stale mappings visible and repairable in the UI

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/components/custom-groups/CustomGroupsManager.vue`
- Test: `frontend/src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts`
- Modify: locale files only if new user-facing text requires them.

**Step 1: Write failing UI tests**

Cover:

- stale mappings are shown in red in the list and edit dialog;
- the edit dialog explains that the source is unavailable and offers removal;
- save is blocked while an unavailable selected mapping remains;
- removing the stale mapping enables save/reselection;
- old backend responses with no `source_available` field remain valid.

**Step 2: Run focused tests and confirm RED**

Run: `cd frontend && npx vitest run src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts`

Expected: FAIL because stale mappings are currently invisible.

**Step 3: Implement the minimal UI**

- Extend the TypeScript contract with optional availability fields.
- Add a compact red warning block above candidates in edit mode.
- Keep stale selected items removable but never selectable as new routes.
- Disable/guard save until stale selected mappings are removed.
- Treat omitted fields from an old backend as available for blue-green compatibility.

**Step 4: Run focused tests and confirm GREEN**

Run the focused command from Step 2.

### Task 4: Regression verification

**Files:**
- Verify only; no production data mutation.

**Step 1: Backend tests**

Run:

`cd backend && go test -tags=unit ./internal/service ./internal/repository ./internal/handler/admin -count=1`

**Step 2: Frontend tests and static checks**

Run:

- `cd frontend && npx vitest run src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts`
- `cd frontend && npm run lint:check`
- `cd frontend && npx vue-tsc --noEmit`

**Step 3: Diff validation**

Run: `git diff --check`

Confirm that no migration, billing calculation, gateway fallback, or existing mapping rows changed.
