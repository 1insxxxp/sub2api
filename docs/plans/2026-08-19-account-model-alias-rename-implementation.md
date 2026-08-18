# Account Model Alias Rename Cascade Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When an admin renames the left-side public model name in an Antigravity account mapping, automatically migrate downstream channel pricing and custom group route references.

**Architecture:** Add a backend cascade service method behind an admin account endpoint. The frontend detects left-side renames after a successful account update and calls that endpoint before closing the edit dialog. The cascade is scoped to the updated account's bound group IDs and preserves old pricing/configuration unless an exact non-destructive copy is safe.

**Tech Stack:** Go, Gin, Ent/sql, PostgreSQL, Vue 3, TypeScript, Vitest, Go tests.

---

### Task 1: Create Backend Cascade Types And Unit Tests

**Files:**
- Modify: `backend/internal/service/admin_service.go`
- Create: `backend/internal/service/account_model_alias_rename.go`
- Test: `backend/internal/service/account_model_alias_rename_test.go`

**Step 1: Write the failing tests**

Add tests for a pure planner function that compares old/new model names and produces safe update operations:

```go
func TestNormalizeAccountModelAliasRenames(t *testing.T) {
	input := []AccountModelAliasRenameInput{
		{OldModel: " old ", NewModel: " new "},
		{OldModel: "OLD", NewModel: "new"},
		{OldModel: "same", NewModel: "same"},
	}

	got := normalizeAccountModelAliasRenames(input)

	require.Equal(t, []AccountModelAliasRename{{OldModel: "old", NewModel: "new"}}, got)
}
```

Add service-level tests with stubs for:

- Loading the account and its `GroupIDs`.
- Updating channel pricing/mapping for the affected groups.
- Updating user custom group routes.
- Updating system custom group routes.
- Returning skipped items when a repo reports conflict.

**Step 2: Run tests to verify failure**

Run: `cd backend && go test ./internal/service -run 'TestNormalizeAccountModelAliasRenames|TestAdminService_CascadeAccountModelAliasRenames'`

Expected: fail because types/function/method do not exist.

**Step 3: Implement minimal service code**

Add:

```go
type AccountModelAliasRenameInput struct {
	OldModel string `json:"old_model"`
	NewModel string `json:"new_model"`
}

type AccountModelAliasRename struct {
	OldModel string `json:"old_model"`
	NewModel string `json:"new_model"`
}

type AccountModelAliasRenameCascadeResult struct {
	ChannelPricingUpdated      int                               `json:"channel_pricing_updated"`
	ChannelMappingsUpdated     int                               `json:"channel_mappings_updated"`
	UserCustomRoutesUpdated    int                               `json:"user_custom_routes_updated"`
	SystemCustomRoutesUpdated  int                               `json:"system_custom_routes_updated"`
	Skipped                    []AccountModelAliasRenameSkipItem `json:"skipped"`
}

type AccountModelAliasRenameSkipItem struct {
	Scope    string `json:"scope"`
	OwnerID  int64  `json:"owner_id,omitempty"`
	OldModel string `json:"old_model"`
	NewModel string `json:"new_model"`
	Reason   string `json:"reason"`
}
```

Add `CascadeAccountModelAliasRenames(ctx, accountID, input)` to `AdminService` and `adminServiceImpl`.

Implementation rules:

- Load account by ID and reject if `Platform != antigravity`.
- Normalize duplicate/no-op renames.
- Use account `GroupIDs` as the affected source groups.
- If no groups or no renames, return a zero result.
- Delegate actual persistence to narrow repository interfaces added in later tasks.

**Step 4: Run tests to verify pass**

Run the same `go test` command.

Expected: pass.

**Step 5: Commit**

```bash
git add backend/internal/service/admin_service.go backend/internal/service/account_model_alias_rename.go backend/internal/service/account_model_alias_rename_test.go
git commit -m "feat: add account model alias cascade service"
```

### Task 2: Implement Repository Cascade Writes

**Files:**
- Modify: `backend/internal/service/group_service.go`
- Modify: `backend/internal/repository/group_repo.go`
- Modify: `backend/internal/service/channel_service.go`
- Modify: `backend/internal/repository/channel_repo.go`
- Test: `backend/internal/repository/group_repo_custom_group_reference_test.go`
- Test: `backend/internal/repository/channel_repo_test.go`

