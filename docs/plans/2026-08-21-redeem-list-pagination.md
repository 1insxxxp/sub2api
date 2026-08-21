# Redeem List Pagination Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add server-side pagination to the user redeem page's recent activity list and generated redeem-code list.

**Architecture:** Merge the existing balance-transfer redeem-code feature branch into `dev`, then make both user list endpoints support paginated responses when `page`, `page_size`, or `limit` is provided. Keep legacy no-query calls returning arrays so existing clients do not break, while the frontend explicitly opts into paginated responses.

**Tech Stack:** Go/Gin backend, Ent repositories, existing `pagination` and `response` helpers, Vue 3 Composition API, TypeScript, existing `Pagination.vue`, Vitest.

---

### Task 1: Bring Generated Redeem-Code Feature Onto `dev`

**Files:**
- Modify: `backend/internal/handler/redeem_handler.go`
- Modify: `backend/internal/repository/redeem_code_repo.go`
- Modify: `backend/internal/service/redeem_service.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `frontend/src/api/redeem.ts`
- Modify: `frontend/src/views/user/RedeemView.vue`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`
- Test: `backend/internal/service/redeem_service_balance_transfer_test.go`
- Test: `frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`
- Test: `frontend/src/api/__tests__/redeem.spec.ts`

**Step 1: Merge the feature branch**

Run:

```bash
git merge --no-ff origin/codex/balance-transfer-redeem-codes
```

Expected: merge succeeds or reports conflicts only in the files listed above.

**Step 2: Resolve conflicts conservatively**

Keep current `dev` behavior for unrelated custom-group and compensation work. Keep the balance-transfer branch behavior for:

```go
POST /api/v1/redeem/generate
GET /api/v1/redeem/generated
DELETE /api/v1/redeem/generated/:id
```

Expected: `rg -n "GetGenerated|GenerateBalanceTransferCode|DeleteGenerated" backend/internal frontend/src` finds backend routes, handler methods, service methods, API methods, and `RedeemView.vue` usage.

**Step 3: Run the existing feature tests**

Run:

```bash
go test ./backend/internal/service -run BalanceTransfer
npm --prefix frontend test -- RedeemView.balanceTransfer redeem.spec
```

Expected: tests pass or reveal conflicts introduced by the merge.

**Step 4: Commit the merge**

Run:

```bash
git add backend frontend
git commit
```

Expected: one merge commit that only brings in the balance-transfer redeem-code feature.

### Task 2: Add Backend Pagination Methods

**Files:**
- Modify: `backend/internal/service/redeem_service.go`
- Modify: `backend/internal/repository/redeem_code_repo.go`
- Test: `backend/internal/service/redeem_service_balance_transfer_test.go`

**Step 1: Write failing service tests**

Add tests that call:

```go
service.GetUserHistoryPaginated(ctx, userID, pagination.PaginationParams{Page: 2, PageSize: 10})
service.ListGeneratedBalanceTransferCodesPaginated(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 10})
```

Expected assertions:

```go
require.Len(t, items, expectedCount)
require.Equal(t, int64(expectedTotal), page.Total)
require.Equal(t, 2, page.Page)
require.Equal(t, 10, page.PageSize)
```

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./backend/internal/service -run "UserHistoryPaginated|GeneratedBalanceTransfer"
```

Expected: fail because the new paginated service methods do not exist.

**Step 3: Implement minimal service/repository changes**

Add to `RedeemCodeRepository`:

```go
ListGeneratedByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error)
```

Add to `RedeemService`:

```go
func (s *RedeemService) GetUserHistoryPaginated(ctx context.Context, userID int64, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error)

