# Model Status Visibility Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an independent per-group model-status visibility allowlist that hides selected models from the model status report without changing actual model availability or request authorization.

**Architecture:** Persist a new JSONB field on `groups` with `{enabled, models}`. Thread it through the service `Group` model and admin group DTO/create/update paths, then filter the cached model-status report at response time and recompute visible metrics. Add a separate admin form section that reuses candidate loading and selection helpers but submits a distinct field from the existing request-enforcing `model_allowlist`.

**Tech Stack:** Go, Ent schema/code generation, PostgreSQL migrations, Gin handlers, Vue 3, TypeScript, Pinia/i18n, Vitest, Go unit tests.

---

### Task 1: Add persistence and domain/service configuration

**Files:**
- Create: `backend/migrations/244_group_model_status_visibility.sql`
- Modify: `backend/ent/schema/group.go`
- Modify: `backend/internal/domain/model_status_visibility.go`
- Modify: `backend/internal/service/group.go`
- Modify: generated `backend/ent/*` files via `go generate ./ent`
- Test: `backend/migrations/group_model_status_visibility_migration_test.go` and service normalization tests

**Step 1: Write failing migration/domain tests**

Add tests that assert the migration adds `groups.model_status_visibility` as `jsonb NOT NULL DEFAULT '{}'::jsonb`, and that the service configuration treats disabled/empty values as unrestricted while trimming and de-duplicating enabled entries.

**Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test ./migrations ./internal/service -run 'ModelStatusVisibility|GroupModelStatusVisibility' -count=1
```

Expected: FAIL because the column, domain type, and normalization helpers do not exist.

**Step 3: Implement the minimal persistence/domain code**

- Add `domain.GroupModelStatusVisibility` with `Enabled bool` and `Models []string`.
- Add `model_status_visibility` JSONB field to the Ent group schema with an empty default and a comment explicitly stating it only filters model-status display.
- Add `ModelStatusVisibility` to `service.Group` and `GroupModelAllowlist`-style helpers: normalize, enabled check, and `Allows`/`FilterForListing` using case-insensitive exact IDs plus trailing `*` prefix support for consistency with candidate entries.
- Add migration 244 with `ALTER TABLE groups ADD COLUMN IF NOT EXISTS model_status_visibility JSONB NOT NULL DEFAULT '{}'::jsonb;` and a comment.
- Regenerate Ent using `go generate ./ent`; do not hand-edit generated files.

**Step 4: Run tests to verify they pass**

Run the targeted migration and service tests again; expected PASS.

**Step 5: Commit**

```bash
git add backend/migrations backend/ent backend/internal/domain backend/internal/service
git commit -m "feat: add model status visibility config"
```

### Task 2: Thread the field through admin group read/write paths

**Files:**
- Modify: `backend/internal/service/admin_service.go`
- Modify: `backend/internal/service/admin_group.go`
- Modify: `backend/internal/handler/admin/group_handler.go`
- Modify: generated/repository mapping files as required by Ent regeneration
- Test: `backend/internal/service/admin_service_model_status_visibility_test.go`
- Test: `backend/internal/handler/admin/group_handler_test.go` or the focused admin DTO test location

**Step 1: Write failing service and handler tests**

Cover create and update with visibility enabled, disabled reset, normalization, and response DTO round-trip. Assert `model_allowlist` is unchanged and the new field is serialized separately.

**Step 2: Run targeted tests to verify they fail**

```bash
cd backend
go test ./internal/service ./internal/handler/admin -run 'ModelStatusVisibility|Group.*Visibility' -count=1
```

Expected: FAIL because admin inputs and DTOs do not contain the new field.

**Step 3: Implement the admin plumbing**

- Add `ModelStatusVisibility` to `CreateGroupInput`, `UpdateGroupInput`, `CreateGroupRequest`, `UpdateGroupRequest`, and `AdminGroupResponse` with pointer semantics on update.
- Map the Ent field to/from `service.Group` in the group repository/entity conversion code.
- Normalize and validate enabled entries using the new service helper. Reject enabled empty selections with a 400 error, matching existing model allowlist behavior, but keep this validation independent.
- Ensure duplicate/copy-group paths copy the new setting where group configuration is cloned.

**Step 4: Run targeted tests to verify they pass**

Run the same service and handler tests; expected PASS.

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/handler/admin backend/internal/repository backend/ent
git commit -m "feat: expose model status visibility in group admin API"
```

