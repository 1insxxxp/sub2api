# Custom Group Safe Delete Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow users to delete a custom group after an explicit second confirmation by atomically unbinding its API keys and soft-deleting the group.

**Architecture:** Preserve the existing non-forced delete guard and add the bound-key count to structured conflict metadata. A `force=true` request delegates to one repository transaction that verifies ownership, clears live API-key bindings, and soft-deletes the custom group; the frontend retries only after the user confirms the stated impact.

**Tech Stack:** Go, Gin, Ent/PostgreSQL, Vue 3, TypeScript, Axios, Vitest, Testify.

---

### Task 1: Define service behavior and conflict metadata

**Files:**
- Modify: `backend/internal/service/user_custom_group_service_test.go`
- Modify: `backend/internal/service/user_custom_group_service.go`

**Step 1: Write failing tests**

Add a repository stub and tests proving ordinary deletion with two bindings returns `CUSTOM_GROUP_IN_USE` with `metadata.bound_api_key_count == 2`, while forced deletion invokes a new atomic repository method and returns its unbound count.

**Step 2: Verify RED**

Run `cd backend && go test -tags=unit ./internal/service -run 'TestUserCustomGroup(DeleteReturnsBoundKeyCount|ForceDeleteUnbindsAtomically)' -count=1`.

Expected: compile/failure because the service API and atomic repository method do not exist.

**Step 3: Implement the minimal service API**

- Add `DeleteAndUnbindAPIKeys(ctx, id, userID) (int, error)` to `UserCustomGroupRepository`.
- Change service deletion to `Delete(ctx, userID, id, force) (int, error)`.
- Preserve ordinary deletion; return structured count metadata on conflict.
- Forced deletion calls only the atomic repository operation.

**Step 4: Verify GREEN**

Run the command from Step 2. Expected: PASS.

### Task 2: Implement the atomic repository mutation

**Files:**
- Modify: `backend/internal/repository/user_custom_group_repo.go`
- Create: `backend/internal/repository/user_custom_group_repo_integration_test.go`

**Step 1: Write the failing integration test**

Using the existing integration harness, create an owned custom group, live and deleted API keys bound to it, and an unrelated key. Assert forced deletion unbinds the live keys, soft-deletes/disables the group, leaves unrelated/deleted rows untouched, and returns the affected count. Assert a different user gets not-found with no changes.

**Step 2: Verify RED**

Run `cd backend && go test -tags=integration ./internal/repository -run TestUserCustomGroupRepositoryDeleteAndUnbindAPIKeys -count=1`.

Expected: compile/failure because the method does not exist.

**Step 3: Implement the transaction**

- Begin an Ent transaction and lock/select the non-deleted group by ID and owner.
- Clear `custom_group_id` on non-deleted API keys bound to it and capture the count.
- Soft-delete/disable the group in the same transaction.
- Commit and return the count; roll back on errors.

**Step 4: Verify GREEN**

Run the command from Step 2. Expected: PASS.

### Task 3: Expose force deletion through the HTTP API

**Files:**
- Modify: `backend/internal/handler/user_custom_group_handler.go`
- Create: `backend/internal/handler/user_custom_group_handler_test.go`

**Step 1: Write failing handler tests**

Test that normal deletion with bindings returns 409 and count metadata, `?force=true` returns `{deleted:true, unbound_api_key_count:2}`, and an invalid boolean returns 400.

**Step 2: Verify RED**

Run `cd backend && go test -tags=unit ./internal/handler -run TestUserCustomGroupHandlerDelete -count=1`.

Expected: FAIL because the handler ignores `force` and does not return the count.

**Step 3: Implement handler parsing and response**

Parse an optional boolean `force`, pass it to the service, and return both deletion status and unbound count.

**Step 4: Verify GREEN**

Run the command from Step 2. Expected: PASS.

### Task 4: Add the two-step frontend confirmation

**Files:**
- Modify: `frontend/src/api/customGroups.ts`
- Modify: `frontend/src/components/custom-groups/CustomGroupsManager.vue`
- Modify: `frontend/src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts`

**Step 1: Write failing component tests**

Cover the initial delete rejection, a second confirmation containing the bound-key count and original-group outcome, forced retry success, second-confirmation cancellation, and forced retry failure.

**Step 2: Verify RED**

Run `cd frontend && npx vitest run src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts`.

Expected: FAIL because the API and component do not support a force retry.

**Step 3: Implement the UI flow**

- Change `customGroupsAPI.delete(id, force = false)` to send `force=true` only for forced requests and return the deletion result.
- Keep the first confirmation.
- On `CUSTOM_GROUP_IN_USE`, validate the metadata count, show the second explicit confirmation, and retry with force.
- Refresh and emit once after success; display all other errors unchanged.

**Step 4: Verify GREEN**

Run the command from Step 2. Expected: PASS.

### Task 5: Regression verification and commit

**Step 1: Backend verification**

Run `cd backend && go test -tags=unit ./internal/service ./internal/handler -count=1` and the focused repository integration test. Expected: PASS.

**Step 2: Frontend verification**

Run the focused Vitest file, `npm run lint:check`, `npx vue-tsc --noEmit`, and `npm run build` from `frontend`. Expected: PASS.

**Step 3: Diff check and commit**

Run `git diff --check`, inspect staged files, and commit as `fix: allow safe custom group deletion`. Keep root `package.json` and `package-lock.json` untracked and untouched.
