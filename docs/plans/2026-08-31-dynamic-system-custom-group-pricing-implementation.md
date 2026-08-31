# Dynamic System Custom Groups And Pricing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace per-model system custom subscription routes with ordered live source-group references, and make newly published source models complete pricing in the same no-refresh administrator workflow.

**Architecture:** Add an ordered source-reference table while retaining legacy route rows for rollback. Build public model catalogs and request candidates dynamically from live source groups using existing scheduler and pricing predicates. Add a pricing-coverage service used by both the group write path and administrator form, then simplify the custom subscription dialog to source selection and connect source-group model selection to inline missing-price controls.

**Tech Stack:** Go, Ent, PostgreSQL migrations, Gin, Wire, Vue 3 Composition API, TypeScript, Tailwind CSS, Vitest, Go `testing` and `testify`.

---

### Task 1: Add Ordered System Custom Source References

**Files:**
- Create: `backend/ent/schema/system_custom_group_source.go`
- Modify: `backend/ent/schema/group.go`
- Create: `backend/migrations/236_system_custom_group_sources.sql`
- Create: `backend/migrations/system_custom_group_sources_migration_test.go`
- Regenerate: affected files under `backend/ent/`

**Step 1: Write the failing migration test**

Add a test that reads migration `236` and verifies the table, constraints, and route-derived backfill:

```go
func TestSystemCustomGroupSourcesMigration(t *testing.T) {
    content, err := FS.ReadFile("236_system_custom_group_sources.sql")
    require.NoError(t, err)
    sql := strings.Join(strings.Fields(string(content)), " ")
    require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS system_custom_group_sources")
    require.Contains(t, sql, "UNIQUE (group_id, source_group_id)")
    require.Contains(t, sql, "UNIQUE (group_id, priority)")
    require.Contains(t, sql, "ROW_NUMBER() OVER")
    require.Contains(t, sql, "FROM system_custom_group_models")
    require.Contains(t, sql, "WHERE NOT EXISTS")
    require.Contains(t, sql, "ON CONFLICT DO NOTHING")
}
```

**Step 2: Run the test to verify it fails**

Run: `cd backend && go test ./migrations -run TestSystemCustomGroupSourcesMigration -count=1`

Expected: FAIL because migration `236_system_custom_group_sources.sql` does not exist.

**Step 3: Add the migration and Ent schema**

Create a table with `group_id`, `source_group_id`, zero-based `priority`, timestamps, foreign keys, a no-self-reference check, and the two uniqueness constraints. Backfill distinct source IDs from retained route rows in first-route order:

```sql
INSERT INTO system_custom_group_sources (group_id, source_group_id, priority)
SELECT group_id,
       source_group_id,
       ROW_NUMBER() OVER (PARTITION BY group_id ORDER BY first_route_id) - 1
FROM (
    SELECT group_id, source_group_id, MIN(id) AS first_route_id
    FROM system_custom_group_models
    GROUP BY group_id, source_group_id
) AS existing_sources
WHERE NOT EXISTS (
    SELECT 1
    FROM system_custom_group_sources AS configured_sources
    WHERE configured_sources.group_id = existing_sources.group_id
)
ON CONFLICT DO NOTHING;
```

Add Ent edges from a container group to selected sources and from a direct group to references that use it.

**Step 4: Regenerate Ent and run schema tests**

Run: `cd backend && go generate ./ent`

Run: `cd backend && go test ./migrations ./ent/migrate -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/ent backend/migrations/236_system_custom_group_sources.sql backend/migrations/system_custom_group_sources_migration_test.go
git commit -m "feat: add system custom group source references"
```

### Task 2: Persist Sources Without Replacing Legacy Routes

**Files:**
- Modify: `backend/internal/service/system_custom_group.go`
- Modify: `backend/internal/service/system_custom_group_service.go`
- Modify: `backend/internal/repository/system_custom_group_repo.go`
- Modify: `backend/internal/repository/system_custom_group_repo_integration_test.go`
- Modify: `backend/internal/service/system_custom_group_service_test.go`

**Step 1: Write failing repository and service tests**

Cover create/update/get semantics, including this invariant:

```go
func TestSystemCustomGroupRepositoryUpdateReplacesSourcesAndPreservesLegacyRoutes(t *testing.T) {
    // Create a legacy-backed group, update source IDs to [9, 7], and assert:
    // source rows are [(9, 0), (7, 1)] while retained route rows still exist.
}
```

Also test empty, duplicate, self, deleted, inactive, nested system-custom, and unsupported source references.

**Step 2: Run focused tests to verify failure**

Run: `cd backend && go test ./internal/repository ./internal/service -run 'SystemCustomGroup.*Source' -count=1`

Expected: FAIL because source-reference repository methods and request fields are absent.