### Task 3: Filter model status reports without affecting requests

**Files:**
- Modify: `backend/internal/service/model_status.go`
- Test: `backend/internal/service/model_status_test.go`
- Test: `backend/internal/handler/model_status_handler_test.go` if response behavior needs coverage

**Step 1: Write failing report tests**

Add cases for disabled visibility retaining all models, enabled visibility hiding an image model, recomputing group and global metrics from visible models, and preserving the full report cache for a later configuration change. Include a regression assertion that `GroupModelAllowlist` still controls request/listing behavior independently.

**Step 2: Run tests to verify they fail**

```bash
cd backend
go test -tags=unit ./internal/service -run 'ModelStatus|Visibility' -count=1
```

Expected: FAIL because reports currently contain every catalog model.

**Step 3: Implement response-time filtering**

- Keep `buildReport` and repository aggregation complete so cache data remains reusable.
- In `filterModelStatusReport`, apply each source group’s `ModelStatusVisibility` to its `Models`, drop empty groups, recompute each visible group’s metrics, and merge visible groups into `Summary`.
- Do not touch `GroupModelAllowlist`, gateway middleware, model listing services, or account scheduling.
- Preserve bucket and recent data for models that remain visible and keep public response fields free of internal configuration.

**Step 4: Run tests to verify they pass**

Run focused service/handler tests and existing model-status tests; expected PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/model_status.go backend/internal/service/model_status_test.go backend/internal/handler/model_status_handler_test.go
git commit -m "feat: filter model status by group visibility"
```

### Task 4: Add the independent admin UI configuration

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/views/admin/groupModelAllowlist.ts` or create `frontend/src/views/admin/groupModelStatusVisibility.ts`
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/overview.ts`
- Modify: `frontend/src/i18n/locales/en/admin/overview.ts`
- Test: `frontend/src/views/admin/__tests__/GroupsView.modelStatusVisibility.spec.ts`

**Step 1: Write failing UI/helper tests**

Assert the new config is distinct from `model_allowlist`, defaults disabled, uses the same candidate selection operations, serializes selected IDs, and renders a clear notice that it only affects model monitoring and not real requests.

**Step 2: Run tests to verify they fail**

```bash
cd frontend
pnpm exec vitest run src/views/admin/__tests__/GroupsView.modelStatusVisibility.spec.ts
```

Expected: FAIL because the new field and section do not exist.

**Step 3: Implement the UI**

- Add `ModelStatusVisibility` type and create/update fields.
- Reuse existing `hydrateModelAllowlistState`, selection, inversion, custom-entry, and candidate-loading patterns without sharing mutable state with the request allowlist.
- Add separate create/edit sections titled “模型监控展示范围” / “Model status visibility”. The toggle defaults off; enabled empty selections are blocked; selected count and all/clear actions remain touch-friendly at 44px on mobile.
- Keep the existing model allowlist copy unchanged so administrators can distinguish request enforcement from monitoring display.
- Add localized strings and preserve admin workspace surface conventions.

**Step 4: Run tests and lint**

```bash
pnpm exec vitest run src/views/admin/__tests__/GroupsView.modelStatusVisibility.spec.ts
pnpm run typecheck
pnpm run lint:check
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/types frontend/src/views/admin frontend/src/i18n/locales
git commit -m "feat: add model status visibility controls"
```

### Task 5: Verify end-to-end behavior and hand off

**Files:**
- Test: targeted backend and frontend suites
- Modify: none unless verification exposes a defect

**Step 1: Run focused regression tests**

```bash
cd backend
go test -tags=unit ./internal/service ./internal/handler/admin ./internal/handler -run 'ModelStatus|Group.*Visibility|ModelAllowlist' -count=1
cd ../frontend
pnpm exec vitest run src/views/__tests__/ModelStatusView.spec.ts src/views/admin/__tests__/GroupsView.modelStatusVisibility.spec.ts
```

**Step 2: Run project checks**

```bash
pnpm run typecheck
pnpm run lint:check
pnpm run build
git diff --check
```

**Step 3: Inspect local behavior**

Open `http://localhost:3000/admin/groups`, configure a GPT group to hide its image model under “模型监控展示范围”, save it, then open `http://localhost:3000/model-status` and confirm the image model is absent while an image request remains governed only by the existing request path.

**Step 4: Commit any verification-only fixes**

Use a focused `fix:` commit and rerun the affected tests.
