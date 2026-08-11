# Tavern Subscription Custom Group Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an administrator-owned system custom group so one subscription API key can route explicitly named models to several Tavern source groups while sharing one daily/monthly quota and inheriting each source group’s live pricing.

**Architecture:** Store the monthly-card product as a normal subscription Group marked with system_custom_routing_enabled, and store exact public-model-to-source-group routes in a new table. Authentication and quota checks run against the original monthly-card group; a request-scoped API key clone points scheduling and pricing at the resolved source group. Request context and usage logs retain both identities so successful usage increments the original subscription while source pricing and operational attribution remain accurate.

**Tech Stack:** Go 1.24, Gin, Ent, PostgreSQL, Wire, Vue 3, TypeScript, Tailwind CSS, Vitest

**Approved design:** docs/plans/2026-08-11-tavern-subscription-custom-group-design.md

---

## Implementation constraints

- Start implementation in a dedicated worktree from the current dev HEAD; do not reuse the dirty primary workspace.
- Preserve ordinary groups, Composite routes, and user-owned custom groups.
- System routes use exact matching only. Do not add fallback, prefix matching, automatic load balancing, or automatic import of future models.
- usage_logs.group_id remains the sold/billing group. usage_logs.source_group_id records the concrete source group.
- Authentication and quota checks use the monthly group. Scheduling, channel pricing, group multiplier, and account-stat cost use the source group.
- A resolved system-custom request never falls back to wallet balance.
- Source groups do not need to be directly bindable by the buyer; an administrator-owned route grants indirect access.

### Task 1: Add persistence for routes and source attribution