func (s *RedeemService) ListGeneratedBalanceTransferCodesPaginated(ctx context.Context, userID int64, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error)
```

Repository query for generated codes must filter:

```go
redeemcode.CreatedByEQ(userID)
redeemcode.SourceEQ(service.RedeemCodeSourceUserBalanceTransfer)
redeemcode.TypeEQ(service.RedeemTypeBalance)
```

Order generated codes by `created_at DESC`.

**Step 4: Run tests to verify pass**

Run:

```bash
go test ./backend/internal/service -run "UserHistoryPaginated|GeneratedBalanceTransfer"
```

Expected: pass.

**Step 5: Commit**

Run:

```bash
git add backend/internal/service/redeem_service.go backend/internal/repository/redeem_code_repo.go backend/internal/service/redeem_service_balance_transfer_test.go
git commit -m "feat: paginate user redeem code queries"
```

### Task 3: Add Backend Handler Pagination

**Files:**
- Modify: `backend/internal/handler/redeem_handler.go`
- Test: `backend/internal/server/api_contract_test.go`

**Step 1: Write failing API contract tests**

Add cases for:

```http
GET /api/v1/redeem/history?page=1&page_size=10
GET /api/v1/redeem/generated?page=1&page_size=10
```

Expected JSON data shape:

```json
{
  "items": [],
  "total": 0,
  "page": 1,
  "page_size": 10,
  "pages": 1
}
```

Keep the existing no-query `/redeem/history` behavior returning an array. Keep no-query `/redeem/generated` returning an array.

**Step 2: Run tests to verify failure**

Run:

```bash
go test ./backend/internal/server -run TestAPIContract/redeem
```

Expected: fail for the new paginated cases.

**Step 3: Implement handler changes**

In `GetHistory` and `GetGenerated`, detect pagination opt-in:

```go
paginated := c.Query("page") != "" || c.Query("page_size") != "" || c.Query("limit") != ""
```

When `paginated` is false, keep the existing array response.

When `paginated` is true:

```go
page, pageSize := response.ParsePagination(c)
params := pagination.PaginationParams{Page: page, PageSize: pageSize}
codes, result, err := h.redeemService.GetUserHistoryPaginated(ctx, subject.UserID, params)
response.Paginated(c, out, result.Total, result.Page, result.PageSize)
```

Use the generated-code paginated service method in `GetGenerated`.

**Step 4: Run tests to verify pass**

Run:

```bash
go test ./backend/internal/server -run TestAPIContract/redeem
```

Expected: pass.

**Step 5: Commit**

Run:

```bash
git add backend/internal/handler/redeem_handler.go backend/internal/server/api_contract_test.go
git commit -m "feat: paginate user redeem endpoints"
```

### Task 4: Add Frontend Pagination API Types

**Files:**
- Modify: `frontend/src/api/redeem.ts`
- Test: `frontend/src/api/__tests__/redeem.spec.ts`

**Step 1: Write failing API tests**

Assert that:

```ts
redeemAPI.getHistory({ page: 2, page_size: 10 })
redeemAPI.getGenerated({ page: 1, page_size: 10 })
```

call:

```ts
/redeem/history?page=2&page_size=10
/redeem/generated?page=1&page_size=10
```

and return `PaginatedResponse<T>`.

**Step 2: Run tests to verify failure**

Run:

```bash
npm --prefix frontend test -- redeem.spec
```

Expected: fail because the API methods do not accept pagination params.

**Step 3: Implement API changes**

Import the shared pagination type:

```ts
import type { PaginatedResponse, RedeemCode, RedeemCodeRequest } from '@/types'
```

Add:

```ts
export interface RedeemListParams {
  page?: number
  page_size?: number
}
```

Change:

```ts
getHistory(params: RedeemListParams): Promise<PaginatedResponse<RedeemHistoryItem>>
getGenerated(params: RedeemListParams): Promise<PaginatedResponse<GeneratedRedeemCode>>
```

Use `apiClient.get(url, { params })`.

**Step 4: Run tests to verify pass**

Run:

```bash
npm --prefix frontend test -- redeem.spec
```

Expected: pass.

**Step 5: Commit**

Run:

```bash
git add frontend/src/api/redeem.ts frontend/src/api/__tests__/redeem.spec.ts
git commit -m "feat: add redeem pagination client APIs"
```

### Task 5: Add Frontend List Controls

**Files:**
- Modify: `frontend/src/views/user/RedeemView.vue`
- Test: `frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`

**Step 1: Write failing view tests**

Add assertions that:

```ts
expect(redeemAPI.getHistory).toHaveBeenCalledWith({ page: 1, page_size: 10 })
expect(redeemAPI.getGenerated).toHaveBeenCalledWith({ page: 1, page_size: 10 })
```

Mount the view with totals greater than one page and assert that two `Pagination` components render, one for generated codes and one for recent activity.

**Step 2: Run tests to verify failure**

Run:

```bash
npm --prefix frontend test -- RedeemView.balanceTransfer
```

Expected: fail because no pagination controls are wired.

**Step 3: Implement view state and handlers**

Import:

```ts
import Pagination from '@/components/common/Pagination.vue'
```

Add state:

```ts
const generatedPagination = reactive({ page: 1, page_size: 10, total: 0 })
const historyPagination = reactive({ page: 1, page_size: 10, total: 0 })
```

Update fetchers:

```ts
const response = await redeemAPI.getHistory({
  page: historyPagination.page,
  page_size: historyPagination.page_size
})
history.value = response.items
historyPagination.total = response.total
```

Use the same pattern for generated codes.

Add handlers:

```ts
const handleHistoryPageChange = (page: number) => { historyPagination.page = page; fetchHistory() }
const handleHistoryPageSizeChange = (pageSize: number) => { historyPagination.page_size = pageSize; historyPagination.page = 1; fetchHistory() }
const handleGeneratedPageChange = (page: number) => { generatedPagination.page = page; fetchGeneratedCodes() }
const handleGeneratedPageSizeChange = (pageSize: number) => { generatedPagination.page_size = pageSize; generatedPagination.page = 1; fetchGeneratedCodes() }
```

On successful redeem, set `historyPagination.page = 1` before fetching history. On successful generation, set `generatedPagination.page = 1` before fetching generated codes.

Render `Pagination` below each non-empty list:

```vue
<Pagination
  v-if="historyPagination.total > historyPagination.page_size"
  :page="historyPagination.page"
  :total="historyPagination.total"
  :page-size="historyPagination.page_size"
  :page-size-options="[10, 20, 50]"
  @update:page="handleHistoryPageChange"
  @update:pageSize="handleHistoryPageSizeChange"
/>
```

Repeat for generated codes.

**Step 4: Run tests to verify pass**

Run:

```bash
npm --prefix frontend test -- RedeemView.balanceTransfer
```

Expected: pass.

**Step 5: Commit**

Run:

```bash
git add frontend/src/views/user/RedeemView.vue frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts
git commit -m "feat: paginate user redeem lists"
```

### Task 6: Final Verification

**Files:**
- Verify only.

**Step 1: Run backend targeted tests**

Run:

```bash
go test ./backend/internal/service -run "BalanceTransfer|UserHistoryPaginated"
go test ./backend/internal/server -run TestAPIContract/redeem
```

Expected: pass.

**Step 2: Run frontend targeted tests**

Run:

```bash
npm --prefix frontend test -- redeem.spec RedeemView.balanceTransfer
```

Expected: pass.

**Step 3: Run broader smoke checks**

Run:

```bash
npm --prefix frontend run type-check
go test ./backend/internal/handler ./backend/internal/server ./backend/internal/service
```

Expected: pass.

**Step 4: Inspect final diff**

Run:

```bash
git status --short --branch
git log --oneline --decorate -n 8
```

Expected: clean worktree after commits, branch ahead of `origin/dev` by the new implementation commits.
