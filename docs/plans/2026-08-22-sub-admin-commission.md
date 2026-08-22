# Secondary Admin Commission Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a first-version secondary-admin commission system in the admin workbench with super-admin group assignment, a global commission rate, calendar consumption totals, and day-level drilldown.

**Architecture:** Add a dedicated commission grant table and scoped service/repository APIs instead of reusing calling permissions. Super-admin endpoints manage settings and grants under `/api/v1/admin/sub-admin-commissions`; secondary-admin workbench endpoints expose only the current user's assigned group data under `/api/v1/admin/workbench/commission`.

**Tech Stack:** Go 1.26, Gin, Ent, PostgreSQL SQL migrations, Vue 3, TypeScript, Vitest, Tailwind.

---

## Current Constraints

- The current branch is `dev`.
- The worktree already has unrelated local changes in redeem/admin workbench files. Do not revert them.
- `docs/*` is ignored, so design/plan docs require `git add -f` if they should be committed.
- Local backend/frontend dev servers are already running; implementation can rely on HMR after files are changed.
- Do not deploy as part of this plan.

## Endpoint Contract

Super-admin-only:

- `GET /api/v1/admin/sub-admin-commissions/settings`
- `PUT /api/v1/admin/sub-admin-commissions/settings`
- `GET /api/v1/admin/sub-admin-commissions/grants`
- `PUT /api/v1/admin/sub-admin-commissions/grants/:sub_admin_id`

Workbench scoped:

- `GET /api/v1/admin/workbench/commission/grants`
- `GET /api/v1/admin/workbench/commission/calendar?month=YYYY-MM`
- `GET /api/v1/admin/workbench/commission/days/:date/groups`
- `GET /api/v1/admin/workbench/commission/days/:date/groups/:group_id/logs?page=1&page_size=20`

Response shapes:

```go
type SubAdminCommissionSettingsResponse struct {
	CommissionRate float64 `json:"commission_rate"`
}

type SubAdminCommissionGrantResponse struct {
	ID            int64  `json:"id"`
	SubAdminID    int64  `json:"sub_admin_id"`
	SubAdminEmail string `json:"sub_admin_email,omitempty"`
	GroupID       int64  `json:"group_id"`
	GroupName     string `json:"group_name"`
	GrantedDate   string `json:"granted_date"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type SubAdminCommissionCalendarDayResponse struct {
	Date             string  `json:"date"`
	Enabled          bool    `json:"enabled"`
	ActualCost       float64 `json:"actual_cost"`
	CommissionAmount float64 `json:"commission_amount"`
}

