# Custom Group Model Call Aliases Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let one user custom group expose multiple call names for the same real model from different source groups, with deterministic routing and source-group billing.

**Architecture:** Keep the existing `public_model`, `source_model`, and `source_group_id` schema, but redefine `public_model` as an editable call name and remove the service rule that requires it to equal `source_model`. Key frontend selection state by source-group/model identity, resolve the call name to a concrete mapping, rewrite the request model to the real model before gateway handlers run, and continue using the resolved concrete group for scheduling and billing.

**Tech Stack:** Go 1.26, Ent/PostgreSQL migrations, Gin middleware, `gjson`/`sjson`, Vue 3, TypeScript, Vitest, Tailwind CSS.

---

### Task 1: Define alias validation in the custom-group service

**Files:**
- Create: `backend/internal/service/user_custom_group_service_test.go`
- Modify: `backend/internal/service/user_custom_group_service.go:117-154`

**Step 1: Write failing service tests**

Add table-driven tests around `validateModels` with repository stubs proving:

```go
models := []UserCustomGroupModelInput{
    {PublicModel: "claude-opus-4-6-balance", SourceGroupID: 10, SourceModel: "claude-opus-4-6"},
    {PublicModel: "claude-opus-4-6-discount", SourceGroupID: 20, SourceModel: "claude-opus-4-6"},
}
```

- two distinct call names for one real model in two source groups are accepted;
- surrounding call-name whitespace is normalized;
- call names that differ only by case are rejected;
- empty and over-200-character call names are rejected;
- the same `source_group_id + source_model` mapping cannot be added twice under different call names;
- the source group must still expose `source_model`.

Use minimal user/group/gateway stubs and assert the request slice contains the normalized values after validation.

**Step 2: Run the focused tests and verify RED**

Run:

```bash
cd backend
go test ./internal/service -run 'TestUserCustomGroupValidateModels' -count=1
```

Expected: FAIL because aliases currently violate `PublicModel == SourceModel`.

**Step 3: Implement the minimal validation change**

In `validateModels`:

- retain trim, non-empty, length, source access, and source availability checks;
- remove `m.PublicModel != m.SourceModel` rejection;
- keep a case-folded `public_model` set;
- add a case-folded `source_group_id + source_model` set to reject an exact duplicate source mapping;
- continue returning `ErrUserCustomGroupInvalidModel` without leaking hidden source details.

Do not add alias-level prices, weights, priorities, or fallback behavior.

**Step 4: Run focused and package tests**

Run:

```bash
cd backend
go test ./internal/service -run 'TestUserCustomGroupValidateModels' -count=1
go test ./internal/service -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/user_custom_group_service.go backend/internal/service/user_custom_group_service_test.go
git commit -m "feat: validate custom group model call aliases"
```

### Task 2: Enforce case-insensitive call-name uniqueness in PostgreSQL

**Files:**
- Create: `backend/migrations/196_user_custom_group_call_name_casefold_notx.sql`
- Modify: `backend/migrations/user_custom_groups_migration_test.go`

**Step 1: Write a failing migration contract test**

Require migration 196 to contain an online-safe, idempotent expression index equivalent to:

```sql
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS
    uq_user_custom_group_public_model_casefold
ON user_custom_group_models (custom_group_id, LOWER(public_model));
```

Also require the filename to end in `_notx.sql` and forbid table rewrites or a regular `CREATE INDEX`.

**Step 2: Run the migration test and verify RED**

Run:

```bash
cd backend
go test ./migrations -run 'TestUserCustomGroupCallNameCasefoldMigration' -count=1
```

Expected: FAIL because migration 196 does not exist.

**Step 3: Add the online-safe migration**

Create the concurrent unique expression index only. Do not drop the existing `(custom_group_id, public_model)` constraint; it remains compatible and avoids a destructive migration.

Before deployment, query for existing case-only conflicts. The existing service already rejects case-insensitive duplicates, so a conflict is unexpected; deployment must stop rather than delete or rename user data if one is found.

**Step 4: Run migration tests**

Run:

```bash
cd backend
go test ./migrations -count=1
go test ./internal/repository -run 'Migration|Checksum' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/migrations/196_user_custom_group_call_name_casefold_notx.sql backend/migrations/user_custom_groups_migration_test.go
git commit -m "feat: enforce unique custom group call names"
```

### Task 3: Return the real model during custom-group resolution