**Step 3: Add source DTOs and compatibility normalization**

Add source selection DTOs and add `source_group_ids` to create/update requests. Keep `models` as an optional compatibility input. If source IDs are empty and legacy models exist, derive ordered distinct source IDs. Reject requests with neither.

**Step 4: Implement atomic source persistence**

Update repository create/update transactions to replace source-reference rows in priority order. Do not delete or regenerate `system_custom_group_models` on dynamic updates. New dynamic groups may have no legacy model rows. Include sources in `Get`; delete source rows when deleting the container.

**Step 5: Run tests and commit**

Run: `cd backend && go test ./internal/repository ./internal/service -run SystemCustomGroup -count=1`

```bash
git add backend/internal/service/system_custom_group.go backend/internal/service/system_custom_group_service.go backend/internal/repository/system_custom_group_repo.go backend/internal/repository/system_custom_group_repo_integration_test.go backend/internal/service/system_custom_group_service_test.go
git commit -m "feat: manage custom subscriptions by source group"
```

### Task 3: Expose The Source-Based Administrator Contract

**Files:**
- Modify: `backend/internal/handler/admin/system_custom_group_handler.go`
- Modify: `backend/internal/handler/admin/system_custom_group_handler_test.go`
- Modify: `backend/internal/server/api_contract_test.go`

**Step 1: Write failing handler tests**

Update request and response assertions to require ordered `source_group_ids` and source summaries. Keep legacy `models` decoding as compatibility input.

**Step 2: Run the handler tests**

Run: `cd backend && go test ./internal/handler/admin ./internal/server -run SystemCustomGroup -count=1`

Expected: FAIL on the old model-route contract.

**Step 3: Implement response mapping**

Return the container, ordered source references, a derived summary, and retained legacy models only as a compatibility field:

```json
{
  "group": { "id": 107, "name": "..." },
  "sources": [
    { "source_group_id": 12, "priority": 0, "group": { "id": 12, "name": "...", "platform": "anthropic" } }
  ],
  "summary": { "unique_models": 0, "fallback_routes": 0, "unavailable_sources": 0, "unpriced_routes": 0 },
  "models": []
}
```

Keep the sync-preview route temporarily but mark it legacy and stop using it from the new frontend.

**Step 4: Run tests and commit**

Run: `cd backend && go test ./internal/handler/admin ./internal/server -run SystemCustomGroup -count=1`

```bash
git add backend/internal/handler/admin/system_custom_group_handler.go backend/internal/handler/admin/system_custom_group_handler_test.go backend/internal/server/api_contract_test.go
git commit -m "feat: expose source-based custom group API"
```

### Task 4: Build Dynamic Catalogs And Ordered Fallback Selection

**Files:**
- Modify: `backend/internal/service/system_custom_group_service.go`
- Modify: `backend/internal/service/gateway_system_custom_group_models.go`
- Modify: `backend/internal/service/api_key_service.go`
- Modify: `backend/internal/service/api_key_system_custom_group_test.go`
- Modify: `backend/internal/service/gateway_system_custom_group_models_test.go`
- Modify: `backend/internal/handler/system_custom_group_models_test.go`
- Modify: `backend/internal/server/routes/gateway_system_custom_group_test.go`
- Modify: `backend/internal/server/routes/system_custom_subscription_integration_test.go`

**Step 1: Write failing catalog tests**

Add tests for case-insensitive union, one public model for duplicate names, stable source priority, fallback when the first source has no schedulable supporting account, model add/remove/rename propagation, inactive/deleted/unpriced omission, and unchanged billing ownership.

```go
resolution, err := apiKeyService.ResolveSystemCustomGroupModel(ctx, key, "claude-opus-5")
require.NoError(t, err)
require.Equal(t, int64(107), resolution.BillingGroupID)
require.Equal(t, int64(22), resolution.SourceGroupID)
```

**Step 2: Run tests to verify failure**

Run: `cd backend && go test ./internal/service ./internal/handler ./internal/server/routes -run 'SystemCustom|DynamicCatalog' -count=1`

Expected: FAIL because runtime still resolves static route rows.

**Step 3: Implement one batched dynamic snapshot**

Load ordered references and source groups in one repository call. Ask the gateway catalog for current allowed/supported models and effective pricing using one schedulable-account snapshot. Normalize model keys case-insensitively, preserve first-source spelling, and retain all valid same-name candidates by priority.

**Step 4: Replace static runtime resolution**

`ListSystemCustomGroupModels` returns the unique dynamic catalog. `ResolveSystemCustomGroupModel` chooses the first valid candidate and clones its source group exactly as the static path does. Continue setting public/source context so subscription billing and usage logs remain unchanged. Do not replay across a second source after upstream dispatch.

**Step 5: Run tests and commit**

