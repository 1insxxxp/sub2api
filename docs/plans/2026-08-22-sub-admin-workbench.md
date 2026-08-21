# Sub Admin Workbench Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `sub_admin` role with access to a single "管理员页面" workbench, and move balance-to-redeem-code generation out of the user redeem page into that workbench.

**Architecture:** Keep full admin privileges restricted to `admin`. Add a narrower workbench privilege shared by `admin` and `sub_admin`, with separate frontend route meta and separate backend route middleware. Reuse the existing balance-transfer redeem service, but change its permission gate from the old per-user flag to the new role-based workbench privilege.

**Tech Stack:** Go/Gin backend, Ent user model, Vue 3 + Pinia + Vue Router frontend, Vitest frontend tests, Go unit/API contract tests.

---

## Task 0: Clear Current Git State Before Code Changes

**Files:**
- Inspect only: repository state

**Step 1: Check active Git operation**

Run:

```bash
git status --short --branch
git status
```

Expected: identify the currently active cherry-pick and the existing staged balance-transfer changes.

**Step 2: Check for conflict markers**

Run:

```bash
rg -n '^(<<<<<<<|=======|>>>>>>>)' backend frontend docs
```

Expected: no output before implementation starts. If markers exist, resolve the active cherry-pick first. Do not start this feature on top of unresolved markers.

**Step 3: Decide state transition with owner**

If the cherry-pick belongs to the existing redeem action work, finish it with:

```bash
git commit
```

or continue it with the command Git suggests. If it is stale or accidental, ask before aborting. Do not run `git cherry-pick --abort` without explicit approval.

---

## Task 1: Add the `sub_admin` Role in Backend Domain and Service

**Files:**
- Modify: `backend/internal/domain/constants.go`
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/user.go`
- Modify: `backend/internal/service/admin_user.go`
- Test: `backend/internal/service/admin_service_role_test.go`

**Step 1: Write failing service role tests**

Add tests covering:

```go
func TestAdminService_CreateUser_SubAdminRoleAccepted(t *testing.T) {
    svc, repo := newAdminServiceWithUserRepoStub()
    user, err := svc.CreateUser(context.Background(), &CreateUserInput{
        Email: "subadmin@example.com",
        Password: "password123",
        Role: RoleSubAdmin,
    })
    require.NoError(t, err)
    require.Equal(t, RoleSubAdmin, user.Role)
    require.Equal(t, RoleSubAdmin, repo.created[0].Role)
}

func TestAdminService_UpdateUser_SubAdminRoleAccepted(t *testing.T) {
    base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser}}
    svc := newAdminServiceWithUserRepo(base)
    updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{Role: RoleSubAdmin})
    require.NoError(t, err)
    require.Equal(t, RoleSubAdmin, updated.Role)
}
```

Adapt helper names to the actual test helpers in `admin_service_role_test.go`.

**Step 2: Run tests and verify failure**

Run:

```bash
cd backend && go test ./internal/service -run 'TestAdminService_.*SubAdminRole|TestAdminService_.*InvalidRole' -count=1
```

Expected: FAIL because `RoleSubAdmin` is undefined or rejected by `normalizeUserRole`.

**Step 3: Implement role constants and helpers**

In `backend/internal/domain/constants.go`:

```go
const (
    RoleAdmin    = "admin"
    RoleSubAdmin = "sub_admin"
    RoleUser     = "user"
)
```

Mirror it in `backend/internal/service/domain_constants.go`.

In `backend/internal/service/user.go`:

```go
func (u *User) IsAdmin() bool {
    return u.Role == RoleAdmin
}

func (u *User) IsSubAdmin() bool {
    return u.Role == RoleSubAdmin
}