type SubAdminCommissionDayGroupResponse struct {
	GroupID          int64   `json:"group_id"`
	GroupName        string  `json:"group_name"`
	Requests         int64   `json:"requests"`
	TotalTokens      int64   `json:"total_tokens"`
	ActualCost       float64 `json:"actual_cost"`
	CommissionAmount float64 `json:"commission_amount"`
}
```

---

### Task 1: Add Database Schema And Migration

**Files:**

- Create: `backend/ent/schema/sub_admin_commission_grant.go`
- Create: `backend/migrations/224_sub_admin_commission_grants.sql`
- Create: `backend/migrations/sub_admin_commission_grants_migration_test.go`
- Generated: `backend/ent/**`

**Step 1: Write the migration test**

Create `backend/migrations/sub_admin_commission_grants_migration_test.go`:

```go
package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubAdminCommissionGrantsMigration(t *testing.T) {
	sql, err := os.ReadFile("224_sub_admin_commission_grants.sql")
	require.NoError(t, err)
	body := string(sql)

	require.Contains(t, body, "CREATE TABLE IF NOT EXISTS sub_admin_commission_grants")
	require.Contains(t, body, "sub_admin_user_id BIGINT NOT NULL REFERENCES users(id)")
	require.Contains(t, body, "group_id BIGINT NOT NULL REFERENCES groups(id)")
	require.Contains(t, body, "granted_date DATE NOT NULL")
	require.Contains(t, body, "enabled BOOLEAN NOT NULL DEFAULT TRUE")
	require.Contains(t, body, "CREATE UNIQUE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_active_unique")
	require.Contains(t, body, "WHERE enabled = TRUE")
}
```

**Step 2: Run the test to verify it fails**

Run:

```bash
cd backend && go test ./migrations -run TestSubAdminCommissionGrantsMigration -count=1
```

Expected: FAIL because the migration file does not exist.

**Step 3: Add the SQL migration**

Create `backend/migrations/224_sub_admin_commission_grants.sql`:

```sql
CREATE TABLE IF NOT EXISTS sub_admin_commission_grants (
    id BIGSERIAL PRIMARY KEY,
    sub_admin_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    granted_date DATE NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_active_unique
    ON sub_admin_commission_grants (sub_admin_user_id, group_id)
    WHERE enabled = TRUE;

CREATE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_sub_admin_enabled
    ON sub_admin_commission_grants (sub_admin_user_id, enabled, granted_date);

CREATE INDEX IF NOT EXISTS idx_sub_admin_commission_grants_group_enabled
    ON sub_admin_commission_grants (group_id, enabled);

COMMENT ON TABLE sub_admin_commission_grants IS 'Secondary admin financial visibility grants for group commission reporting.';
COMMENT ON COLUMN sub_admin_commission_grants.granted_date IS 'Local natural date from which the assigned group is visible.';
```

**Step 4: Add the Ent schema**

Create `backend/ent/schema/sub_admin_commission_grant.go`:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type SubAdminCommissionGrant struct {
	ent.Schema
}

func (SubAdminCommissionGrant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sub_admin_commission_grants"},
	}
}

func (SubAdminCommissionGrant) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("sub_admin_user_id"),
		field.Int64("group_id"),
		field.Time("granted_date").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Bool("enabled").
			Default(true),
		field.Int64("created_by").
			Optional().
			Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SubAdminCommissionGrant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sub_admin", User.Type).
			Unique().
			Required().
			Field("sub_admin_user_id"),
		edge.To("group", Group.Type).
			Unique().
			Required().
			Field("group_id"),
		edge.To("creator", User.Type).
			Unique().
			Field("created_by"),
	}
}

func (SubAdminCommissionGrant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sub_admin_user_id", "enabled", "granted_date"),
		index.Fields("group_id", "enabled"),
	}
}
```

**Step 5: Generate Ent code**

Run:

```bash
cd backend && go generate ./ent
```

Expected: generated Ent files include `subadmincommissiongrant`.

**Step 6: Run migration test**

Run:

```bash
cd backend && go test ./migrations -run TestSubAdminCommissionGrantsMigration -count=1
```

Expected: PASS.

**Step 7: Commit**

```bash
git add backend/ent/schema/sub_admin_commission_grant.go backend/ent backend/migrations/224_sub_admin_commission_grants.sql backend/migrations/sub_admin_commission_grants_migration_test.go
git commit -m "feat: add sub-admin commission grants schema"
```

---

### Task 2: Add Service Types, Settings Helpers, And Grant Rules

**Files:**

- Create: `backend/internal/service/sub_admin_commission.go`
- Create: `backend/internal/service/sub_admin_commission_test.go`
- Create: `backend/internal/service/setting_service_sub_admin_commission.go`
- Modify: `backend/internal/service/wire.go`

**Step 1: Write service tests**

Create `backend/internal/service/sub_admin_commission_test.go` with tests for:

- `SetSettings` rejects rate below `0`.
- `SetSettings` rejects rate above `1`.
- `ReplaceGrants` rejects a target user whose role is not `sub_admin`.
- `ReplaceGrants` uses `GroupUsageDate(now)` as `granted_date` for new grants.
- `GetDayGroupLogs` rejects an unassigned group.

Use a stub repository and setting repository in the test file.

**Step 2: Run the service tests to verify they fail**

Run:

```bash
cd backend && go test ./internal/service -run TestSubAdminCommission -count=1
```

Expected: FAIL because service types do not exist.

**Step 3: Add service domain types and repository interface**

Create `backend/internal/service/sub_admin_commission.go` with:

```go
package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type SubAdminCommissionGrant struct {
	ID            int64
	SubAdminID    int64
	SubAdminEmail string
	GroupID       int64
	GroupName     string
	GrantedDate   string
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SubAdminCommissionCalendarDay struct {
	Date             string
	Enabled          bool
	ActualCost       float64
	CommissionAmount float64
}

type SubAdminCommissionDayGroup struct {
	GroupID          int64
	GroupName        string
	Requests         int64
	TotalTokens      int64
	ActualCost       float64
	CommissionAmount float64
}

type SubAdminCommissionUsageLog struct {
	ID            int64
	RequestID     string
	CreatedAt     time.Time
	UserID        int64
	UserEmail     string
	APIKeyID      int64
	APIKeyName    string
	GroupID       int64
	GroupName     string
	Model         string
	RequestedModel *string
	InputTokens   int
	OutputTokens  int
	CacheCreationTokens int
	CacheReadTokens     int
	ActualCost    float64
}

type ReplaceSubAdminCommissionGrantsInput struct {
	SubAdminID int64
	GroupIDs   []int64
	OperatorID int64
	Now        time.Time
}

type SubAdminCommissionRepository interface {
	ListAllGrants(ctx context.Context) ([]SubAdminCommissionGrant, error)
	ListGrantsForSubAdmin(ctx context.Context, subAdminID int64) ([]SubAdminCommissionGrant, error)
	ReplaceGrants(ctx context.Context, input ReplaceSubAdminCommissionGrantsInput, grantedDate string) ([]SubAdminCommissionGrant, error)
	ListCalendar(ctx context.Context, subAdminID int64, month string, commissionRate float64, now time.Time) ([]SubAdminCommissionCalendarDay, error)
	ListDayGroups(ctx context.Context, subAdminID int64, date string, commissionRate float64) ([]SubAdminCommissionDayGroup, error)
	ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]SubAdminCommissionUsageLog, pagination.PaginationResult, error)
}

type SubAdminCommissionService struct {
	repo        SubAdminCommissionRepository
	userRepo    UserRepository
	settingSvc  *SettingService
}

func NewSubAdminCommissionService(repo SubAdminCommissionRepository, userRepo UserRepository, settingSvc *SettingService) *SubAdminCommissionService {
	return &SubAdminCommissionService{repo: repo, userRepo: userRepo, settingSvc: settingSvc}
}

var ErrSubAdminCommissionForbidden = infraerrors.Forbidden("SUB_ADMIN_COMMISSION_FORBIDDEN", "group is not assigned to this secondary admin")
```

Implement methods:

- `GetSettings(ctx) (float64, error)`
- `SetSettings(ctx, rate float64) (float64, error)`
- `ListAllGrants(ctx)`
- `ReplaceGrants(ctx, input)`
- `ListWorkbenchGrants(ctx, userID)`
- `ListCalendar(ctx, userID, month, now)`
- `ListDayGroups(ctx, userID, date)`
- `ListDayGroupLogs(ctx, userID, groupID, date, params)`

**Step 4: Add setting helpers**

Create `backend/internal/service/setting_service_sub_admin_commission.go`:

```go
package service

import (
	"context"
	"strconv"
)

const SettingKeySubAdminCommissionRate = "sub_admin_commission_rate"

func (s *SettingService) GetSubAdminCommissionRate(ctx context.Context) float64 {
	if s == nil || s.settingRepo == nil {
		return 0
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeySubAdminCommissionRate)
	if err != nil {
		return 0
	}
	rate, err := strconv.ParseFloat(raw, 64)
	if err != nil || rate < 0 {
		return 0
	}
	return rate
}

func (s *SettingService) SetSubAdminCommissionRate(ctx context.Context, rate float64) error {
	if s == nil || s.settingRepo == nil {
		return ErrSettingNotFound
	}
	return s.settingRepo.Set(ctx, SettingKeySubAdminCommissionRate, strconv.FormatFloat(rate, 'f', -1, 64))
}
```

**Step 5: Register the service in Wire**

Modify `backend/internal/service/wire.go` provider set to include:

```go
NewSubAdminCommissionService,
```

**Step 6: Run service tests**

Run:

```bash
cd backend && go test ./internal/service -run TestSubAdminCommission -count=1
```

Expected: PASS.

**Step 7: Commit**

```bash
git add backend/internal/service/sub_admin_commission.go backend/internal/service/sub_admin_commission_test.go backend/internal/service/setting_service_sub_admin_commission.go backend/internal/service/wire.go
git commit -m "feat: add sub-admin commission service"
```

---

### Task 3: Implement Repository Queries

**Files:**

- Create: `backend/internal/repository/sub_admin_commission_repo.go`
- Create: `backend/internal/repository/sub_admin_commission_repo_test.go`
- Modify: `backend/internal/repository/wire.go`

**Step 1: Write repository tests**

Create sqlmock tests for:

- `ListCalendar` joins only enabled grants for the current sub-admin.
- `ListDayGroups` rejects dates before `granted_date` by returning no rows.
- `ListDayGroupLogs` applies pagination and joins `users`, `api_keys`, and `groups`.

Run:

```bash
cd backend && go test ./internal/repository -run TestSubAdminCommissionRepository -count=1
```

Expected: FAIL.

**Step 2: Implement repository**

Create `backend/internal/repository/sub_admin_commission_repo.go`.

Constructor:

```go
func NewSubAdminCommissionRepository(client *ent.Client, db *sql.DB) service.SubAdminCommissionRepository {
	return &subAdminCommissionRepository{client: client, db: db}
}
```

Calendar query should:

- Read `service.GroupUsageTimezoneName()`.
- Generate month days.
- Join enabled grants for the current sub-admin.
- Use `usage_group_daily_rollups` for closed historical days.
- Use `usage_logs` for today/unclosed tail.
- Return zero-cost enabled days when assigned groups had no usage.

Day group summary query should use direct `usage_logs` for the selected day because the drilldown requires requests and token totals, not only cost.

Log detail query should:

- Check the grant in the SQL join.
- Filter by local date window from `service.ParseGroupUsageDate(date)`.
- Order by `usage_logs.created_at DESC`.
- Use `pagination.PaginationParams`.

**Step 3: Register repository in Wire**

Modify `backend/internal/repository/wire.go` provider set:

```go
NewSubAdminCommissionRepository,
```

**Step 4: Run repository tests**

Run:

```bash
cd backend && go test ./internal/repository -run TestSubAdminCommissionRepository -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/repository/sub_admin_commission_repo.go backend/internal/repository/sub_admin_commission_repo_test.go backend/internal/repository/wire.go
git commit -m "feat: add sub-admin commission repository"
```

---

### Task 4: Add HTTP Handler, Routes, And API Contract Tests

**Files:**

- Create: `backend/internal/handler/admin/sub_admin_commission_handler.go`
- Create: `backend/internal/handler/admin/sub_admin_commission_handler_test.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/server/api_contract_test.go`
- Generated: `backend/cmd/server/wire_gen.go`

**Step 1: Write handler tests**

Test these cases:

- `PUT /settings` rejects `commission_rate < 0` and `commission_rate > 1`.
- `PUT /grants/:sub_admin_id` calls service with selected group IDs.
- Workbench `GET /calendar` derives user ID from auth subject.
- Workbench logs endpoint returns `403` when service returns `ErrSubAdminCommissionForbidden`.

Run:

```bash
cd backend && go test ./internal/handler/admin -run TestSubAdminCommissionHandler -count=1
```

Expected: FAIL.

**Step 2: Add handler**

Create `backend/internal/handler/admin/sub_admin_commission_handler.go` with methods:

- `GetSettings`
- `UpdateSettings`
- `ListGrants`
- `ReplaceGrants`
- `GetWorkbenchGrants`
- `GetWorkbenchCalendar`
- `GetWorkbenchDayGroups`
- `GetWorkbenchDayGroupLogs`

Use `middleware.GetAuthSubjectFromContext(c)` for workbench endpoints.

**Step 3: Register handler in aggregate structs**

Modify `backend/internal/handler/handler.go`:

```go
SubAdminCommission *admin.SubAdminCommissionHandler
```

Modify `backend/internal/handler/wire.go`:

- Add `subAdminCommissionHandler *admin.SubAdminCommissionHandler` to `ProvideAdminHandlers` and `ProvideAdminHandlersWithSystemCustomGroup`.
- Assign it in the returned `AdminHandlers`.

**Step 4: Register routes**

Modify `backend/internal/server/routes/admin.go`.

In admin route registration:

```go
if adminHandlers.SubAdminCommission != nil {
	registerSubAdminCommissionRoutes(admin, h)
}
```

Add:

```go
func registerSubAdminCommissionRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	commissions := admin.Group("/sub-admin-commissions")
	{
		commissions.GET("/settings", h.Admin.SubAdminCommission.GetSettings)
		commissions.PUT("/settings", h.Admin.SubAdminCommission.UpdateSettings)
		commissions.GET("/grants", h.Admin.SubAdminCommission.ListGrants)
		commissions.PUT("/grants/:sub_admin_id", h.Admin.SubAdminCommission.ReplaceGrants)
	}
}
```

Update `registerAdminWorkbenchRoutes` so it registers commission routes when `h.Admin.SubAdminCommission != nil`, not only when redeem exists:

```go
commission := workbench.Group("/commission")
{
	commission.GET("/grants", h.Admin.SubAdminCommission.GetWorkbenchGrants)
	commission.GET("/calendar", h.Admin.SubAdminCommission.GetWorkbenchCalendar)
	commission.GET("/days/:date/groups", h.Admin.SubAdminCommission.GetWorkbenchDayGroups)
	commission.GET("/days/:date/groups/:group_id/logs", h.Admin.SubAdminCommission.GetWorkbenchDayGroupLogs)
}
```

Also change the top-level workbench route guard condition from `h.Redeem != nil` to allow either redeem or commission handlers.

**Step 5: Update API contract tests**

Modify `backend/internal/server/api_contract_test.go` route lists to include the four workbench commission routes and four admin commission routes.

Run:

```bash
cd backend && go test ./internal/server -run TestAdminWorkbenchAPIContractRoutes -count=1
```

Expected: PASS after implementation.

**Step 6: Regenerate Wire**

Run:

```bash
cd backend && go generate ./cmd/server
```

Expected: `backend/cmd/server/wire_gen.go` updates cleanly.

**Step 7: Run handler and contract tests**

Run:

```bash
cd backend && go test ./internal/handler/admin -run TestSubAdminCommissionHandler -count=1
cd backend && go test ./internal/server -run 'TestAdminWorkbenchAPIContractRoutes|TestAdminAPIContractRoutes' -count=1
```

Expected: PASS.

**Step 8: Commit**

```bash
git add backend/internal/handler/admin/sub_admin_commission_handler.go backend/internal/handler/admin/sub_admin_commission_handler_test.go backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/server/routes/admin.go backend/internal/server/api_contract_test.go backend/cmd/server/wire_gen.go
git commit -m "feat: add sub-admin commission api"
```

---

### Task 5: Add Frontend API Client And Tests

**Files:**

- Create: `frontend/src/api/admin/subAdminCommission.ts`
- Create: `frontend/src/api/admin/__tests__/subAdminCommission.spec.ts`
- Modify: `frontend/src/api/admin/index.ts`

**Step 1: Write frontend API tests**

Create `frontend/src/api/admin/__tests__/subAdminCommission.spec.ts` to assert endpoint paths:

```ts
import { describe, expect, it, vi } from 'vitest'
import subAdminCommissionAPI from '../subAdminCommission'
import { apiClient } from '../../client'

vi.mock('../../client', () => ({
  apiClient: {
    get: vi.fn(),
    put: vi.fn()
  }
}))

describe('subAdminCommissionAPI', () => {
  it('updates settings', async () => {
    vi.mocked(apiClient.put).mockResolvedValueOnce({ data: { commission_rate: 0.08 } })
    await subAdminCommissionAPI.updateSettings({ commission_rate: 0.08 })
    expect(apiClient.put).toHaveBeenCalledWith('/admin/sub-admin-commissions/settings', { commission_rate: 0.08 })
  })

  it('loads workbench calendar', async () => {
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: [] })
    await subAdminCommissionAPI.getWorkbenchCalendar({ month: '2026-08' })
    expect(apiClient.get).toHaveBeenCalledWith('/admin/workbench/commission/calendar', { params: { month: '2026-08' } })
  })
})
```

Run:

```bash
cd frontend && pnpm test:run src/api/admin/__tests__/subAdminCommission.spec.ts
```

Expected: FAIL.

**Step 2: Add API module**

Create `frontend/src/api/admin/subAdminCommission.ts` with TypeScript interfaces matching the backend response shapes and functions:

- `getSettings`
- `updateSettings`
- `listGrants`
- `replaceGrants`
- `getWorkbenchGrants`
- `getWorkbenchCalendar`
- `getWorkbenchDayGroups`
- `getWorkbenchDayGroupLogs`

**Step 3: Export through admin barrel**

Modify `frontend/src/api/admin/index.ts`:

```ts
import subAdminCommissionAPI from './subAdminCommission'
```

Add `subAdminCommission: subAdminCommissionAPI` to `adminAPI` and named export list.

**Step 4: Run frontend API test**

Run:

```bash
cd frontend && pnpm test:run src/api/admin/__tests__/subAdminCommission.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/subAdminCommission.ts frontend/src/api/admin/__tests__/subAdminCommission.spec.ts frontend/src/api/admin/index.ts
git commit -m "feat: add sub-admin commission frontend api"
```

---

### Task 6: Add Workbench Commission UI Components

**Files:**

- Create: `frontend/src/components/admin/workbench/SubAdminCommissionPanel.vue`
- Create: `frontend/src/components/admin/workbench/SubAdminCommissionManagement.vue`
- Create: `frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue`
- Create: `frontend/src/components/admin/workbench/SubAdminCommissionDayDrawer.vue`
- Modify: `frontend/src/views/admin/AdminWorkbenchView.vue`
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/common.ts`
- Modify: `frontend/src/i18n/locales/en/common.ts`

**Step 1: Write component-level tests through AdminWorkbenchView**

Extend `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts` mocks:

- Add `adminAPI.subAdminCommission`.
- For role `admin`, assert the management section renders and calls settings/grants APIs.
- For role `sub_admin`, assert the calendar renders and calls workbench APIs.
- Assert clicking a day loads group summary.
- Assert expanding a group loads paginated logs.

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts
```

Expected: FAIL.

**Step 2: Add the panel shell**

Create `SubAdminCommissionPanel.vue`.

Behavior:

- If `authStore.user?.role === 'admin'`, render management and calendar tabs or stacked sections.
- If `authStore.user?.role === 'sub_admin'`, render only the calendar.
- Preserve the existing balance redeem-code section below or above this panel without removing any existing functionality.

**Step 3: Add management component**

Create `SubAdminCommissionManagement.vue`.

Behavior:

- Load global rate.
- Load sub-admin users with `adminAPI.users.list(1, 50, { role: 'sub_admin' })`.
- Load groups with `adminAPI.groups.getAll()`.
- Select a sub-admin and check assigned groups.
- Save via `adminAPI.subAdminCommission.replaceGrants(subAdminId, { group_ids })`.
- Save global rate via `updateSettings`.

**Step 4: Add calendar component**

Create `SubAdminCommissionCalendar.vue`.

Behavior:

- Current month defaults to today's local `YYYY-MM`.
- Calls `getWorkbenchCalendar({ month })`.
- Renders a 7-column calendar grid with stable cell heights.
- Shows `actual_cost` and `commission_amount`.
- Disables future days and non-enabled days.
- Emits `select-day` for enabled days.

**Step 5: Add day drawer**

Create `SubAdminCommissionDayDrawer.vue`.

Behavior:

- Loads group summary for selected date.
- Shows group rows with requests, tokens, actual cost, commission amount.
- Expands one group at a time.
- Loads logs with pagination.

**Step 6: Wire panel into workbench**

Modify `frontend/src/views/admin/AdminWorkbenchView.vue`:

```ts
import SubAdminCommissionPanel from '@/components/admin/workbench/SubAdminCommissionPanel.vue'
```

Place the panel near the top of the page before the balance redeem-code section.

**Step 7: Add i18n keys**

Add `adminWorkbench.commission` keys in both `zh/common.ts` and `en/common.ts`.

Minimum keys:

- `title`
- `subtitle`
- `settings`
- `commissionRate`
- `saveSettings`
- `selectSubAdmin`
- `assignedGroups`
- `saveGrants`
- `calendar`
- `actualCost`
- `commissionAmount`
- `monthTotal`
- `emptyGrants`
- `dayDetails`
- `requestLogs`
- `loadFailed`
- `saveSuccess`
- `saveFailed`

**Step 8: Run frontend tests**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts
cd frontend && pnpm typecheck
cd frontend && pnpm lint:check
```

Expected: PASS.

**Step 9: Commit**

```bash
git add frontend/src/components/admin/workbench/SubAdminCommissionPanel.vue frontend/src/components/admin/workbench/SubAdminCommissionManagement.vue frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue frontend/src/components/admin/workbench/SubAdminCommissionDayDrawer.vue frontend/src/views/admin/AdminWorkbenchView.vue frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts frontend/src/i18n/locales/zh/common.ts frontend/src/i18n/locales/en/common.ts
git commit -m "feat: add sub-admin commission workbench ui"
```

---

### Task 7: Full Verification

**Files:**

- No new files unless fixes are required.

**Step 1: Run backend targeted tests**

Run:

```bash
cd backend && go test ./migrations -run TestSubAdminCommissionGrantsMigration -count=1
cd backend && go test ./internal/service -run TestSubAdminCommission -count=1
cd backend && go test ./internal/repository -run TestSubAdminCommissionRepository -count=1
cd backend && go test ./internal/handler/admin -run TestSubAdminCommissionHandler -count=1
cd backend && go test ./internal/server -run 'TestAdminWorkbenchAPIContractRoutes|TestAdminAPIContractRoutes' -count=1
```

Expected: all PASS.

**Step 2: Run frontend targeted tests**

Run:

```bash
cd frontend && pnpm test:run src/api/admin/__tests__/subAdminCommission.spec.ts src/views/admin/__tests__/AdminWorkbenchView.spec.ts
cd frontend && pnpm typecheck
cd frontend && pnpm lint:check
```

Expected: all PASS.

**Step 3: Run generation checks**

Run:

```bash
cd backend && go generate ./ent
cd backend && go generate ./cmd/server
git diff --exit-code backend/ent backend/cmd/server/wire_gen.go
```

Expected: no diff after generated files are committed.

**Step 4: Manual local check**

Use the already-running local app at:

```text
http://localhost:3000/admin/workbench
```

Check:

- Admin sees commission settings and grant management.
- Sub-admin sees only assigned group calendar.
- Calendar starts at assigned date and stops at today.
- Clicking a day shows group summary.
- Expanding a group shows request logs.
- Existing balance redeem-code workflow still works.

**Step 5: Final commit if fixes were needed**

```bash
git status --short
git add <changed-files>
git commit -m "test: verify sub-admin commission flow"
```

Do not deploy.