Run: `cd backend && go test ./internal/service ./internal/handler ./internal/server/routes -run 'SystemCustom|DynamicCatalog' -count=1`

```bash
git add backend/internal/service/system_custom_group_service.go backend/internal/service/gateway_system_custom_group_models.go backend/internal/service/api_key_service.go backend/internal/service/api_key_system_custom_group_test.go backend/internal/service/gateway_system_custom_group_models_test.go backend/internal/handler/system_custom_group_models_test.go backend/internal/server/routes/gateway_system_custom_group_test.go backend/internal/server/routes/system_custom_subscription_integration_test.go
git commit -m "feat: resolve custom subscription models dynamically"
```

### Task 5: Add Effective Pricing Coverage Preview And Validation

**Files:**
- Create: `backend/internal/service/group_pricing_coverage.go`
- Create: `backend/internal/service/group_pricing_coverage_test.go`
- Modify: `backend/internal/service/model_pricing_admission.go`
- Modify: `backend/internal/handler/admin/group_handler.go`
- Create: `backend/internal/handler/admin/group_pricing_coverage_test.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/server/api_contract_test.go`
- Modify: `backend/internal/service/wire.go`
- Regenerate: `backend/internal/service/wire_gen.go`

**Step 1: Write failing coverage tests**

Cover explicit prospective group pricing, existing channel pricing, LiteLLM/fallback pricing, normalized model names, per-request/image pricing, missing pricing, and incomplete pricing.

```go
func TestGroupPricingCoverageMarksUnknownModelMissing(t *testing.T) {
    result := svc.Preview(ctx, GroupPricingCoverageInput{
        Platform: "gemini",
        Models: []string{"new-unique-model"},
    })
    require.Equal(t, PricingCoverageMissing, result.Models[0].Status)
}
```

**Step 2: Run tests to verify failure**

Run: `cd backend && go test ./internal/service -run GroupPricingCoverage -count=1`

Expected: FAIL because the service does not exist.

**Step 3: Implement coverage using gateway semantics**

Extract a reusable effective-pricing predicate from admission logic. Resolve prospective models with `PricingInput{Model: model, GroupID: groupID, Group: prospectiveGroup}`. Return stable statuses `priced`, `missing`, and `invalid`, plus effective source and billing mode. Do not reproduce field rules in the browser.

**Step 4: Add the admin preview endpoint**

Register `POST /api/v1/admin/groups/pricing-coverage`. Accept optional group ID, platform, model names, and unsaved `model_pricing`. Return one result per normalized unique model.

**Step 5: Enforce newly published model coverage**

Before group create/update commits, compare prospective advertised models with stored models. Require effective pricing for newly published models. Preserve legacy unpriced models as warnings so unrelated edits are not blocked.

**Step 6: Run tests, regenerate Wire, and commit**

Run: `cd backend && go generate ./cmd/server`

Run: `cd backend && go test ./internal/service ./internal/handler/admin ./internal/server -run 'PricingCoverage|APIContract' -count=1`

```bash
git add backend/internal/service/group_pricing_coverage.go backend/internal/service/group_pricing_coverage_test.go backend/internal/service/model_pricing_admission.go backend/internal/handler/admin/group_handler.go backend/internal/handler/admin/group_pricing_coverage_test.go backend/internal/server/routes/admin.go backend/internal/server/api_contract_test.go backend/internal/service/wire.go backend/internal/service/wire_gen.go
git commit -m "feat: validate model pricing during group save"
```

### Task 6: Simplify The System Custom Group Dialog

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/groups.ts`
- Modify: `frontend/src/components/admin/groups/SystemCustomGroupDialog.vue`
- Modify: `frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/GroupsView.systemCustom.spec.ts`
- Modify: relevant Chinese and English labels under `frontend/src/i18n/locales/`

**Step 1: Rewrite failing component tests**

Assert ordered `source_group_ids`, one source row per candidate, source priority movement, derived counts, removal of alias/model/sync controls, preserved state after save errors, and no-refresh create/update/delete behavior.

**Step 2: Run tests to verify failure**

Run: `cd frontend && pnpm exec vitest run src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts src/views/admin/__tests__/GroupsView.systemCustom.spec.ts`

Expected: FAIL against the route-editor UI.

**Step 3: Update contracts and rebuild the dialog**

Replace request `models` with `source_group_ids`. Render basic settings, ordered source selection, and a dynamic summary. Use arrow icon buttons with tooltips for priority. On mobile stack rows and summaries in one column with full-width actions and no horizontal overflow.

**Step 4: Run tests and commit**

Run: `cd frontend && pnpm exec vitest run src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts src/views/admin/__tests__/GroupsView.systemCustom.spec.ts`

```bash
git add frontend/src/types/index.ts frontend/src/api/admin/groups.ts frontend/src/components/admin/groups/SystemCustomGroupDialog.vue frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts frontend/src/views/admin/__tests__/GroupsView.systemCustom.spec.ts frontend/src/i18n/locales
git commit -m "feat(frontend): select sources for custom subscriptions"
```

### Task 7: Link Model Selection To Inline Missing Pricing

**Files:**
- Create: `frontend/src/views/admin/groupsModelPricingCoverage.ts`
- Create: `frontend/src/views/admin/__tests__/groupsModelPricingCoverage.spec.ts`
- Modify: `frontend/src/api/admin/groups.ts`
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/views/admin/__tests__/GroupsView.spec.ts`
- Modify: `frontend/src/components/admin/channel/PricingEntryCard.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/groups.ts`
- Modify: `frontend/src/i18n/locales/en/admin/groups.ts`