func (u *User) CanAccessAdminWorkbench() bool {
    return u.IsAdmin() || u.IsSubAdmin()
}
```

In `backend/internal/service/admin_user.go`, update `normalizeUserRole` to accept `RoleSubAdmin` and update its error message.

**Step 4: Run tests and verify pass**

Run:

```bash
cd backend && go test ./internal/service -run 'TestAdminService_.*Role' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/domain/constants.go backend/internal/service/domain_constants.go backend/internal/service/user.go backend/internal/service/admin_user.go backend/internal/service/admin_service_role_test.go
git commit -m "feat: add sub admin role"
```

---

## Task 2: Add Backend Workbench Authentication

**Files:**
- Modify: `backend/internal/server/middleware/admin_auth.go`
- Modify: `backend/internal/server/middleware/wire.go`
- Test: `backend/internal/server/middleware/admin_auth_test.go`

**Step 1: Write failing middleware tests**

Add tests for the new workbench middleware:

```go
func TestAdminWorkbenchAuth_AllowsSubAdminJWT(t *testing.T) {
    // Arrange user with RoleSubAdmin and StatusActive.
    // Call route protected by NewAdminWorkbenchAuthMiddleware.
    // Expect HTTP 200.
}

func TestAdminWorkbenchAuth_RejectsUserJWT(t *testing.T) {
    // Arrange user with RoleUser and StatusActive.
    // Call route protected by NewAdminWorkbenchAuthMiddleware.
    // Expect HTTP 403.
}

func TestAdminAuth_StillRejectsSubAdminForFullAdmin(t *testing.T) {
    // Arrange user with RoleSubAdmin.
    // Call route protected by NewAdminAuthMiddleware.
    // Expect HTTP 403.
}
```

Use the existing test stubs in `admin_auth_test.go` for auth service and user service.

**Step 2: Run tests and verify failure**

Run:

```bash
cd backend && go test ./internal/server/middleware -run 'Admin.*Auth' -count=1
```

Expected: FAIL because the workbench middleware does not exist.

**Step 3: Implement middleware**

Add a new middleware constructor near `NewAdminAuthMiddleware`:

```go
func NewAdminWorkbenchAuthMiddleware(
    authService *service.AuthService,
    userService *service.UserService,
    settingService *service.SettingService,
    auditService *service.AuditLogService,
) AdminAuthMiddleware {
    return func(c *gin.Context) {
        // Reuse the same token extraction behavior as NewAdminAuthMiddleware.
        // Admin API key remains accepted as an admin identity.
        // JWT validation must call user.CanAccessAdminWorkbench().
    }
}
```

Keep `NewAdminAuthMiddleware` strict: it must still require `user.IsAdmin()`.

If the existing code has shared private helpers for token extraction, reuse them. Otherwise extract a private helper to avoid duplicating session binding and token-version checks.

**Step 4: Run tests and verify pass**

Run:

```bash
cd backend && go test ./internal/server/middleware -run 'Admin.*Auth' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/server/middleware/admin_auth.go backend/internal/server/middleware/wire.go backend/internal/server/middleware/admin_auth_test.go
git commit -m "feat: add admin workbench auth"
```

---

## Task 3: Move Balance Transfer Redeem Endpoints to Workbench Routes

**Files:**
- Modify: `backend/internal/server/router.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/handler/redeem_handler.go`
- Modify: `backend/internal/service/redeem_service.go`
- Test: `backend/internal/server/api_contract_test.go`
- Test: `backend/internal/service/redeem_service_balance_transfer_test.go`

**Step 1: Write failing route/API tests**

Add API contract coverage:

```go
// sub_admin can POST /api/v1/admin/workbench/redeem/generated
// user gets 403 for POST /api/v1/admin/workbench/redeem/generated
// user gets 404 or 403 for old POST /api/v1/redeem/generate
// admin can GET /api/v1/admin/workbench/redeem/generated?page=1&page_size=10
```

Add service tests:

```go
func TestRedeemService_GenerateBalanceTransferCodes_AllowsSubAdmin(t *testing.T) {
    repo := &userRepoStub{user: &User{
        ID: 7,
        Status: StatusActive,
        Role: RoleSubAdmin,
        Balance: 50,
    }}
    svc := newRedeemServiceForBalanceTransfer(repo, redeemRepo)
    codes, err := svc.GenerateBalanceTransferCodes(context.Background(), 7, GenerateBalanceTransferCodeInput{
        Amount: 10,
        Count: 1,
    })
    require.NoError(t, err)
    require.Len(t, codes, 1)
}

