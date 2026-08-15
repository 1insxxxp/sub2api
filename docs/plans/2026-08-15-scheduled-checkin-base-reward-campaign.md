# Scheduled Check-in Base Reward Campaigns Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add administrator-managed Beijing-calendar campaigns that temporarily replace only the random base reward tiers while leaving all other daily check-in rules unchanged.

**Architecture:** Keep the existing settings-backed check-in configuration as the baseline and store scheduled campaigns in a dedicated Ent/PostgreSQL table. Resolve the one enabled campaign for the current Beijing date at request time, merge only its reward tiers into the baseline, and snapshot campaign identity and tiers onto each awarded check-in. Manage campaign lifecycle through dedicated admin APIs and a focused panel/dialog in the existing check-in administration page.

**Tech Stack:** Go 1.24, Ent, PostgreSQL migrations, Gin, Wire, Vue 3 Composition API, TypeScript, Tailwind CSS, Vitest, `testify`, and testcontainers-go.

---

### Task 1: Add the campaign domain model, database migration, and Ent schema

**Files:**
- Create: `backend/internal/domain/checkin_campaign.go`
- Create: `backend/ent/schema/checkin_reward_campaign.go`
- Modify: `backend/ent/schema/user_checkin.go`
- Create: `backend/migrations/222_checkin_reward_campaigns.sql`
- Create: `backend/migrations/checkin_reward_campaign_migration_test.go`
- Create: `backend/migrations/checkin_reward_campaign_migration_integration_test.go`
- Generated: `backend/ent/checkinrewardcampaign/**`
- Generated: `backend/ent/usercheckin/**`
- Generated: `backend/ent/client.go`
- Generated: `backend/ent/config.go`
- Generated: `backend/ent/context.go`
- Generated: `backend/ent/ent.go`
- Generated: `backend/ent/mutation.go`
- Generated: `backend/ent/runtime/runtime.go`
- Generated: `backend/ent/schema.go`
- Generated: `backend/ent/tx.go`

**Step 1: Write the failing migration contract test**

Add a test that reads `222_checkin_reward_campaigns.sql` from `migrations.FS` and asserts all required storage and safety clauses exist:

```go
func TestCheckinRewardCampaignMigrationContract(t *testing.T) {
    raw, err := FS.ReadFile("222_checkin_reward_campaigns.sql")
    require.NoError(t, err)
    sql := string(raw)

    require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS checkin_reward_campaigns")
    require.Contains(t, sql, "reward_tiers JSONB")
    require.Contains(t, sql, "CHECK (start_date <= end_date)")
    require.Contains(t, sql, "EXCLUDE USING gist")
    require.Contains(t, sql, "WHERE (status = 'enabled')")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reward_campaign_id")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reward_campaign_name")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reward_campaign_tiers_snapshot")
}
```

The integration test must apply the migration twice, insert non-overlapping enabled campaigns successfully, and prove overlapping enabled dates fail with PostgreSQL exclusion-constraint SQLSTATE `23P01`. It must also prove overlapping drafts are allowed.

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd backend
go test ./migrations -run 'CheckinRewardCampaignMigration' -count=1
```

Expected: FAIL because `222_checkin_reward_campaigns.sql` does not exist.

**Step 3: Add the domain types**

Create stable domain types so both Ent JSON fields and the service layer use the same shape:

```go
package domain

const (
    CheckinRewardCampaignStatusDraft    = "draft"
    CheckinRewardCampaignStatusEnabled  = "enabled"
    CheckinRewardCampaignStatusDisabled = "disabled"
)

type CheckinRewardTier struct {
    Amount      float64 `json:"amount"`
    Probability float64 `json:"probability"`
    SortOrder   int     `json:"sort_order"`
}
```

**Step 4: Add the SQL migration**

Implement an idempotent migration with this shape:

```sql
CREATE TABLE IF NOT EXISTS checkin_reward_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    reward_tiers JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by BIGINT NULL,
    updated_by BIGINT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT checkin_reward_campaigns_status_check
        CHECK (status IN ('draft', 'enabled', 'disabled')),
    CONSTRAINT checkin_reward_campaigns_date_order_check
        CHECK (start_date <= end_date)
);