**Step 1: Write failing repository tests**

For group custom routes, test SQL updates:

- `user_custom_group_models.source_model` changes from old to new for active custom groups.
- `user_custom_group_models.public_model` changes only when it equals old and does not collide.
- `system_custom_group_models.source_model` changes from old to new.
- `system_custom_group_models.public_model` changes only when it equals old and does not collide.

For channels, test:

- A pricing row containing old model is extended with new model when new is absent.
- Existing new pricing is not overwritten.
- `channels.model_mapping` copies `old -> target` to `new -> target` when new key is absent.

**Step 2: Run tests to verify failure**

Run:

```bash
cd backend
go test ./internal/repository -run 'Test.*ModelAliasRename'
```

Expected: fail because repository methods do not exist.

**Step 3: Add repository interfaces**

Add narrow interfaces:

```go
type AccountModelAliasReferenceRepository interface {
	RenameAccountModelAliasReferences(ctx context.Context, groupIDs []int64, renames []AccountModelAliasRename) (AccountModelAliasRenameCascadeResult, error)
}
```

and, for channels:

```go
type ChannelModelAliasCascadeRepository interface {
	CopyModelAliasPricingAndMapping(ctx context.Context, groupIDs []int64, platform string, renames []AccountModelAliasRename) (AccountModelAliasRenameCascadeResult, error)
}
```

**Step 4: Implement SQL carefully**

Use transactions and group-reference advisory locks for affected group IDs.

For custom routes, prefer SQL that avoids unique collisions:

```sql
UPDATE user_custom_group_models AS model
SET source_model = $new, updated_at = NOW()
FROM user_custom_groups AS custom_group
WHERE custom_group.id = model.custom_group_id
  AND custom_group.deleted_at IS NULL
  AND model.source_group_id = ANY($group_ids)
  AND LOWER(model.source_model) = LOWER($old)
  AND NOT EXISTS (
    SELECT 1 FROM user_custom_group_models AS peer
    WHERE peer.custom_group_id = model.custom_group_id
      AND peer.id <> model.id
      AND LOWER(peer.source_model) = LOWER($new)
  );
```

Use similar guarded updates for `public_model` and `system_custom_group_models`.

For channel pricing/mapping, load affected channels, mutate in Go using existing JSON helpers, and update rows through existing repository update path.

**Step 5: Wire service dependencies**

Update `adminServiceImpl` to call the optional repositories if available, merge counts, invalidate channel cache, and invalidate auth cache for affected groups.

**Step 6: Run tests**

Run:

```bash
cd backend
go test ./internal/service ./internal/repository -run 'Test.*ModelAliasRename|TestAdminService_CascadeAccountModelAliasRenames'
```

Expected: pass.

**Step 7: Commit**

```bash
git add backend/internal/service backend/internal/repository
git commit -m "feat: migrate downstream model alias references"
```

### Task 3: Add Admin HTTP Endpoint

**Files:**
- Modify: `backend/internal/handler/admin/account_handler.go`
- Modify: `backend/internal/server/routes/admin.go`
- Test: `backend/internal/handler/admin/account_handler_test.go`

**Step 1: Write failing handler tests**

Add tests for:

- `POST /admin/accounts/:id/model-alias-renames` accepts a valid payload and returns counts.
- Empty/no-op payload returns zero counts.
- Non-Antigravity accounts return a bad request or no-op result, matching the service error.

**Step 2: Run test to verify failure**

Run:

```bash
cd backend
go test ./internal/handler/admin -run 'TestAccountHandler.*ModelAliasRenames'
```

Expected: fail because handler/route do not exist.

**Step 3: Implement handler and route**

Add request struct:

```go
type AccountModelAliasRenameRequest struct {
	Renames []service.AccountModelAliasRenameInput `json:"renames"`
}
```

Add `AccountHandler.CascadeModelAliasRenames`, parse account ID, bind JSON, call service, and return `response.Success`.

Register:

```go
accounts.POST("/:id/model-alias-renames", h.Admin.Account.CascadeModelAliasRenames)
```

**Step 4: Run tests**