**Files:**
- Modify: `backend/internal/service/api_key_service.go:433-453`
- Modify: `backend/internal/service/api_key_custom_group_test.go`

**Step 1: Write failing resolution tests**

Change the test route to:

```go
UserCustomGroupModel{
    PublicModel:   "claude-opus-4-6-discount",
    SourceModel:   "claude-opus-4-6",
    SourceGroupID: 42,
}
```

Assert resolution returns both:

- a request-scoped API key clone bound to group 42;
- the real model `claude-opus-4-6`.

Retain the test proving an unavailable source does not fall back.

**Step 2: Run the focused test and verify RED**

Run:

```bash
cd backend
go test ./internal/service -run 'TestResolveCustomGroupModel' -count=1
```

Expected: FAIL because the current method returns only the API key.

**Step 3: Implement the resolution result**

Introduce a small result type:

```go
type CustomGroupModelResolution struct {
    APIKey      *APIKey
    PublicModel string
    SourceModel string
}
```

Make `ResolveCustomGroupModel` return this result. Populate it only after source-group status and user access validation. Preserve the original API key and custom-group ID.

**Step 4: Run focused tests**

Run:

```bash
cd backend
go test ./internal/service -run 'TestResolveCustomGroupModel' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/api_key_service.go backend/internal/service/api_key_custom_group_test.go
git commit -m "feat: resolve custom group aliases to real models"
```

### Task 4: Rewrite alias requests before gateway dispatch

**Files:**
- Modify: `backend/internal/server/routes/gateway.go:445-482`
- Create: `backend/internal/server/routes/gateway_custom_group_alias_test.go`

**Step 1: Write failing middleware tests**

Build a Gin test router around `customGroupTargetMiddleware` and a controllable API-key service. Cover:

- JSON request `{ "model": "claude-opus-4-6-discount" }` reaches the next handler with `model` rewritten to `claude-opus-4-6`;
- the request-scoped API key uses the configured source group;
- the original alias is retained in request context for usage/audit code;
- an unknown alias returns the existing generic permission error;
- an unavailable source returns no fallback;
- when the call name equals the real model, the body remains semantically unchanged;
- multipart endpoints retain their existing supported/unsupported behavior and are never corrupted by a JSON rewrite.

**Step 2: Run the route test and verify RED**

Run:

```bash
cd backend
go test ./internal/server/routes -run 'TestCustomGroupTargetMiddlewareAlias' -count=1
```

Expected: FAIL because the middleware currently changes only the effective group.

**Step 3: Implement JSON model rewriting**

After successful resolution:

- preserve the inbound call name in a typed request context value;
- for valid JSON bodies, use `sjson.SetBytes(body, "model", resolution.SourceModel)`;
- reset the request body with the rewritten bytes;
- set the resolved API key in Gin context;
- return a bad-request error if a supported JSON request cannot be rewritten;
- do not guess at multipart reconstruction in this feature.

Add a small typed context accessor in the closest existing service/context helper location so billing and audit code do not depend on a raw string key.

**Step 4: Run route and API contract tests**

Run:

```bash
cd backend
go test ./internal/server/routes -run 'TestCustomGroupTargetMiddlewareAlias' -count=1
go test ./internal/server -run 'APIContract|CustomGroup' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_custom_group_alias_test.go backend/internal/service
git commit -m "feat: rewrite custom group aliases before dispatch"
```

### Task 5: Verify alias-only model discovery and audit identities

**Files:**
- Modify: `backend/internal/service/api_key_custom_group_test.go`
- Modify: `backend/internal/repository/usage_log_custom_group_unit_test.go`
- Modify: `backend/internal/service/gateway_usage_billing.go` only if the failing tests show the requested alias is lost

**Step 1: Add failing regression tests**

Prove that:

- `ListCustomGroupModels` returns both call names for the repeated real model;
- model discovery does not return `source_model`, source group name/ID, or multiplier metadata;
- usage creation retains `custom_group_id` and effective source `group_id`;
- requested-model audit data uses the submitted call name while resolved/upstream model data uses the real model;
- billing input continues to use the resolved source group and has no alias multiplier.

**Step 2: Run focused tests and verify behavior**

Run:

```bash
cd backend
go test ./internal/service -run 'TestListCustomGroupModels|TestCustomGroupAliasBilling' -count=1
go test ./internal/repository -run 'TestUsageLogCustomGroup' -count=1
```