CREATE INDEX IF NOT EXISTS checkin_reward_campaigns_status_dates_idx
    ON checkin_reward_campaigns (status, start_date, end_date);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'checkin_reward_campaigns_enabled_dates_excl'
    ) THEN
        ALTER TABLE checkin_reward_campaigns
            ADD CONSTRAINT checkin_reward_campaigns_enabled_dates_excl
            EXCLUDE USING gist (
                daterange(start_date, end_date, '[]') WITH &&
            ) WHERE (status = 'enabled');
    END IF;
END $$;

ALTER TABLE user_checkins
    ADD COLUMN IF NOT EXISTS reward_campaign_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS reward_campaign_name VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reward_campaign_tiers_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb;
```

Add the foreign key in an idempotent `DO $$` block with `ON DELETE RESTRICT`. Historical campaigns must not be deletable after a check-in references them.

**Step 5: Add matching Ent schemas**

Define `CheckinRewardCampaign` with PostgreSQL `date`, `jsonb`, and `timestamptz` schema types. Add a `checkins` edge. Extend `UserCheckin` with optional `reward_campaign_id`, default-empty campaign name and tiers snapshot, plus a unique inbound `reward_campaign` edge.

Use the shared domain type in both JSON fields:

```go
field.JSON("reward_tiers", []domain.CheckinRewardTier{}).
    SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
```

**Step 6: Generate Ent code**

Run:

```bash
cd backend
go generate ./ent
```

Expected: PASS and generated campaign query/create/update APIs compile.

**Step 7: Run the migration and schema tests**

Run:

```bash
cd backend
go test ./migrations -run 'CheckinRewardCampaignMigration' -count=1
DOCKER_API_VERSION=1.48 go test -tags=integration ./migrations -run 'CheckinRewardCampaignMigration' -count=1
go test -tags=unit ./ent/... -count=1
```

Expected: all PASS; integration test skips only when Docker is genuinely unavailable outside CI.

**Step 8: Commit**

```bash
git add backend/internal/domain/checkin_campaign.go \
  backend/ent backend/migrations/222_checkin_reward_campaigns.sql \
  backend/migrations/checkin_reward_campaign_migration_test.go \
  backend/migrations/checkin_reward_campaign_migration_integration_test.go
git commit -m "feat: add scheduled checkin campaign schema"
```

---

### Task 2: Extract tier validation and resolve the effective Beijing-date rule

**Files:**
- Modify: `backend/internal/service/checkin_service.go`
- Create: `backend/internal/service/checkin_reward_campaign_service.go`
- Create: `backend/internal/service/checkin_reward_campaign_service_test.go`

**Step 1: Write failing effective-rule tests**

Cover these cases with the existing SQLite Ent harness:

```go
func TestResolveEffectiveCheckinConfigUsesBaselineWithoutCampaign(t *testing.T)
func TestResolveEffectiveCheckinConfigReplacesOnlyTiers(t *testing.T)
func TestResolveEffectiveCheckinConfigIncludesStartAndEndDates(t *testing.T)
func TestResolveEffectiveCheckinConfigIgnoresDraftAndDisabled(t *testing.T)
func TestResolveEffectiveCheckinConfigRejectsMultipleActiveRows(t *testing.T)
func TestUpdateConfigRejectsConfigIncompatibleWithEnabledCampaign(t *testing.T)
```

The replacement test must assert all non-tier fields are byte-for-byte or value-for-value unchanged:

```go
require.Equal(t, daily.StreakRules, effective.Config.StreakRules)
require.Equal(t, daily.UsageRebateRatePercent, effective.Config.UsageRebateRatePercent)
require.Equal(t, daily.UsageRebateCap, effective.Config.UsageRebateCap)
require.Equal(t, daily.TotalRewardCap, effective.Config.TotalRewardCap)
require.Equal(t, campaignTiers, effective.Config.Tiers)
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'ResolveEffectiveCheckinConfig|UpdateConfigRejectsConfigIncompatible' -count=1
```

Expected: FAIL because the effective-rule resolver does not exist.

**Step 3: Extract reward-tier normalization**

Refactor the tier portion of `normalizeRewardRules` into a focused helper and keep existing behavior unchanged:

```go
func normalizeCheckinRewardTiers(tiers []CheckinRewardTier) ([]CheckinRewardTier, error)
```

Keep `CheckinRewardTier` source-compatible by aliasing it to `domain.CheckinRewardTier`:

```go
type CheckinRewardTier = domain.CheckinRewardTier
```

Run all existing reward configuration tests immediately after the refactor.

**Step 4: Implement effective-rule resolution**

Add internal result types and one resolver shared by status and award paths:

```go
type EffectiveCheckinConfig struct {
    Config   *CheckinConfig
    Campaign *CheckinRewardCampaign
}