**Step 1: Write failing helper and view tests**

Test normalization, de-duplication, preserving existing pricing entries, creating pending entries for missing models, retaining form values across previews, displaying `Pricing required`, blocking incomplete save, and submitting model-list plus pricing in one update call.

**Step 2: Run focused tests to verify failure**

Run: `cd frontend && pnpm exec vitest run src/views/admin/__tests__/groupsModelPricingCoverage.spec.ts src/views/admin/__tests__/GroupsView.spec.ts`

Expected: FAIL because pricing coverage is not connected to model selection.

**Step 3: Implement debounced preview and inline completion**

Watch selected advertised models and prospective pricing. Cancel stale requests. Append missing entries without overwriting administrator input. Re-run preview before save; on failure keep the dialog open and focus the first missing editor. Allow reviewed template copy and explicit batch application, never silent copying.

**Step 4: Finish responsive layout**

Order the workflow `Models`, `Pricing required`, `Review`. Use a compact desktop layout and one-column mobile layout with stable controls and wrapped model names.

**Step 5: Run tests and commit**

Run: `cd frontend && pnpm exec vitest run src/views/admin/__tests__/groupsModelPricingCoverage.spec.ts src/views/admin/__tests__/GroupsView.spec.ts src/components/admin/channel/__tests__/PricingEntryCard.spec.ts`

```bash
git add frontend/src/views/admin/groupsModelPricingCoverage.ts frontend/src/views/admin/__tests__/groupsModelPricingCoverage.spec.ts frontend/src/api/admin/groups.ts frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/__tests__/GroupsView.spec.ts frontend/src/components/admin/channel/PricingEntryCard.vue frontend/src/i18n/locales/zh/admin/groups.ts frontend/src/i18n/locales/en/admin/groups.ts
git commit -m "feat(frontend): price new models in the group workflow"
```

### Task 8: Verify Migration, Compatibility, Mobile UI, And Build

**Files:**
- Modify only files required by verification failures.
- Update: `docs/plans/2026-08-31-dynamic-system-custom-group-pricing-design.md` if implementation details changed materially.

**Step 1: Run backend focused and full tests**

Run: `cd backend && go test ./migrations ./internal/repository ./internal/service ./internal/handler/admin ./internal/handler ./internal/server ./internal/server/routes -count=1`

Run: `cd backend && go test ./... -count=1`

Expected: PASS.

**Step 2: Run frontend tests, type checks, lint, and build**

Run: `cd frontend && pnpm exec vitest run src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts src/views/admin/__tests__/GroupsView.systemCustom.spec.ts src/views/admin/__tests__/groupsModelPricingCoverage.spec.ts src/views/admin/__tests__/GroupsView.spec.ts`

Run: `cd frontend && pnpm run typecheck && pnpm run typecheck:tests && pnpm run lint:check && pnpm run build`

Expected: PASS.

**Step 3: Compare migration output with retained production routes**

Against a read-only production snapshot, verify that every current custom group receives the same distinct source IDs in stable order. Compare old public names with dynamic names and require zero alias drift before enabling dynamic resolution.

**Step 4: Verify desktop and mobile behavior**

Start the local application. Capture the custom group dialog and source-group editor around `1440x900`, `390x844`, and `360x800`. Verify no horizontal scrolling, clipped names, overlapping actions, page reload, or lost state.

**Step 5: Verify end to end**

1. Create a uniquely named source model with valid pricing in one save.
2. Confirm it appears in the selected custom subscription `/models` response without editing that custom group.
3. Remove or rename it and confirm the catalog follows.
4. Add an unpriced unique model and confirm source-group save is blocked clearly.
5. Add one model to two sources, make priority one unschedulable, and confirm priority two is selected while billing remains owned by the subscription container.

**Step 6: Review final state**

Run: `git status --short && git diff --check && git log --oneline --decorate -12`

Expected: clean worktree, no whitespace errors, and all task commits present.