func TestRedeemService_GenerateBalanceTransferCodes_RejectsRegularUserEvenIfOldFlagEnabled(t *testing.T) {
    repo := &userRepoStub{user: &User{
        ID: 7,
        Status: StatusActive,
        Role: RoleUser,
        Balance: 50,
        BalanceRedeemCodeEnabled: true,
    }}
    svc := newRedeemServiceForBalanceTransfer(repo, redeemRepo)
    _, err := svc.GenerateBalanceTransferCodes(context.Background(), 7, GenerateBalanceTransferCodeInput{
        Amount: 10,
        Count: 1,
    })
    require.ErrorIs(t, err, ErrBalanceTransferRedeemNotAllowed)
}
```

Adapt helper names to existing test helpers.

**Step 2: Run tests and verify failure**

Run:

```bash
cd backend && go test ./internal/service -run 'TestRedeemService_GenerateBalanceTransferCodes' -count=1
cd backend && go test ./internal/server -run 'TestAPIContract.*Redeem|TestAPIContract.*Workbench' -count=1
```

Expected: FAIL because old permission still uses `balance_redeem_code_enabled` and workbench routes do not exist.

**Step 3: Register workbench routes**

Update `RegisterAdminRoutes` or router wiring so `/api/v1/admin/workbench` uses the new workbench auth middleware instead of full admin auth.

Add a route function:

```go
func registerAdminWorkbenchRoutes(workbench *gin.RouterGroup, h *handler.Handlers) {
    redeem := workbench.Group("/redeem")
    {
        redeem.POST("/generated", h.Redeem.GenerateBalanceTransferCode)
        redeem.GET("/generated", h.Redeem.GetGenerated)
        redeem.DELETE("/generated/:id", h.Redeem.DeleteGenerated)
        redeem.POST("/generated/batch-delete", h.Redeem.DeleteGeneratedBatch)
    }
}
```

Remove these endpoints from user routes in `backend/internal/server/routes/user.go`:

```go
redeem.POST("/generate", h.Redeem.GenerateBalanceTransferCode)
redeem.GET("/generated", h.Redeem.GetGenerated)
redeem.DELETE("/generated/:id", h.Redeem.DeleteGenerated)
redeem.POST("/generated/batch-delete", h.Redeem.DeleteGeneratedBatch)
```

Keep normal redeem endpoints:

```go
redeem.POST("", h.Redeem.Redeem)
redeem.GET("/history", h.Redeem.GetHistory)
```

**Step 4: Change service permission gate**

In `backend/internal/service/redeem_service.go`, replace:

```go
if !user.BalanceRedeemCodeEnabled {
    return nil, ErrBalanceTransferRedeemNotAllowed
}
```

with:

```go
if !user.CanAccessAdminWorkbench() {
    return nil, ErrBalanceTransferRedeemNotAllowed
}
```

**Step 5: Run backend tests**

Run:

```bash
cd backend && go test ./internal/service -run 'TestRedeemService_GenerateBalanceTransferCodes|TestRedeemService_DeleteGenerated' -count=1
cd backend && go test ./internal/server -run 'TestAPIContract' -count=1
```

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/internal/server/router.go backend/internal/server/routes/admin.go backend/internal/server/routes/user.go backend/internal/handler/redeem_handler.go backend/internal/service/redeem_service.go backend/internal/server/api_contract_test.go backend/internal/service/redeem_service_balance_transfer_test.go
git commit -m "feat: move balance redeem codes to workbench"
```

---

## Task 4: Add Frontend Role and Route Guards