func (s *CheckinService) resolveEffectiveCheckinConfig(
    ctx context.Context,
    client *dbent.Client,
    checkinDate string,
    baseline *CheckinConfig,
) (*EffectiveCheckinConfig, error)
```

The resolver must:

1. parse `checkinDate` in `Asia/Shanghai`;
2. query enabled campaigns with `start_date <= day <= end_date`;
3. return an explicit internal error if more than one row is found;
4. clone the baseline config;
5. replace only `Tiers`;
6. call `normalizeCheckinConfig` on the merged config;
7. return campaign identity and normalized tier snapshot.

Do not change `GetConfig`; it remains the administrator's daily baseline configuration.

**Step 5: Protect baseline config updates**

Before `UpdateConfig` persists settings, query every enabled future/current campaign and validate the proposed baseline merged with each campaign tier snapshot. Return a typed `CHECKIN_CAMPAIGN_INCOMPATIBLE_WITH_CONFIG` error containing campaign ID and name metadata instead of allowing an active campaign to become unusable.

**Step 6: Run tests**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'CheckinReward|ResolveEffectiveCheckinConfig|UpdateConfig' -count=1
```

Expected: PASS with all pre-existing reward normalization tests unchanged.

**Step 7: Commit**

```bash
git add backend/internal/service/checkin_service.go \
  backend/internal/service/checkin_reward_campaign_service.go \
  backend/internal/service/checkin_reward_campaign_service_test.go
git commit -m "feat: resolve scheduled checkin reward campaigns"
```

---

### Task 3: Implement campaign lifecycle operations and overlap protection

**Files:**
- Modify: `backend/internal/service/checkin_reward_campaign_service.go`
- Modify: `backend/internal/service/checkin_reward_campaign_service_test.go`

**Step 1: Write failing lifecycle tests**

Add tests for:

```go
func TestCheckinRewardCampaignCreateCopiesNormalizedTiers(t *testing.T)
func TestCheckinRewardCampaignUpdateAllowsDraftOnly(t *testing.T)
func TestCheckinRewardCampaignEnableRejectsOverlap(t *testing.T)
func TestCheckinRewardCampaignEnableRejectsInvalidProbability(t *testing.T)
func TestCheckinRewardCampaignEnableRejectsBaselineIncompatibility(t *testing.T)
func TestCheckinRewardCampaignDisableFallsBackImmediately(t *testing.T)
func TestCheckinRewardCampaignCopyCreatesDraft(t *testing.T)
func TestCheckinRewardCampaignDeleteAllowsUnreferencedDraftOnly(t *testing.T)
func TestCheckinRewardCampaignListDerivesLifecycle(t *testing.T)
```

Derived lifecycle values must be exactly `draft`, `upcoming`, `active`, `ended`, and `disabled`, using the service's Beijing clock rather than the database or browser timezone.

**Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'CheckinRewardCampaign(Create|Update|Enable|Disable|Copy|Delete|List)' -count=1
```

Expected: FAIL because lifecycle methods are missing.

**Step 3: Add typed request/response models and errors**

Implement explicit models without `map[string]any`:

```go
type CheckinRewardCampaign struct {
    ID               int64                  `json:"id"`
    Name             string                 `json:"name"`
    Status           string                 `json:"status"`
    LifecycleStatus  string                 `json:"lifecycle_status"`
    StartDate        string                 `json:"start_date"`
    EndDate          string                 `json:"end_date"`
    RewardTiers      []CheckinRewardTier    `json:"reward_tiers"`
    ProbabilityTotal float64                 `json:"probability_total"`
    Preview          CheckinRewardPreview   `json:"preview"`
    CreatedBy        *int64                  `json:"created_by,omitempty"`
    UpdatedBy        *int64                  `json:"updated_by,omitempty"`
    CreatedAt        time.Time               `json:"created_at"`
    UpdatedAt        time.Time               `json:"updated_at"`
}
```

Add stable errors for not found, invalid date range, invalid state transition, overlapping dates, active-history protection, and baseline incompatibility. Attach conflicting campaign ID/name/start/end as metadata to overlap errors.

**Step 4: Implement lifecycle methods**

Add these methods to `CheckinService`:

```go
ListRewardCampaigns(ctx, lifecycle string) ([]CheckinRewardCampaign, error)
GetRewardCampaign(ctx, id int64) (*CheckinRewardCampaign, error)
CreateRewardCampaign(ctx, input CreateCheckinRewardCampaignInput) (*CheckinRewardCampaign, error)
UpdateRewardCampaign(ctx, id int64, input UpdateCheckinRewardCampaignInput) (*CheckinRewardCampaign, error)
EnableRewardCampaign(ctx, id, adminID int64) (*CheckinRewardCampaign, error)
DisableRewardCampaign(ctx, id, adminID int64) (*CheckinRewardCampaign, error)
CopyRewardCampaign(ctx, id int64, name string, adminID int64) (*CheckinRewardCampaign, error)
DeleteRewardCampaign(ctx, id int64) error
```

Lifecycle mutations must use an Ent transaction. `EnableRewardCampaign` must:

1. lock/reload the target row;
2. confirm it is a draft or disabled future campaign;
3. normalize tiers;
4. validate the merged baseline;
5. query and report a human-readable overlap before update;
6. update status to enabled;
7. translate PostgreSQL exclusion constraint `23P01` to the typed overlap error.

`UpdateRewardCampaign` must only update drafts. `DeleteRewardCampaign` must only delete drafts and must explicitly check no `UserCheckin` references exist before deletion.

**Step 5: Run service and migration concurrency tests**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'CheckinRewardCampaign' -count=1
DOCKER_API_VERSION=1.48 go test -tags=integration ./migrations -run 'CheckinRewardCampaignMigration' -count=1
```

Expected: PASS; concurrent enabled overlaps leave exactly one enabled row.

**Step 6: Commit**

```bash
git add backend/internal/service/checkin_reward_campaign_service.go \
  backend/internal/service/checkin_reward_campaign_service_test.go \
  backend/migrations/checkin_reward_campaign_migration_integration_test.go
git commit -m "feat: manage checkin reward campaign lifecycle"
```

---

### Task 4: Apply campaigns to status and award transactions and persist audit snapshots

**Files:**
- Modify: `backend/internal/service/checkin_service.go`
- Modify: `backend/internal/service/checkin_service_test.go`
- Modify: `backend/internal/service/checkin_reward_campaign_service_test.go`

**Step 1: Write failing award-path tests**

Add tests proving:

```go
func TestCheckinStatusShowsActiveRewardCampaign(t *testing.T)
func TestCheckinAwardsCampaignBaseButPreservesUsageAndStreakRewards(t *testing.T)
func TestCheckinPersistsCampaignAuditSnapshot(t *testing.T)
func TestCheckinRechecksCampaignInsideTransaction(t *testing.T)
func TestAlreadyCheckedInKeepsOriginalCampaignReward(t *testing.T)
func TestCheckinAfterCampaignEndUsesBaseline(t *testing.T)
```

The main calculation assertion must demonstrate that only the base changes:

```go
require.Equal(t, 5.0, result.BaseRewardAmount)       // campaign
require.Equal(t, 4.0, result.UsageRebateAmount)      // baseline config
require.Equal(t, 10.0, result.BonusRewardAmount)     // baseline streak rule
require.Equal(t, 19.0, result.TotalRewardAmount)
require.Equal(t, campaign.ID, *result.RewardCampaignID)
```

**Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'Checkin(StatusShowsActive|AwardsCampaign|PersistsCampaign|RechecksCampaign|AlreadyCheckedInKeeps|AfterCampaignEnd)' -count=1
```

Expected: FAIL because status, award records, and DTOs do not contain campaign data.

**Step 3: Extend service DTOs and entity mapping**

Add these fields to `CheckinStatus` and `CheckinRecord`:

```go
RewardCampaignID   *int64 `json:"reward_campaign_id,omitempty"`
RewardCampaignName string `json:"reward_campaign_name,omitempty"`
```

Add `RewardCampaignTiers []CheckinRewardTier` to the internal/admin record response only if needed for audit detail; do not expose the full tier snapshot in the compact user status payload.

Update `checkinRecordFromEntity`, `alreadyCheckedInResult`, recent history, and admin list mapping.

**Step 4: Use the effective rule in both paths**

- `GetStatus`: load the daily baseline, resolve the campaign for `checkinDate`, and use the merged config for reward preview and next-step status while leaving eligibility semantics unchanged.
- `Checkin`: keep the inexpensive preflight, then resolve the effective config again with `tx.Client()` before counters and reward selection. The transactional result is authoritative.
- Create the `UserCheckin` with campaign ID/name/tier snapshot when a campaign exists.
- Keep the existing `(user_id, checkin_date)` uniqueness behavior and rollback/reload path.

Do not add a scheduler or timer.

**Step 5: Run all check-in service tests**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'Checkin' -count=1
```

Expected: PASS, including all existing baseline, usage rebate, streak, cap, blacklist, and idempotency tests.

**Step 6: Commit**

```bash
git add backend/internal/service/checkin_service.go \
  backend/internal/service/checkin_service_test.go \
  backend/internal/service/checkin_reward_campaign_service_test.go
git commit -m "feat: award scheduled checkin campaign rewards"
```

---

### Task 5: Expose campaign administration APIs

**Files:**
- Modify: `backend/internal/handler/admin/checkin_handler.go`
- Modify: `backend/internal/server/routes/admin.go`
- Create: `backend/internal/handler/admin/checkin_campaign_handler_test.go`
- Modify: `backend/internal/server/api_contract_test.go`

**Step 1: Write failing handler and route contract tests**

Define a narrow campaign interface for the handler tests and cover all routes:

```text
GET    /admin/checkins/campaigns
POST   /admin/checkins/campaigns
GET    /admin/checkins/campaigns/:id
PUT    /admin/checkins/campaigns/:id
POST   /admin/checkins/campaigns/:id/enable
POST   /admin/checkins/campaigns/:id/disable
POST   /admin/checkins/campaigns/:id/copy
DELETE /admin/checkins/campaigns/:id
```

Test success envelopes, malformed JSON, invalid IDs, invalid dates, invalid probabilities, overlap conflict metadata, invalid transitions, and unexpected errors with no internal detail leakage.

**Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
go test -tags=unit ./internal/handler/admin ./internal/server -run 'CheckinCampaign|APIContract' -count=1
```

Expected: FAIL because the campaign endpoints are not registered.

**Step 3: Implement request DTOs and handlers**

Use explicit DTOs:

```go
type UpsertCheckinRewardCampaignRequest struct {
    Name        string                     `json:"name" binding:"required"`
    StartDate   string                     `json:"start_date" binding:"required"`
    EndDate     string                     `json:"end_date" binding:"required"`
    RewardTiers []service.CheckinRewardTier `json:"reward_tiers" binding:"required"`
}