Run the same handler test command.

Expected: pass.

**Step 5: Commit**

```bash
git add backend/internal/handler/admin/account_handler.go backend/internal/server/routes/admin.go backend/internal/handler/admin/account_handler_test.go
git commit -m "feat: expose account model alias cascade endpoint"
```

### Task 4: Add Frontend API And Rename Detection

**Files:**
- Modify: `frontend/src/api/admin/accounts.ts`
- Modify: `frontend/src/components/account/EditAccountModal.vue`
- Test: `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`

**Step 1: Write failing frontend tests**

Add tests that:

- Open an Antigravity account with `model_mapping: { old: "target" }`.
- Change the left-side `from` value to `new` while keeping target.
- Submit and assert `adminAPI.accounts.cascadeModelAliasRenames(account.id, [{ old_model: "old", new_model: "new" }])` is called after `update`.
- Change only the right-side `to` value and assert cascade is not called.
- Assert no cascade call if account update fails.

**Step 2: Run test to verify failure**

Run:

```bash
cd frontend
pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts
```

Expected: fail because API/helper does not exist.

**Step 3: Add API helper**

In `frontend/src/api/admin/accounts.ts`, add:

```ts
export interface AccountModelAliasRenameInput {
  old_model: string
  new_model: string
}

export interface AccountModelAliasRenameCascadeResult {
  channel_pricing_updated: number
  channel_mappings_updated: number
  user_custom_routes_updated: number
  system_custom_routes_updated: number
  skipped: Array<{ scope: string; owner_id?: number; old_model: string; new_model: string; reason: string }>
}

export async function cascadeModelAliasRenames(
  id: number,
  renames: AccountModelAliasRenameInput[]
): Promise<AccountModelAliasRenameCascadeResult> {
  const { data } = await apiClient.post<AccountModelAliasRenameCascadeResult>(
    `/admin/accounts/${id}/model-alias-renames`,
    { renames }
  )
  return data
}
```

Export it in the default `accountsAPI` object.

**Step 4: Add frontend detection**

In `EditAccountModal.vue`:

- Store `originalAntigravityModelMappings` when loading account credentials.
- Add a helper that pairs original and current rows by stable row key and detects left-side changes when `to` is unchanged.
- After `adminAPI.accounts.update` succeeds, call cascade if `props.account.platform === 'antigravity'`.
- If cascade succeeds, show an info/success toast with total affected rows.
- If cascade has skipped rows, show warning text including skipped count.
- If cascade fails, keep the account update successful but show warning, then close as usual.

**Step 5: Run frontend tests**

Run the same Vitest command.

Expected: pass.

**Step 6: Commit**

```bash
git add frontend/src/api/admin/accounts.ts frontend/src/components/account/EditAccountModal.vue frontend/src/components/account/__tests__/EditAccountModal.spec.ts
git commit -m "feat: cascade antigravity model alias renames from UI"
```

### Task 5: Integration Verification

**Files:**
- No source edits expected.

**Step 1: Run targeted backend tests**

```bash
cd backend
go test ./internal/service ./internal/repository ./internal/handler/admin -run 'ModelAliasRename|CascadeAccountModelAliasRenames'
```

Expected: pass.

**Step 2: Run broader backend smoke tests**

```bash
cd backend
go test ./cmd/server ./internal/handler ./internal/service -run '^$'
```

Expected: pass.

**Step 3: Run targeted frontend tests**

```bash
cd frontend
pnpm exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts
```

Expected: pass.

**Step 4: Run typechecks**

```bash
cd frontend
pnpm typecheck
pnpm typecheck:tests
```

Expected: pass.

**Step 5: Manual local test**

Use a local dev server. Edit an Antigravity account mapping from `old-public`
to `new-public`, save, then verify:

- Account credentials contain `new-public`.
- Channel pricing includes `new-public`.
- User custom groups that referenced `old-public` now reference `new-public`.
- System custom groups that referenced `old-public` now reference `new-public`.
- Old pricing is still present.

**Step 6: Final commit or status**

If all checks pass and there are no unintended changes:

```bash
git status --short
```

Expected: only unrelated pre-existing local changes remain in the original dirty workspace, or a clean dedicated worktree.