**Files:**
- Modify: `frontend/src/stores/auth.ts`
- Modify: `frontend/src/router/meta.d.ts`
- Modify: `frontend/src/router/index.ts`
- Test: `frontend/src/router/__tests__/guards.spec.ts`
- Test: `frontend/src/router/__tests__/feature-access.spec.ts`

**Step 1: Write failing route guard tests**

Add cases:

```ts
it('allows sub_admin to access /admin/workbench', () => {
  authState.isAuthenticated = true
  authState.user = { role: 'sub_admin' }
  const redirect = simulateGuard('/admin/workbench', { requiresAdminWorkbench: true }, authState)
  expect(redirect).toBeUndefined()
})

it('redirects sub_admin away from full admin routes', () => {
  authState.isAuthenticated = true
  authState.user = { role: 'sub_admin' }
  const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, authState)
  expect(redirect).toBe('/dashboard')
})

it('rejects user from /admin/workbench', () => {
  authState.isAuthenticated = true
  authState.user = { role: 'user' }
  const redirect = simulateGuard('/admin/workbench', { requiresAdminWorkbench: true }, authState)
  expect(redirect).toBe('/dashboard')
})
```

Adapt to the actual guard test helper shape.

**Step 2: Run tests and verify failure**

Run:

```bash
cd frontend && pnpm test:run src/router/__tests__/guards.spec.ts src/router/__tests__/feature-access.spec.ts
```

Expected: FAIL because `requiresAdminWorkbench` and sub-admin computed state do not exist.

**Step 3: Implement auth computed values**

In `frontend/src/stores/auth.ts`:

```ts
const isAdmin = computed(() => user.value?.role === 'admin')
const isSubAdmin = computed(() => user.value?.role === 'sub_admin')
const canAccessAdminWorkbench = computed(() => isAdmin.value || isSubAdmin.value)
```

Return the new computed values from the store.

**Step 4: Add route meta and guard behavior**

In `frontend/src/router/meta.d.ts`:

```ts
requiresAdminWorkbench?: boolean
```

In `frontend/src/router/index.ts`, add the route:

```ts
{
  path: '/admin/workbench',
  name: 'AdminWorkbench',
  component: () => import('@/views/admin/AdminWorkbenchView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdminWorkbench: true,
    title: 'Admin Workbench',
    titleKey: 'admin.workbench.title',
    descriptionKey: 'admin.workbench.description'
  }
}
```

Guard order:

```ts
const requiresAdmin = to.meta.requiresAdmin === true
const requiresAdminWorkbench = to.meta.requiresAdminWorkbench === true

if (requiresAdmin && !authStore.isAdmin) {
  next('/dashboard')
  return
}

if (requiresAdminWorkbench && !authStore.canAccessAdminWorkbench) {
  next('/dashboard')
  return
}
```

For `/admin` redirect, prefer:

```ts
redirect: () => authStore.isAdmin ? '/admin/dashboard' : '/admin/workbench'
```

or keep a static redirect and add guard fallback. Ensure sub-admin does not land on `/admin/dashboard`.

**Step 5: Run tests and verify pass**

Run:

```bash
cd frontend && pnpm test:run src/router/__tests__/guards.spec.ts src/router/__tests__/feature-access.spec.ts
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/stores/auth.ts frontend/src/router/meta.d.ts frontend/src/router/index.ts frontend/src/router/__tests__/guards.spec.ts frontend/src/router/__tests__/feature-access.spec.ts
git commit -m "feat: add sub admin route guard"
```

---

## Task 5: Add Workbench API Client

**Files:**
- Create: `frontend/src/api/admin/workbench.ts`
- Modify: `frontend/src/api/index.ts`
- Modify: `frontend/src/api/redeem.ts`
- Test: `frontend/src/api/__tests__/admin.workbench.spec.ts`
- Test: `frontend/src/api/__tests__/redeem.spec.ts`

**Step 1: Write failing API tests**