type CopyCheckinRewardCampaignRequest struct {
    Name string `json:"name" binding:"required"`
}
```

Read administrator identity from `middleware.GetAuthSubjectFromContext`. Return `201` for create/copy, `200` for reads and state transitions, and a stable `{id, deleted:true}` payload for delete.

**Step 4: Register routes and update the API contract fixture**

Add the campaign routes under the existing `/admin/checkins` group. Update any route/API golden fixtures that enumerate check-in endpoints or response fields.

**Step 5: Run handler and server tests**

Run:

```bash
cd backend
go test -tags=unit ./internal/handler/admin ./internal/server -run 'CheckinCampaign|Checkin|APIContract' -count=1
go vet -tags=unit ./internal/handler/admin ./internal/server
```

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/internal/handler/admin/checkin_handler.go \
  backend/internal/handler/admin/checkin_campaign_handler_test.go \
  backend/internal/server/routes/admin.go backend/internal/server/api_contract_test.go
git commit -m "feat: add checkin campaign admin api"
```

---

### Task 6: Add typed frontend API support

**Files:**
- Modify: `frontend/src/api/admin/checkins.ts`
- Modify: `frontend/src/api/admin/index.ts`
- Modify: `frontend/src/api/__tests__/admin.checkins.spec.ts`
- Modify: `frontend/tsconfig.type-tests.json`

**Step 1: Write failing API tests**

Add tests for every campaign method and exact method/path/body combination. Use complete fixtures with `satisfies AdminCheckinRewardCampaign`; do not use `as`, `any`, or incomplete forced casts.

**Step 2: Run tests to verify they fail**

Run:

```bash
cd frontend
pnpm vitest run src/api/__tests__/admin.checkins.spec.ts
pnpm run typecheck:tests
```

Expected: FAIL because campaign types and API functions do not exist.

**Step 3: Add typed models**

Add:

```ts
export type CheckinRewardCampaignStatus = 'draft' | 'enabled' | 'disabled'
export type CheckinRewardCampaignLifecycle =
  | 'draft'
  | 'upcoming'
  | 'active'
  | 'ended'
  | 'disabled'

export interface AdminCheckinRewardCampaign {
  id: number
  name: string
  status: CheckinRewardCampaignStatus
  lifecycle_status: CheckinRewardCampaignLifecycle
  start_date: string
  end_date: string
  reward_tiers: CheckinRewardTier[]
  probability_total: number
  preview: CheckinRewardPreview
  created_by?: number | null
  updated_by?: number | null
  created_at: string
  updated_at: string
}
```

Add typed create/update/copy requests and API functions for list/get/create/update/enable/disable/copy/delete. Preserve `AbortSignal` support on list/get operations.

**Step 4: Include the API spec in persistent test typechecking**

Ensure `tsconfig.type-tests.json` includes the updated API spec so fixture shape changes fail CI rather than being transpile-only.

**Step 5: Run API, type, and lint checks**

Run:

```bash
cd frontend
pnpm vitest run src/api/__tests__/admin.checkins.spec.ts
pnpm run typecheck:tests
pnpm run typecheck
pnpm exec eslint src/api/admin/checkins.ts src/api/__tests__/admin.checkins.spec.ts
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/api/admin/checkins.ts frontend/src/api/admin/index.ts \
  frontend/src/api/__tests__/admin.checkins.spec.ts frontend/tsconfig.type-tests.json
git commit -m "feat: add checkin campaign frontend api"
```

---

### Task 7: Build the campaign management panel and dialog

**Files:**
- Create: `frontend/src/components/admin/checkin/CheckinRewardCampaignPanel.vue`
- Create: `frontend/src/components/admin/checkin/CheckinRewardCampaignDialog.vue`
- Create: `frontend/src/components/admin/checkin/__tests__/CheckinRewardCampaignPanel.spec.ts`
- Create: `frontend/src/components/admin/checkin/__tests__/CheckinRewardCampaignDialog.spec.ts`
- Modify: `frontend/src/views/admin/CheckinsView.vue`
- Modify: `frontend/src/views/admin/__tests__/CheckinsView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/restored.ts`
- Modify: `frontend/src/i18n/locales/en/restored.ts`
- Modify: `frontend/tsconfig.type-tests.json`

**Step 1: Write failing component tests**

Cover:

