# User Custom Model Groups Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let users bind one API key to a personal model collection whose models route to explicitly selected source groups and inherit those groups' scheduling and billing.

**Architecture:** Persist user-owned virtual groups and model mappings separately from administrator groups. Resolve a custom-key model to a concrete source group before gateway dispatch, then reuse the existing API-key group authorization, scheduler, channel pricing, quota, and usage paths with the resolved group as the effective group.

**Tech Stack:** Go, Gin, Ent, PostgreSQL migrations, Vue 3, TypeScript, Vitest.

---

### Task 1: Persist Custom Groups and API-Key Bindings

**Files:**
- Create: `backend/ent/schema/user_custom_group.go`
- Create: `backend/ent/schema/user_custom_group_model.go`
- Modify: `backend/ent/schema/user.go`
- Modify: `backend/ent/schema/group.go`
- Modify: `backend/ent/schema/api_key.go`
- Modify: `backend/ent/schema/usage_log.go`
- Create: `backend/migrations/194_user_custom_model_groups.sql`
- Test: `backend/internal/repository/migrations_schema_integration_test.go`

**Steps:**
1. Add failing schema/migration tests for both tables, ownership indexes, unique `(custom_group_id, public_model)`, nullable `api_keys.custom_group_id`, and nullable `usage_logs.custom_group_id`.
2. Run the migration test and verify failure.
3. Add Ent schemas/edges and forward-only SQL migration, including a check constraint that API keys cannot bind both group types.
4. Regenerate Ent and server Wire code with `cd backend && go generate ./ent && go generate ./cmd/server`.
5. Run schema and migration tests.

### Task 2: Add Domain, Repository, and Service CRUD

**Files:**
- Create: `backend/internal/service/user_custom_group.go`
- Create: `backend/internal/service/user_custom_group_service.go`
- Create: `backend/internal/service/user_custom_group_service_test.go`
- Create: `backend/internal/repository/user_custom_group_repo.go`
- Create: `backend/internal/repository/user_custom_group_repo_integration_test.go`
- Modify: `backend/internal/repository/wire.go`
- Modify: `backend/internal/service/wire.go`

**Steps:**
1. Write failing service tests for ownership isolation, duplicate models, atomic mapping replacement, stale mappings, and deletion blocked by bound keys.
2. Define repository contracts and domain errors.
3. Implement the Ent repository and service validation.
4. Re-run focused service/repository tests.

### Task 3: Expose User APIs and Candidate Models

**Files:**
- Create: `backend/internal/handler/user_custom_group_handler.go`
- Create: `backend/internal/handler/user_custom_group_handler_test.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/user.go`

**Steps:**
1. Add failing HTTP tests for list/create/get/update/delete, atomic model replacement, and candidate groups/models.
2. Implement DTO validation and JWT-owner scoping.
3. Register `/api/v1/custom-groups` routes.
4. Run handler and route tests.

### Task 4: Bind API Keys to One Group Type

**Files:**
- Modify: `backend/internal/service/api_key.go`
- Modify: `backend/internal/service/api_key_service.go`
- Modify: `backend/internal/service/api_key_auth_cache.go`
- Modify: `backend/internal/service/api_key_auth_cache_impl.go`
- Modify: `backend/internal/repository/api_key_repo.go`
- Modify: `backend/internal/handler/api_key_handler.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Test: existing API-key repository, service, handler, and auth-cache tests.

**Steps:**
1. Add failing tests for create/update validation, ownership, serialization, repository loading, and auth-cache round trips.
2. Add `CustomGroupID` and the custom-group snapshot while preserving existing concrete-key behavior.
3. Invalidate key auth cache when custom mappings or status change.
4. Run focused API-key and auth-cache tests.

### Task 5: Resolve the Effective Source Group for Text Requests

**Files:**
- Create: `backend/internal/service/custom_group_resolver.go`
- Create: `backend/internal/service/custom_group_resolver_test.go`
- Create: `backend/internal/server/middleware/custom_group.go`
- Create: `backend/internal/server/middleware/custom_group_test.go`
- Modify: `backend/internal/server/routes/gateway.go`
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`

**Steps:**
1. Add failing tests for exact model resolution, disabled/stale/access-lost mappings, unsupported endpoints, and no fallback.
2. Implement a body-preserving resolver after API-key authentication and before concrete-group middleware.
3. Clone the authenticated API key per request and attach the resolved concrete group while retaining `CustomGroupID` for usage attribution.
4. Enable only `/messages`, `/chat/completions`, `/responses`, and `/models` in phase 1; fail closed elsewhere.
5. Run gateway route and handler tests, including unchanged concrete keys.

### Task 6: Use Source-Group Billing and Record Provenance

**Files:**
- Modify: `backend/internal/service/gateway_usage_billing.go`
- Modify: `backend/internal/service/openai_usage_billing.go`
- Modify: `backend/internal/service/usage.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Modify: `backend/internal/handler/dto/types.go`
- Test: focused gateway/OpenAI billing tests and usage mapper tests.

**Steps:**
1. Write failing tests proving source-group channel price, user override, peak multiplier, subscription eligibility, and limits are used.
2. Persist both effective source `group_id` and entry `custom_group_id`.
3. Verify account cost multipliers do not alter user charges.
4. Run billing and usage tests.

### Task 7: Return the Merged Model List

**Files:**
- Modify: `backend/internal/handler/gateway_handler.go`
- Modify: `backend/internal/handler/openai_codex_models_handler.go`
- Test: `backend/internal/handler/gateway_models_test.go`
- Test: `backend/internal/handler/openai_codex_models_handler_test.go`

**Steps:**
1. Add failing tests for a custom-key merged model list and stale-model exclusion.
2. Return valid configured public models without aliases or duplicates.
3. Preserve existing provider-specific model metadata where compatible.
4. Run model endpoint tests.

### Task 8: Build the User Management UI

**Files:**
- Create: `frontend/src/api/customGroups.ts`
- Create: `frontend/src/views/user/CustomGroupsView.vue`
- Create: `frontend/src/views/user/__tests__/CustomGroupsView.spec.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/i18n/locales/zh/*`
- Modify: `frontend/src/i18n/locales/en/*`

**Steps:**
1. Write failing API/view tests for CRUD, provenance, unique model selection, stale state, and key binding.
2. Implement typed API calls and the Custom Groups page.
3. Add separate concrete/custom group choices to the API Key form.
4. Add navigation and localized labels.
5. Run focused Vitest tests and TypeScript checking.

### Task 9: Full Verification

**Steps:**
1. Run `cd backend && gofmt -w <modified-go-files>`.
2. Run `cd backend && go test ./...`.
3. Run `cd frontend && pnpm type-check`.
4. Run `cd frontend && pnpm test --run`.
5. Run `git diff --check` and review that unrelated `package.json` and `package-lock.json` remain untouched.
6. Commit implementation in coherent backend and frontend commits.