Create `frontend/src/api/__tests__/admin.workbench.spec.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'
import { adminWorkbenchAPI } from '@/api/admin/workbench'
import { apiClient } from '@/api/client'

vi.mock('@/api/client', () => ({
  apiClient: {
    post: vi.fn(),
    get: vi.fn(),
    delete: vi.fn()
  }
}))

describe('adminWorkbenchAPI', () => {
  it('generates balance transfer codes through workbench endpoint', async () => {
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: [{ id: 1, code: 'abc' }] })
    await adminWorkbenchAPI.generateBalanceTransferCodes({ amount: 10, count: 1 })
    expect(apiClient.post).toHaveBeenCalledWith('/admin/workbench/redeem/generated', { amount: 10, count: 1 })
  })

  it('lists generated codes through workbench endpoint', async () => {
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: { items: [], total: 0, page: 1, page_size: 10 } })
    await adminWorkbenchAPI.getGenerated({ page: 1, page_size: 10 })
    expect(apiClient.get).toHaveBeenCalledWith('/admin/workbench/redeem/generated', { params: { page: 1, page_size: 10 } })
  })
})
```

Update `redeem.spec.ts` to remove expectations for `/redeem/generate` and `/redeem/generated` if those functions are removed from `redeem.ts`.

**Step 2: Run tests and verify failure**

Run:

```bash
cd frontend && pnpm test:run src/api/__tests__/admin.workbench.spec.ts src/api/__tests__/redeem.spec.ts
```

Expected: FAIL because the new API module does not exist.

**Step 3: Implement API client**

Create `frontend/src/api/admin/workbench.ts`:

```ts
import { apiClient } from '../client'
import type { PaginatedResponse, RedeemCode } from '@/types'

export interface GenerateBalanceTransferCodeRequest {
  amount: number
  count?: number
  expires_in_days?: number
  notes?: string
  single_use_per_user?: boolean
}

export type GeneratedRedeemCode = RedeemCode

export interface RedeemListParams {
  page: number
  page_size: number
}

export async function generateBalanceTransferCodes(payload: GenerateBalanceTransferCodeRequest): Promise<GeneratedRedeemCode[]> {
  const { data } = await apiClient.post<GeneratedRedeemCode[] | GeneratedRedeemCode>(
    '/admin/workbench/redeem/generated',
    payload
  )
  return Array.isArray(data) ? data : [data]
}

export async function getGenerated(params: RedeemListParams): Promise<PaginatedResponse<GeneratedRedeemCode>> {
  const { data } = await apiClient.get<PaginatedResponse<GeneratedRedeemCode>>('/admin/workbench/redeem/generated', { params })
  return data
}

export async function deleteGenerated(id: number): Promise<GeneratedRedeemCode> {
  const { data } = await apiClient.delete<GeneratedRedeemCode>(`/admin/workbench/redeem/generated/${id}`)
  return data
}

export async function deleteGeneratedBatch(ids: number[]): Promise<GeneratedRedeemCode[]> {
  const { data } = await apiClient.post<GeneratedRedeemCode[]>('/admin/workbench/redeem/generated/batch-delete', { ids })
  return data
}

export const adminWorkbenchAPI = {
  generateBalanceTransferCodes,
  getGenerated,
  deleteGenerated,
  deleteGeneratedBatch
}
```

Remove balance-transfer generation/list/delete functions from `frontend/src/api/redeem.ts`, leaving normal redeem and history calls.

**Step 4: Run tests and verify pass**

Run:

```bash
cd frontend && pnpm test:run src/api/__tests__/admin.workbench.spec.ts src/api/__tests__/redeem.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/workbench.ts frontend/src/api/index.ts frontend/src/api/redeem.ts frontend/src/api/__tests__/admin.workbench.spec.ts frontend/src/api/__tests__/redeem.spec.ts
git commit -m "feat: add workbench redeem API client"
```

---

## Task 6: Build the Admin Workbench Page