- list status filters and Beijing-date labels;
- create defaults to a copy of current daily tiers;
- tier add/remove/edit with exactly `100%` probability validation;
- live min/max/average preview;
- invalid date range blocks submit;
- enabled/active/ended campaigns are read-only;
- enable and disable confirmations;
- copy opens a new editable draft name;
- overlap metadata shows the conflicting campaign and dates;
- API error details are sanitized through `extractApiErrorMessage`;
- stale list/detail requests cannot overwrite a reopened dialog;
- mutation buttons and dialog close are disabled while saving/enabling/disabling/deleting.

**Step 2: Run tests to verify they fail**

Run:

```bash
cd frontend
pnpm vitest run \
  src/components/admin/checkin/__tests__/CheckinRewardCampaignPanel.spec.ts \
  src/components/admin/checkin/__tests__/CheckinRewardCampaignDialog.spec.ts \
  src/views/admin/__tests__/CheckinsView.spec.ts
```

Expected: FAIL because the components do not exist.

**Step 3: Implement the dialog**

Use `BaseDialog`, stable `data-test` attributes, a request generation token, and one mutation state. The form owns a complete draft snapshot:

```ts
interface CampaignDraft {
  name: string
  start_date: string
  end_date: string
  reward_tiers: CheckinRewardTier[]
}
```

Use `type="date"` inputs and submit `YYYY-MM-DD` strings unchanged. Do not convert them through `Date` or UTC. Reuse the existing tier validation semantics: positive amounts, unique amounts, positive probabilities with at most two decimals, maximum 20 tiers, and exact total `100`.

**Step 4: Implement the panel**

Render status chips for `active`, `upcoming`, `ended`, `draft`, and `disabled`. Provide create/edit/view/copy/enable/disable/delete actions according to lifecycle. Refresh after every successful mutation without replacing active dialog state with stale requests.

**Step 5: Integrate into `CheckinsView`**

Place the panel after the baseline configuration and before check-in records. Pass a deep copy of `configForm.tiers` as the create default. Campaign mutations must not call `updateConfig`; daily rule saving remains independent.

**Step 6: Add Chinese and English translations**

Add all campaign labels, lifecycle names, confirmations, validation errors, conflict messages, success messages, and audit labels under `admin.checkins.campaigns`. Do not hardcode Chinese or English in Vue templates.

**Step 7: Run component and frontend checks**

Run:

```bash
cd frontend
pnpm vitest run \
  src/components/admin/checkin/__tests__/CheckinRewardCampaignPanel.spec.ts \
  src/components/admin/checkin/__tests__/CheckinRewardCampaignDialog.spec.ts \
  src/views/admin/__tests__/CheckinsView.spec.ts \
  src/api/__tests__/admin.checkins.spec.ts
pnpm run typecheck:tests
pnpm run typecheck
pnpm run lint:check
```

Expected: PASS with no unsafe type casts and no untranslated literal UI strings.

**Step 8: Commit**

```bash
git add frontend/src/components/admin/checkin frontend/src/views/admin/CheckinsView.vue \
  frontend/src/views/admin/__tests__/CheckinsView.spec.ts \
  frontend/src/i18n/locales/zh/restored.ts frontend/src/i18n/locales/en/restored.ts \
  frontend/tsconfig.type-tests.json
git commit -m "feat: manage scheduled checkin campaigns"
```

---

### Task 8: Show campaign origin in user status and administrator records

**Files:**
- Modify: `frontend/src/api/checkin.ts`
- Modify: `frontend/src/api/admin/checkins.ts`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`
- Modify: `frontend/src/views/admin/CheckinsView.vue`
- Modify: `frontend/src/views/admin/__tests__/CheckinsView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/restored.ts`
- Modify: `frontend/src/i18n/locales/en/restored.ts`

**Step 1: Write failing display tests**

Add user and admin tests that prove:

- an active campaign name is shown before check-in;
- after check-in, the awarded campaign name remains visible from the record snapshot;
- normal daily check-ins show no empty campaign chip;
- administrator records show campaign name next to the reward breakdown;
- existing base/rebate/streak/total amounts remain unchanged.

**Step 2: Run tests to verify they fail**

Run:

```bash
cd frontend
pnpm vitest run src/components/layout/__tests__/AppHeader.spec.ts \
  src/views/admin/__tests__/CheckinsView.spec.ts