Expected: discovery and billing may already pass; audit identity should fail if the alias context is not yet consumed.

**Step 3: Implement only missing identity plumbing**

Use the typed alias context from Task 4 at the existing requested-model audit boundary. Do not alter source-group pricing, scheduler selection, quota, subscription eligibility, or usage cost formulas.

**Step 4: Run service, repository, and handler tests**

Run:

```bash
cd backend
go test ./internal/service -run 'CustomGroup|Billing' -count=1
go test ./internal/repository -run 'UsageLogCustomGroup' -count=1
go test ./internal/handler -run 'Usage|Audit|CustomGroup' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/repository backend/internal/handler
git commit -m "test: preserve custom group alias billing identities"
```

### Task 6: Allow duplicate real-model selections in the responsive editor

**Files:**
- Modify: `frontend/src/components/custom-groups/CustomGroupsManager.vue`
- Modify: `frontend/src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts`

**Step 1: Write failing component source/behavior tests**

Assert the editor:

- keys selection by `source_group_id + source_model`, not `public_model`;
- allows source group 10/model X and source group 20/model X to remain selected together;
- renders an editable input labelled `调用名称` for every selected mapping;
- keeps real model and source group visible as provenance;
- updates only `public_model` when the input changes;
- shows duplicate call-name validation before save;
- stacks fields on mobile and keeps a minimum 44px touch target.

Prefer mounting the component with mocked `customGroupsAPI` for selection behavior instead of relying only on source-string assertions.

**Step 2: Run the component test and verify RED**

Run:

```bash
cd frontend
pnpm vitest run src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts
```

Expected: FAIL because `selected` is currently keyed by lower-cased model name.

**Step 3: Implement source-identity selection**

Use a stable key:

```ts
const sourceMappingKey = (sourceGroupId: number, sourceModel: string) =>
  `${sourceGroupId}:${sourceModel.toLowerCase()}`
```

When selecting a mapping:

- default `public_model` to `source_model` if unused;
- otherwise suggest `${source_model}-${source_group_name}` with whitespace normalized;
- keep the suggestion editable;
- never silently modify an alias after the user edits it.

Render selected mapping details in responsive cards or stacked rows. Update the existing hint “每个模型只能选择一个来源分组” to explain that repeated real models require distinct call names.

**Step 4: Add client-side validation**

Before calling the API:

- trim aliases;
- reject empty aliases;
- reject aliases longer than 200 characters;
- reject case-insensitive duplicate aliases;
- reject duplicate source mapping keys;
- focus or visually identify the first invalid field.

The backend remains authoritative.

**Step 5: Run component tests and type checking**

Run:

```bash
cd frontend
pnpm vitest run src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts
pnpm typecheck
pnpm lint:check
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/components/custom-groups/CustomGroupsManager.vue frontend/src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts
git commit -m "feat: edit custom group model call names"
```

### Task 7: Add end-to-end regression coverage and verify the release

**Files:**
- Modify tests discovered by the prior tasks only; do not add unrelated production changes

**Step 1: Add a cross-layer regression scenario**

Create one custom group containing two mappings for the same real model from two source groups. Verify:

1. both call names appear in `/v1/models`;
2. each call name resolves to its configured source group;
3. the dispatched body contains the real model;
4. each request uses its source group's multiplier and pricing path;
5. the usage record retains the custom group and concrete source group;
6. no source group or multiplier is exposed by model discovery.

Use service/route integration tests if the repository's full e2e harness cannot create deterministic pricing fixtures.

**Step 2: Run all backend gates**

Run:

```bash
cd backend
go test ./...
go vet ./...
golangci-lint run ./...
govulncheck ./...
```

Expected: PASS with zero lint issues and zero reachable vulnerabilities.

**Step 3: Run all frontend gates**

Run:

```bash
cd frontend
pnpm lint:check
pnpm typecheck
pnpm vitest run
pnpm build
```

Expected: all tests and the production build pass.

**Step 4: Verify migrations and diff hygiene**

Run:

```bash
cd /Users/alien/Documents/sub2
git diff --check
git status --short
```

Confirm user-owned root `package.json` and `package-lock.json` remain untracked and are not staged.

**Step 5: Commit any final test-only adjustments**

```bash
git add <only-the-final-test-files>
git commit -m "test: cover custom group duplicate real models"
```

Do not push or deploy until the user reviews the local behavior and explicitly authorizes it.