**Files:**
- Create: `frontend/src/views/admin/AdminWorkbenchView.vue`
- Create: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/overview.ts`
- Modify: `frontend/src/i18n/locales/en/admin/overview.ts`

**Step 1: Write failing page tests**

Create tests covering:

```ts
it('renders balance redeem code form', () => {
  // mount AdminWorkbenchView with auth user balance
  // expect title "管理员页面"
  // expect balance transfer form fields
})

it('submits balance redeem code generation', async () => {
  // fill amount/count/expiry/notes
  // trigger submit
  // expect adminWorkbenchAPI.generateBalanceTransferCodes called
})

it('loads generated codes with pagination', async () => {
  // mock adminWorkbenchAPI.getGenerated
  // expect list rendered
})
```

Use existing test style from `frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`.

**Step 2: Run tests and verify failure**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts
```

Expected: FAIL because the page does not exist.

**Step 3: Implement page**

Create `AdminWorkbenchView.vue` by moving the balance-transfer UI and logic from `frontend/src/views/user/RedeemView.vue`, but import `adminWorkbenchAPI` instead of `redeemAPI`.

Use a page structure like:

```vue
<template>
  <div class="admin-workspace">
    <header class="admin-panel-header">
      <div>
        <h1>{{ t('admin.workbench.title') }}</h1>
        <p>{{ t('admin.workbench.description') }}</p>
      </div>
    </header>

    <section class="admin-surface">
      <!-- balance transfer form -->
    </section>

    <section class="admin-surface">
      <!-- generated code list with pagination -->
    </section>
  </div>
</template>
```

Keep the page quiet and operational. Do not make it a landing page.

**Step 4: Run tests and verify pass**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/AdminWorkbenchView.vue frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts frontend/src/i18n/locales/zh/admin/overview.ts frontend/src/i18n/locales/en/admin/overview.ts
git commit -m "feat: add admin workbench page"
```

---

## Task 7: Remove User Redeem Page Balance Transfer UI

**Files:**
- Modify: `frontend/src/views/user/RedeemView.vue`
- Modify: `frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`

**Step 1: Write/update failing tests**

Update `RedeemView.balanceTransfer.spec.ts` so the expected behavior is:

```ts
it('does not render balance transfer panel for any user', () => {
  // auth user can be user/admin/sub_admin; /redeem should not show data-test="balance-transfer-panel"
})
```

Delete tests that expect generation from the user redeem page.

**Step 2: Run tests and verify failure**

Run:

```bash
cd frontend && pnpm test:run src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts
```

Expected: FAIL until the UI is removed.

**Step 3: Remove user-side generation UI and logic**

In `frontend/src/views/user/RedeemView.vue`, remove:

- `data-test="balance-transfer-panel"` section
- `canGenerateBalanceTransferCodes`
- generated-code refs, watchers, pagination, delete handlers
- generation form submit handler
- imports used only by balance-transfer generation

Keep:

- normal redeem form
- redemption history with pagination

Remove now-unused i18n strings from dashboard locale files only if no other page needs them. If the workbench page reuses similar wording, move those strings to `admin.workbench`.

**Step 4: Run tests and verify pass**

Run:

```bash
cd frontend && pnpm test:run src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/user/RedeemView.vue frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts frontend/src/i18n/locales/zh/dashboard.ts frontend/src/i18n/locales/en/dashboard.ts
git commit -m "refactor: remove user balance redeem generator"
```

---

## Task 8: Update Navigation, User Forms, and Role Labels

**Files:**
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/components/layout/AppLayout.vue`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/user/profile/ProfileInfoCard.vue`
- Modify: `frontend/src/components/admin/user/UserCreateModal.vue`
- Modify: `frontend/src/components/admin/user/UserEditModal.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/overview.ts`
- Modify: `frontend/src/i18n/locales/en/admin/overview.ts`
- Modify: `frontend/src/types/index.ts`
- Test: `frontend/src/views/admin/__tests__/UsersView.spec.ts`
- Test: add or update sidebar/layout tests if present

**Step 1: Write failing UI tests**

Add tests covering:

```ts
it('shows workbench nav for sub_admin and hides full admin nav', () => {
  // authStore.user.role = 'sub_admin'
  // expect "管理员页面"
  // expect no "用户管理", "分组管理", "账号管理"
})