```

Expected: FAIL because the campaign fields are not typed or rendered.

**Step 3: Extend API types and render campaign origin**

Add optional `reward_campaign_id` and `reward_campaign_name` to user and admin check-in types. Render a compact activity chip only when the name is non-empty. Keep the reward breakdown labels and amount formatting exactly as they are now.

**Step 4: Run affected tests and accessibility checks**

Run:

```bash
cd frontend
pnpm vitest run src/components/layout/__tests__/AppHeader.spec.ts \
  src/views/admin/__tests__/CheckinsView.spec.ts
pnpm run typecheck
pnpm exec eslint src/api/checkin.ts src/api/admin/checkins.ts \
  src/components/layout/AppHeader.vue src/views/admin/CheckinsView.vue
```

Expected: PASS. Campaign chips must not be the sole carrier of reward information and must preserve readable contrast in light/dark themes.

**Step 5: Commit**

```bash
git add frontend/src/api/checkin.ts frontend/src/api/admin/checkins.ts \
  frontend/src/components/layout/AppHeader.vue \
  frontend/src/components/layout/__tests__/AppHeader.spec.ts \
  frontend/src/views/admin/CheckinsView.vue \
  frontend/src/views/admin/__tests__/CheckinsView.spec.ts \
  frontend/src/i18n/locales/zh/restored.ts frontend/src/i18n/locales/en/restored.ts
git commit -m "feat: display checkin campaign reward origin"
```

---

### Task 9: Run final generation, regression, integration, and build verification

**Files:**
- Verify: all files changed in Tasks 1–8
- Generated/verify: `backend/cmd/server/wire_gen.go`

**Step 1: Verify generated code is stable**

Run twice and require no diff after either run:

```bash
cd backend
go generate ./ent ./cmd/server
cd ..
git diff --exit-code -- backend/ent backend/cmd/server/wire_gen.go
```

Expected: PASS. No Wire constructor change should be needed because campaign operations remain on the existing `CheckinService` and `CheckinHandler`.

**Step 2: Run backend unit tests**

Run:

```bash
cd backend
go test -tags=unit ./internal/service ./internal/handler/admin ./internal/handler ./internal/server -count=1
go test -tags=unit ./... -count=1
```

Expected: PASS. If the known unrelated ImageStudio startup race appears, record the exact failure, rerun that test independently, and do not hide any check-in-related failure.

**Step 3: Run PostgreSQL integration tests**

Run:

```bash
cd backend
DOCKER_API_VERSION=1.48 go test -tags=integration ./migrations \
  -run 'CheckinRewardCampaignMigration' -count=1
```

Expected: PASS with idempotent migration replay and enabled-overlap exclusion proven against real PostgreSQL.

**Step 4: Run backend static checks and build**

Run:

```bash
cd backend
go vet -tags=unit ./internal/service ./internal/handler/admin ./internal/handler ./internal/server ./ent/...
CGO_ENABLED=0 go build -o /tmp/sub2api-checkin-campaign-server ./cmd/server
```

Expected: PASS. Remove the temporary binary after recording success.

**Step 5: Run frontend regression checks**

Run:

```bash
cd frontend
pnpm run lint:check
pnpm run typecheck
pnpm run typecheck:tests
pnpm run test:run
pnpm run build
```

Expected: PASS with no test failures; existing Browserslist or chunk-size warnings are non-fatal only if exit code is zero.

**Step 6: Inspect the final diff and worktree**

Run:

```bash
git diff --check
git status --short
git log --oneline --decorate -10
```

Expected: no whitespace errors, no temporary fixture links/binaries, and only intentional commits/files.

**Step 7: Commit any final generated or contract-only changes**

Only if Step 1 or the final regression required legitimate tracked changes:

```bash
git add <exact-generated-or-contract-files>
git commit -m "test: verify scheduled checkin campaigns"
```

Do not commit temporary binaries, local dependency directories, database data, or ignored runtime pricing fixtures.