**Files:**
- Create: backend/migrations/221_system_custom_group_routes.sql
- Create: backend/migrations/221a_system_custom_group_route_indexes_notx.sql
- Create: backend/migrations/system_custom_group_routes_migration_test.go
- Create: backend/ent/schema/system_custom_group_model.go
- Modify: backend/ent/schema/group.go
- Modify: backend/ent/schema/usage_log.go
- Regenerate: backend/ent/**

**Step 1: Write the failing migration contract**

Create a test that reads both migration files and requires:

~~~go
func TestSystemCustomGroupRoutesMigration(t *testing.T) {
    sql := readMigration(t, "221_system_custom_group_routes.sql")
    require.Contains(t, sql, "system_custom_routing_enabled BOOLEAN NOT NULL DEFAULT FALSE")
    require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS system_custom_group_models")
    require.Contains(t, sql, "group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE")
    require.Contains(t, sql, "source_group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS source_group_id BIGINT NULL")
    require.Contains(t, sql, "CHECK (group_id <> source_group_id)")
}
~~~

Reuse the read helper pattern from existing backend/migrations/*_migration_test.go tests.

**Step 2: Run the test and verify it fails**

Run:

~~~bash
cd backend && go test ./migrations -run TestSystemCustomGroupRoutesMigration -count=1
~~~

Expected: FAIL because the migration files do not exist.

**Step 3: Add the schema and idempotent SQL**

Add groups.system_custom_routing_enabled, create system_custom_group_models, and add nullable usage_logs.source_group_id. The route table contains group_id, public_model, source_group_id, source_model, enabled, created_at, and updated_at.

Create case-insensitive unique indexes:

~~~sql
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_system_custom_group_public_model_ci
ON system_custom_group_models(group_id, LOWER(public_model));

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_system_custom_group_source_model_ci
ON system_custom_group_models(group_id, source_group_id, LOWER(source_model));
~~~

Add Group edges for owned routes and source routes. Add a UsageLog source-group edge without changing the existing billing-group edge.

**Step 4: Regenerate Ent and format**

Run:

~~~bash
cd backend
go generate ./ent
gofmt -w ent/schema/system_custom_group_model.go ent/schema/group.go ent/schema/usage_log.go migrations/system_custom_group_routes_migration_test.go
~~~

Expected: Ent generates systemcustomgroupmodel builders, predicates, mutations, and client/query types.

**Step 5: Verify and commit**

~~~bash
cd backend && go test ./migrations ./ent/... -count=1
git add backend/migrations/221_system_custom_group_routes.sql backend/migrations/221a_system_custom_group_route_indexes_notx.sql backend/migrations/system_custom_group_routes_migration_test.go backend/ent/schema backend/ent
git commit -m "feat: add system custom group route schema"
~~~

Expected: tests pass and the commit contains only schema, migration, test, and generated Ent changes.

### Task 2: Implement validated transactional management

**Files:**
- Create: backend/internal/service/system_custom_group.go
- Create: backend/internal/service/system_custom_group_service.go
- Create: backend/internal/service/system_custom_group_service_test.go
- Create: backend/internal/repository/system_custom_group_repo.go
- Create: backend/internal/repository/system_custom_group_repo_integration_test.go
- Modify: backend/internal/service/group.go
- Modify: backend/internal/repository/group_repo.go

**Step 1: Write failing invariant tests**

Cover:

~~~go
func TestCreateSystemCustomGroupNormalizesContainer(t *testing.T) {}
func TestValidateSystemCustomRoutesRejectsDuplicatePublicNameCaseInsensitive(t *testing.T) {}
func TestValidateSystemCustomRoutesRejectsDuplicateSourceModel(t *testing.T) {}
func TestValidateSystemCustomRoutesRejectsSelfReference(t *testing.T) {}
func TestValidateSystemCustomRoutesRejectsInactiveOrNestedSource(t *testing.T) {}
func TestValidateSystemCustomRoutesPreservesExplicitAliases(t *testing.T) {}
func TestSyncPreviewMarksAddedMissingAndConflictingModels(t *testing.T) {}
~~~

The normalized container must be PlatformComposite, SubscriptionTypeSubscription, exclusive, rate 1, and system-custom enabled. Unique imports retain their original names; a second source containing the same name requires an alias.

**Step 2: Verify failure**

~~~bash
cd backend && go test ./internal/service -run 'Test(CreateSystemCustomGroup|ValidateSystemCustomRoutes|SyncPreview)' -count=1
~~~

Expected: FAIL because the service and types do not exist.

**Step 3: Add domain types and errors**

Define dedicated types rather than reusing UserCustomGroup:

~~~go
type SystemCustomGroupModelInput struct {
    PublicModel   string `json:"public_model"`
    SourceGroupID int64  `json:"source_group_id"`
    SourceModel   string `json:"source_model"`
    Enabled       bool   `json:"enabled"`
}

type CreateSystemCustomGroupRequest struct {
    Name                string                         `json:"name"`
    Description         *string                        `json:"description"`
    DailyLimitUSD       *float64                       `json:"daily_limit_usd"`
    WeeklyLimitUSD      *float64                       `json:"weekly_limit_usd"`
    MonthlyLimitUSD     *float64                       `json:"monthly_limit_usd"`
    DefaultValidityDays int                            `json:"default_validity_days"`
    Models              []SystemCustomGroupModelInput  `json:"models"`
}
~~~

Add stable typed errors for not found, duplicate public model, duplicate source model, invalid source group, missing source model, and self-reference. Include the conflicting model in the error message.

Add SystemCustomRoutingEnabled and IsSystemCustomRouteGroup() to the service Group model and all Ent/service mappings.

**Step 4: Implement repository transactions**

Expose:

~~~go
type SystemCustomGroupRepository interface {
    Create(ctx context.Context, group *Group, models []SystemCustomGroupModelInput) error
    Update(ctx context.Context, group *Group, models []SystemCustomGroupModelInput) error
    Get(ctx context.Context, groupID int64) (*SystemCustomGroup, error)
    ListModels(ctx context.Context, groupID int64, enabledOnly bool) ([]SystemCustomGroupModel, error)
    ResolveModel(ctx context.Context, groupID int64, publicModel string) (*SystemCustomGroupModel, error)
}
~~~

Create inserts the Group and all routes in one Ent transaction. Update replaces the entire route snapshot atomically. Put the scheduler group-change outbox event in the same transaction. Repository integration tests must prove rollback when any route fails.

**Step 5: Implement candidate and sync preview**

Candidates contain active direct source groups only. Exclude Composite and nested system-custom groups. Candidate models must use the same available-model computation as the gateway and respect a source group’s custom model list.

Sync preview returns added, missing, and conflicting entries. Added entries are preview-only and unselected; missing entries remain stored until the administrator confirms replacement.

**Step 6: Verify and commit**

~~~bash
cd backend && go test ./internal/service ./internal/repository -run 'SystemCustomGroup' -count=1
git add backend/internal/service/system_custom_group.go backend/internal/service/system_custom_group_service.go backend/internal/service/system_custom_group_service_test.go backend/internal/service/group.go backend/internal/repository/system_custom_group_repo.go backend/internal/repository/system_custom_group_repo_integration_test.go backend/internal/repository/group_repo.go
git commit -m "feat: manage system custom subscription groups"
~~~

### Task 3: Expose administrator APIs and wire dependencies

**Files:**
- Create: backend/internal/handler/admin/system_custom_group_handler.go
- Create: backend/internal/handler/admin/system_custom_group_handler_test.go
- Modify: backend/internal/handler/handler.go
- Modify: backend/internal/handler/wire.go
- Modify: backend/internal/repository/wire.go
- Modify: backend/internal/service/wire.go
- Modify: backend/internal/server/routes/admin.go
- Modify: backend/internal/server/api_contract_test.go
- Modify: backend/cmd/server/wire_gen.go

**Step 1: Write failing route and response tests**

Require:

~~~text
GET    /api/v1/admin/system-custom-groups/candidates
POST   /api/v1/admin/system-custom-groups
GET    /api/v1/admin/system-custom-groups/:id
PUT    /api/v1/admin/system-custom-groups/:id
GET    /api/v1/admin/system-custom-groups/:id/sync-preview
DELETE /api/v1/admin/system-custom-groups/:id
~~~

Duplicate aliases return 409 with a stable code; invalid sources return 400; unexpected repository failures use the standard error envelope instead of a bare internal error. Deletion refuses active plan/subscription references.

**Step 2: Verify failure**

~~~bash
cd backend && go test ./internal/handler/admin ./internal/server -run 'SystemCustomGroup|APIContract' -count=1
~~~

**Step 3: Implement handlers, admin routes, and Wire providers**

Keep handlers thin. Register the routes under existing admin authentication, audit, and compliance middleware. Add the handler to AdminHandlers. Register repository and service providers, then regenerate Wire:

~~~bash
cd backend
go generate ./cmd/server
gofmt -w internal/handler/admin/system_custom_group_handler.go internal/handler/handler.go internal/handler/wire.go internal/repository/wire.go internal/service/wire.go internal/server/routes/admin.go cmd/server/wire_gen.go
~~~

**Step 4: Verify and commit**

~~~bash
cd backend && go test ./internal/handler/admin ./internal/server ./cmd/server -run 'SystemCustomGroup|APIContract' -count=1
git add backend/internal/handler/admin/system_custom_group_handler.go backend/internal/handler/admin/system_custom_group_handler_test.go backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/repository/wire.go backend/internal/service/wire.go backend/internal/server/routes/admin.go backend/internal/server/api_contract_test.go backend/cmd/server/wire_gen.go
git commit -m "feat: expose system custom group admin api"
~~~

### Task 4: Resolve public models to source groups per request

**Files:**
- Modify: backend/internal/pkg/ctxkey/ctxkey.go
- Modify: backend/internal/service/composite_platform.go
- Modify: backend/internal/service/api_key_service.go
- Create: backend/internal/service/api_key_system_custom_group_test.go
- Modify: backend/internal/server/middleware/api_key_auth.go
- Modify: backend/internal/server/routes/gateway.go
- Create: backend/internal/server/routes/gateway_system_custom_group_test.go

**Step 1: Write failing resolver and middleware tests**

Prove that resolution clones the API key, preserves the original monthly group, bypasses direct buyer access checks for the source, rejects inactive/nested sources without fallback, rewrites JSON and multipart models, and rewrites Gemini path models. Also assert that the general request Group context becomes the source group while SystemCustomGroupResolution retains the billing group.

**Step 2: Verify failure**

~~~bash
cd backend && go test ./internal/service ./internal/server/routes -run 'SystemCustom' -count=1
~~~

**Step 3: Add dual-group request context**

~~~go
type SystemCustomGroupResolution struct {
    BillingGroupID int64
    SourceGroupID  int64
    PublicModel    string
    SourceModel    string
    SourcePlatform string
}

func WithSystemCustomGroupResolution(ctx context.Context, r SystemCustomGroupResolution) context.Context
func SystemCustomGroupResolutionFromContext(ctx context.Context) (SystemCustomGroupResolution, bool)
~~~

Reuse the existing requested-public-model and resolved-upstream-model context values, while adding dedicated billing/source group IDs.

**Step 4: Implement runtime resolution**

For key.Group.IsSystemCustomRouteGroup():

1. Resolve an exact enabled public model under the billing group.
2. Load the active source group.
3. Reject Composite, nested system-custom, deleted, or disabled sources.
4. Clone the key and point only the clone’s GroupID/Group at the source.
5. Do not call canUserBindGroup for the source.
6. Return both group IDs and model names.

**Step 5: Install middleware in the correct order**

For model-bearing root and Gemini routes:

~~~text
apiKeyAuth → systemCustomTarget → userCustomTarget → compositeTarget → requireGroup → handler
~~~

Unknown aliases return a clear model-not-allowed response. Missing/disabled sources return 503 model-unavailable. Never try another route.

After replacing the request-scoped API key, replace the ordinary Group context with the resolved source group so downstream pricing and scheduler helpers cannot accidentally read the monthly container. Keep the monthly group only in SystemCustomGroupResolution and the already-loaded subscription context.

**Step 6: Verify and commit**

~~~bash
cd backend && go test ./internal/service ./internal/server/routes -run 'SystemCustom' -count=1
git add backend/internal/pkg/ctxkey/ctxkey.go backend/internal/service/composite_platform.go backend/internal/service/api_key_service.go backend/internal/service/api_key_system_custom_group_test.go backend/internal/server/middleware/api_key_auth.go backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_system_custom_group_test.go
git commit -m "feat: route system custom group requests"
~~~

### Task 5: Restrict model-list endpoints to configured aliases

**Files:**
- Modify: backend/internal/service/api_key_service.go
- Modify: backend/internal/handler/gateway_handler.go
- Modify: backend/internal/handler/gemini_v1beta_handler.go
- Create: backend/internal/handler/system_custom_group_models_test.go
- Modify: backend/internal/handler/gemini_custom_group_models_test.go

**Step 1: Write failing model-list tests**

Require /v1/models to return enabled public aliases only. Require /v1beta/models to return only aliases whose source platform is Gemini. Do not expose source model names. Omit disabled routes and unavailable sources.

**Step 2: Verify failure**

~~~bash
cd backend && go test ./internal/handler -run 'SystemCustomGroupModels|Gemini.*CustomGroupModels' -count=1
~~~

**Step 3: Implement list helpers**

Add ListSystemCustomGroupModels(ctx, key, platform). Check the billing-group flag, list enabled routes, validate source availability, optionally filter platform, deduplicate case-insensitively, and sort aliases. Branch before ordinary Composite handling in both model-list handlers.

**Step 4: Verify and commit**

~~~bash
cd backend && go test ./internal/handler ./internal/service -run 'SystemCustomGroupModels|Gemini.*CustomGroupModels' -count=1
git add backend/internal/service/api_key_service.go backend/internal/handler/gateway_handler.go backend/internal/handler/gemini_v1beta_handler.go backend/internal/handler/system_custom_group_models_test.go backend/internal/handler/gemini_custom_group_models_test.go
git commit -m "feat: list system custom group models"
~~~

### Task 6: Bill Anthropic and Gemini traffic to the shared subscription

**Files:**
- Modify: backend/internal/service/gateway_usage_billing.go
- Modify: backend/internal/service/gateway_service_subscription_billing_test.go
- Modify: backend/internal/service/gateway_record_usage_test.go
- Modify: backend/internal/service/usage_log.go

**Step 1: Write failing billing tests**

Create two source groups with different multipliers/prices, one monthly billing group, one subscription, and two routes. For each route assert:

- the source group determines ActualCost;
- both costs increment the same UserSubscription.ID;
- wallet deduction is never called;
- UsageLog.GroupID is the monthly group;
- UsageLog.SourceGroupID is the concrete source;
- RequestedModel is the public alias;
- Model and UpstreamModel retain the actual model chain.

Also verify that missing identifiable pricing fails and cannot create a successful zero-cost record.

**Step 2: Run and verify failure**

~~~bash
cd backend && go test ./internal/service -run 'SystemCustom.*(Billing|Usage)|GatewayServiceSubscription' -count=1
~~~

Expected: FAIL because the cloned source is a standard group, so current code classifies the request as balance billing and logs the source as group_id.

**Step 3: Separate billing identity from pricing identity**

Continue using the cloned source apiKey.Group for multiplier, channel pricing, peak pricing, and account-stat cost. Determine subscription billing with the original subscription plus the system-custom context:

~~~go
resolution, systemCustom := SystemCustomGroupResolutionFromContext(ctx)
isSubscriptionBilling := subscription != nil &&
    ((apiKey.Group != nil && apiKey.Group.IsSubscriptionType()) || systemCustom)
~~~

When systemCustom is true:

~~~go
usageLog.GroupID = &resolution.BillingGroupID
usageLog.SourceGroupID = &resolution.SourceGroupID
~~~

For ordinary calls, preserve current GroupID behavior and leave SourceGroupID nil. applyUsageBilling must receive IsSubscriptionBill=true so it increments the loaded subscription and never deducts wallet balance.

**Step 4: Verify and commit**

~~~bash
cd backend && go test ./internal/service -run 'SystemCustom.*(Billing|Usage)|GatewayServiceSubscription' -count=1
git add backend/internal/service/gateway_usage_billing.go backend/internal/service/gateway_service_subscription_billing_test.go backend/internal/service/gateway_record_usage_test.go backend/internal/service/usage_log.go
git commit -m "feat: bill routed usage to shared subscription"
~~~

### Task 7: Apply the billing contract to OpenAI and persist source_group_id

**Files:**
- Modify: backend/internal/service/openai_gateway_usage.go
- Modify: backend/internal/service/openai_gateway_record_usage_test.go
- Modify: backend/internal/repository/usage_log_repo_insert.go
- Modify: backend/internal/repository/usage_log_repo_query.go
- Modify: backend/internal/repository/usage_log_repo.go
- Modify: backend/internal/repository/usage_log_repo_unit_test.go
- Modify: backend/internal/repository/usage_log_repo_integration_test.go
- Modify: backend/internal/handler/dto/types.go
- Modify: backend/internal/handler/dto/mappers.go

**Step 1: Write failing OpenAI and repository tests**

Mirror Task 6 for OpenAI RecordUsage. Add a repository round-trip test proving source_group_id survives insert, query, and DTO mapping while group_id remains the monthly/sold group.

**Step 2: Verify failure**

~~~bash
cd backend && go test ./internal/service ./internal/repository ./internal/handler/dto -run 'SystemCustom|SourceGroupID' -count=1
~~~

Expected: FAIL because OpenAI still checks the source group’s subscription type and raw SQL does not include source_group_id.

**Step 3: Implement dual-group billing and SQL plumbing**

Use the same context classification as Task 6. Update every raw usage-log insert column list, placeholder list, prepared argument list, select column constant, scan target, service struct, and DTO mapper in the same change. Historical rows remain compatible because source_group_id is nullable.

Keep group dashboards and group filters based on billing group_id. SourceGroupID is additional detail/operational attribution; do not silently change existing aggregate semantics.

**Step 4: Verify and commit**

~~~bash
cd backend && go test ./internal/service ./internal/repository ./internal/handler/dto -run 'SystemCustom|SourceGroupID' -count=1
git add backend/internal/service/openai_gateway_usage.go backend/internal/service/openai_gateway_record_usage_test.go backend/internal/repository/usage_log_repo_insert.go backend/internal/repository/usage_log_repo_query.go backend/internal/repository/usage_log_repo.go backend/internal/repository/usage_log_repo_unit_test.go backend/internal/repository/usage_log_repo_integration_test.go backend/internal/handler/dto/types.go backend/internal/handler/dto/mappers.go
git commit -m "feat: record source group for routed usage"
~~~

### Task 8: Add typed frontend API support

**Files:**
- Modify: frontend/src/types/index.ts
- Modify: frontend/src/api/admin/groups.ts
- Create: frontend/src/api/__tests__/admin.groups.systemCustom.spec.ts

**Step 1: Write failing API tests**

Mock apiClient and verify exact methods and URLs for candidates, create, get, update, sync preview, and delete. Assert payloads preserve explicit aliases and enabled flags.

**Step 2: Verify failure**

~~~bash
npm --prefix frontend run test:run -- src/api/__tests__/admin.groups.systemCustom.spec.ts
~~~

Expected: FAIL because the API functions do not exist.

**Step 3: Add types and methods**

Add system_custom_routing_enabled to Group/AdminGroup and define:

~~~ts
export interface SystemCustomGroupModelInput {
  public_model: string
  source_group_id: number
  source_model: string
  enabled: boolean
}

export interface SystemCustomGroupCandidate {
  id: number
  name: string
  platform: Exclude<GroupPlatform, 'composite'>
  models: string[]
}
~~~

Add the six endpoint functions to groupsAPI with typed requests and responses.

**Step 4: Verify and commit**

~~~bash
npm --prefix frontend run test:run -- src/api/__tests__/admin.groups.systemCustom.spec.ts
git add frontend/src/types/index.ts frontend/src/api/admin/groups.ts frontend/src/api/__tests__/admin.groups.systemCustom.spec.ts
git commit -m "feat: add system custom group frontend api"
~~~

### Task 9: Build the Group Management import and sync interface

**Files:**
- Create: frontend/src/components/admin/groups/SystemCustomGroupDialog.vue
- Create: frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts
- Modify: frontend/src/views/admin/GroupsView.vue
- Modify: frontend/src/views/admin/__tests__/GroupsView.spec.ts
- Modify: frontend/src/i18n/locales/zh/admin/overview.ts
- Modify: frontend/src/i18n/locales/en/admin/overview.ts

**Step 1: Write failing component tests**

Test this workflow:

1. Open “新建自定义路由分组”.
2. Enter name and daily/monthly limits.
3. Select multiple source groups.
4. View candidate models grouped by source.
5. Explicitly select models; unique names remain unchanged.
6. Create a duplicate public name and verify Save is disabled.
7. Rename the duplicate to 模型名@来源简称 and verify the conflict clears.
8. Open sync preview; added models are unselected and missing models are invalid/disable suggestions.
9. Confirm sends one complete route snapshot.
10. Backend validation details are displayed instead of generic internal error.

**Step 2: Verify failure**

~~~bash
npm --prefix frontend run test:run -- src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts src/views/admin/__tests__/GroupsView.spec.ts
~~~

**Step 3: Implement the focused dialog**

Keep GroupsView.vue thin: it owns open/close/refresh state. The new component owns candidate loading, source selection, model selection, alias editing, conflict validation, sync preview, route snapshot creation, and API errors.

Add “新建自定义路由分组” beside ordinary group creation. System-custom rows get a visible type badge and “管理路由” action. Do not show ordinary Composite-route controls on those rows.

Use stable test IDs:

~~~text
system-custom-create
system-custom-source-select
system-custom-model-row
system-custom-public-model
system-custom-conflict
system-custom-sync
system-custom-save
~~~

**Step 4: Run checks and commit**

~~~bash
npm --prefix frontend run test:run -- src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts src/views/admin/__tests__/GroupsView.spec.ts
npm --prefix frontend run typecheck
git add frontend/src/components/admin/groups/SystemCustomGroupDialog.vue frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/__tests__/GroupsView.spec.ts frontend/src/i18n/locales/zh/admin/overview.ts frontend/src/i18n/locales/en/admin/overview.ts
git commit -m "feat: manage system custom groups in admin"
~~~

### Task 10: Add cross-layer quota and closed-failure regressions

**Files:**
- Create: backend/internal/server/routes/system_custom_subscription_integration_test.go
- Modify: backend/internal/server/middleware/api_key_auth_test.go
- Modify: backend/internal/service/subscription_daily_midnight_reset_test.go
- Modify: backend/internal/service/subscription_monthly_window_test.go

**Step 1: Write the end-to-end service test**

Create one billing group, two source groups, two explicit routes, one user subscription, and one API key. Make successful calls through both routes and assert:

~~~text
route A actual cost + route B actual cost
    = the same subscription's usage increment
wallet deduction calls
    = 0
usage billing group IDs
    = monthly group ID for both
usage source group IDs
    = source A and source B
~~~

Exhaust daily quota and require HTTP 429 USAGE_LIMIT_EXCEEDED with no wallet fallback and no upstream selection. Repeat for monthly quota.

**Step 2: Add closed-failure cases**

Require deterministic failure for unknown public model, disabled route, disabled source group, removed source model, missing pricing, and no available account. None may switch source or record a zero-price success.

**Step 3: Verify and commit**

~~~bash
cd backend && go test ./internal/server/routes ./internal/server/middleware ./internal/service -run 'SystemCustom|Subscription.*(Daily|Monthly)' -count=1
git add backend/internal/server/routes/system_custom_subscription_integration_test.go backend/internal/server/middleware/api_key_auth_test.go backend/internal/service/subscription_daily_midnight_reset_test.go backend/internal/service/subscription_monthly_window_test.go
git commit -m "test: cover shared custom group subscription quota"
~~~

### Task 11: Run complete verification and local smoke tests

**Files:**
- Verify: backend/**
- Verify: frontend/**
- Verify: deploy/docker-compose.dev.yml

**Step 1: Check formatting and generated code**

~~~bash
cd backend
git diff --name-only --diff-filter=ACM -- '*.go' | xargs gofmt -w
go generate ./ent ./cmd/server
cd ..
git diff --exit-code -- backend/ent backend/cmd/server/wire_gen.go
~~~

Expected: generation is reproducible and leaves no new diff.

**Step 2: Run backend gates**

~~~bash
cd backend && go test ./...
cd backend && golangci-lint run ./...
cd backend && CGO_ENABLED=0 go build ./cmd/server
~~~

Expected: all exit 0.

**Step 3: Run frontend gates**

~~~bash
npm --prefix frontend run lint:check
npm --prefix frontend run typecheck
npm --prefix frontend run test:run
npm --prefix frontend run build
~~~

Expected: all exit 0.

**Step 4: Rebuild only the local application container**

~~~bash
docker compose -f deploy/docker-compose.dev.yml build sub2api
docker compose -f deploy/docker-compose.dev.yml up -d --no-deps --force-recreate sub2api
docker inspect --format '{{.State.Health.Status}}' sub2api-dev
curl -fsS http://127.0.0.1:18080/health
~~~

Expected: healthy and {"status":"ok"}.

**Step 5: Perform local product smoke checks**

- Create two local source groups with different prices and unique model names.
- Create an internal system custom monthly group from both.
- Assign one subscription and create one Key.
- Call both public model names.
- Confirm the same subscription usage increases by the two source-priced amounts.
- Confirm usage details show the monthly group plus each source group.
- Exhaust daily quota and confirm 429 with unchanged wallet balance.
- Disable one source and confirm only its model fails with no reroute.

**Step 6: Inspect final state**

~~~bash
git status --short
git diff --check
git log --oneline --decorate -12
~~~

Expected: no generated drift and no unstaged implementation files. Do not deploy to production in this plan; production blue-green deployment needs separate approval after local verification and review.