it('allows admin user form to select sub admin role', () => {
  // mount UserCreateModal or UserEditModal
  // expect option value "sub_admin" text "二级管理员"
})
```

**Step 2: Run tests and verify failure**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/UsersView.spec.ts
```

Expected: FAIL until labels/options/nav are implemented.

**Step 3: Implement navigation behavior**

In sidebar:

- Full admin nav remains behind `authStore.isAdmin`.
- Workbench nav appears when `authStore.canAccessAdminWorkbench`.
- `sub_admin` should still see normal user-side navigation.

Example item:

```ts
{ path: '/admin/workbench', label: t('nav.adminWorkbench'), icon: DashboardIcon }
```

Add i18n:

```ts
adminWorkbench: '管理员页面'
```

**Step 4: Add role labels and form option**

Add role option:

```vue
<option value="sub_admin">{{ t('admin.users.roles.sub_admin') }}</option>
```

Add locale:

```ts
roles: {
  admin: '管理员',
  sub_admin: '二级管理员',
  user: '用户'
}
```

Update TypeScript user role unions if they currently only allow `'admin' | 'user'`.

**Step 5: Run tests and verify pass**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/UsersView.spec.ts
cd frontend && pnpm typecheck
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/components/layout/AppSidebar.vue frontend/src/components/layout/AppLayout.vue frontend/src/components/layout/AppHeader.vue frontend/src/components/user/profile/ProfileInfoCard.vue frontend/src/components/admin/user/UserCreateModal.vue frontend/src/components/admin/user/UserEditModal.vue frontend/src/i18n/locales/zh/admin/overview.ts frontend/src/i18n/locales/en/admin/overview.ts frontend/src/types/index.ts frontend/src/views/admin/__tests__/UsersView.spec.ts
git commit -m "feat: expose sub admin workbench navigation"
```

---

## Task 9: End-to-End Verification

**Files:**
- No code changes expected unless verification finds bugs.

**Step 1: Run focused backend tests**

Run:

```bash
cd backend && go test ./internal/service ./internal/server/middleware ./internal/server -run 'Role|Admin.*Auth|Redeem|Workbench' -count=1
```

Expected: PASS.

**Step 2: Run focused frontend tests**

Run:

```bash
cd frontend && pnpm test:run src/router/__tests__/guards.spec.ts src/api/__tests__/admin.workbench.spec.ts src/api/__tests__/redeem.spec.ts src/views/admin/__tests__/AdminWorkbenchView.spec.ts src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts
```

Expected: PASS.

**Step 3: Run type checks and builds**

Run:

```bash
cd frontend && pnpm typecheck
cd frontend && pnpm build
cd backend && go build -o /tmp/sub2api-sub-admin-workbench ./cmd/server
```

Expected: all commands PASS.

**Step 4: Local manual smoke test**

Start backend and frontend:

```bash
cd backend && /tmp/sub2api-sub-admin-workbench
cd frontend && pnpm exec vite --host 127.0.0.1 --port 3000
```

Manual checks:

- Admin can open `/admin/workbench`.
- Sub-admin can open `/admin/workbench`.
- Sub-admin cannot open `/admin/users`.
- Regular user cannot open `/admin/workbench`.
- `/redeem` no longer shows balance-to-code generator.
- Workbench generation deducts current user's balance and lists generated codes.

**Step 5: Final commit if smoke fixes were needed**

```bash
git add <changed-files>
git commit -m "fix: polish sub admin workbench flow"
```

---

## Rollout Notes

- No database migration is required because `users.role` is already a string.
- Existing `balance_redeem_code_enabled` remains in the database for compatibility but stops granting generation permission.
- Before deployment, decide which accounts should be changed from `user` to `sub_admin`.
- After deployment, verify existing normal users cannot call the old `/api/v1/redeem/generate` endpoint.
