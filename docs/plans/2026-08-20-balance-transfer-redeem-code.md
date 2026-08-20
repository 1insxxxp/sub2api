# Balance Transfer Redeem Code Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow admin-designated users to convert their own balance into one-time balance redeem codes that other users can redeem.

**Architecture:** Add a per-user permission flag and redeem-code audit fields, expose self-service generate/list endpoints under the existing user redeem API, and reuse the existing balance redeem flow for recipients. User-generated transfer codes are marked with `source = user_balance_transfer` so redemption can skip affiliate rebates.

**Tech Stack:** Go, Gin, ent, PostgreSQL migrations, Vue 3, Pinia, Vite, Vitest, pnpm.

---

## Baseline Notes

- Frontend relevant baseline passed before implementation:
  - `pnpm exec vitest run src/views/user/__tests__/RedeemView.checkinReward.spec.ts src/views/user/__tests__/RedeemView.batchSingleUse.spec.ts src/views/user/__tests__/RedeemViewLayout.spec.ts src/api/__tests__/admin.users.spec.ts src/api/__tests__/user.spec.ts src/views/admin/__tests__/UsersView.spec.ts`
- Backend `go test ./...` had pre-existing failures before feature work:
  - `backend/internal/handler/available_channel_pricing_fallback_test.go` cannot find `../../data/model_pricing.json`.
  - `backend/internal/handler/usage_record_success_boundary_contract_test.go` reports a usage submission order contract mismatch.
  - The command was stopped after no new output; related packages already completed.
- Use focused backend commands for this feature, then run broad build/targeted tests at the end.

## Task 1: Schema And Migration

**Files:**
- Modify: `backend/ent/schema/user.go`
- Modify: `backend/ent/schema/redeem_code.go`
- Create: `backend/migrations/227_balance_transfer_redeem_codes.sql`
- Test: `backend/internal/repository/migrations_schema_integration_test.go`

**Step 1: Write the failing test**

Add migration schema assertions for:

```go
requireColumn(t, tx, "users", "balance_redeem_code_enabled", "boolean", 0, false)
requireColumn(t, tx, "redeem_codes", "created_by", "bigint", 0, true)
requireColumn(t, tx, "redeem_codes", "source", "character varying", 32, false)
requireIndex(t, tx, "idx_redeem_codes_created_by")
requireIndex(t, tx, "idx_redeem_codes_source")
```

**Step 2: Run the test to verify it fails**

```bash
go test -tags=integration ./internal/repository -run TestMigrationsSchema -count=1
```

Expected: FAIL because the new columns and indexes do not exist.

**Step 3: Implement schema and migration**

Add `balance_redeem_code_enabled` to `User` ent schema with default false.

Add to `RedeemCode` ent schema:

```go
field.Int64("created_by").Optional().Nillable()
field.String("source").MaxLen(32).Default("admin")
```

Create `227_balance_transfer_redeem_codes.sql`:

```sql
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS balance_redeem_code_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE redeem_codes
  ADD COLUMN IF NOT EXISTS created_by BIGINT NULL,
  ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT 'admin';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'redeem_codes_created_by_fkey'
  ) THEN
    ALTER TABLE redeem_codes
      ADD CONSTRAINT redeem_codes_created_by_fkey
      FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL NOT VALID;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_created_by ON redeem_codes(created_by);
CREATE INDEX IF NOT EXISTS idx_redeem_codes_source ON redeem_codes(source);
```

Run `go generate ./ent`.

**Step 4: Run the test to verify it passes**

```bash
go test -tags=integration ./internal/repository -run TestMigrationsSchema -count=1
```

**Step 5: Commit**

```bash
git add backend/ent backend/migrations backend/internal/repository/migrations_schema_integration_test.go
git commit -m "feat: add balance transfer redeem code schema"
```

## Task 2: Service Models And Repositories

**Files:**
- Modify: `backend/internal/service/user.go`
- Modify: `backend/internal/service/user_service.go`
- Modify: `backend/internal/service/redeem_code.go`
- Modify: `backend/internal/repository/user_repo.go`
- Modify: `backend/internal/repository/redeem_code_repo.go`
- Test: `backend/internal/repository/user_repo_integration_test.go`
- Test: `backend/internal/repository/redeem_code_repo_integration_test.go`

**Step 1: Write failing tests**

Add repository tests proving:

- `balance_redeem_code_enabled` persists through user create/update/get/list.
- `created_by` and `source` persist through redeem code create/get/list.
- Generated list can load the creator and used user independently.

**Step 2: Run the tests to verify they fail**

```bash
go test ./internal/repository -run 'TestUserRepoSuite|TestRedeemCodeRepoSuite' -count=1
```

Expected: FAIL because service structs and mappings do not include the fields.

**Step 3: Implement model and mapping**

Add to `service.User`:

```go
BalanceRedeemCodeEnabled bool
```

Add to `UserUpdateFields`:

```go
BalanceRedeemCodeEnabled bool
```

Add to `service.RedeemCode`:

```go
CreatedBy *int64
Source    string
Creator   *User
```

Add constants:

```go
const (
  RedeemCodeSourceAdmin = "admin"
  RedeemCodeSourceUserBalanceTransfer = "user_balance_transfer"
)
```

Update repository create, create batch, update, list eager loading, and entity-to-service mappers.

**Step 4: Run the tests to verify they pass**

```bash
go test ./internal/repository -run 'TestUserRepoSuite|TestRedeemCodeRepoSuite' -count=1
```

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/repository
git commit -m "feat: persist balance transfer redeem metadata"
```

## Task 3: Admin Permission Toggle Backend

**Files:**
- Modify: `backend/internal/service/admin_service.go`
- Modify: `backend/internal/service/admin_user.go`
- Modify: `backend/internal/handler/admin/user_handler.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Test: `backend/internal/service/admin_service_create_user_test.go`
- Test: `backend/internal/service/admin_service_role_test.go`
- Test: `backend/internal/server/api_contract_test.go`

**Step 1: Write failing tests**

Cover:

- Admin create user can set `balance_redeem_code_enabled`.
- Admin update user can change only this flag.
- User DTOs include the flag.
- API contract accepts and returns the flag.

**Step 2: Run the tests to verify they fail**

```bash
go test ./internal/service -run 'TestAdminService.*User|Test.*BalanceRedeem' -count=1
go test ./internal/server -run TestAPIContract -count=1
```

**Step 3: Implement admin flow**

Add request/input fields, wire through handler to service, set the `UserUpdateFields` bit only when update input pointer is provided, and include the flag in user DTOs.

**Step 4: Run the tests to verify they pass**

```bash
go test ./internal/service -run 'TestAdminService.*User|Test.*BalanceRedeem' -count=1
go test ./internal/server -run TestAPIContract -count=1
```

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/handler backend/internal/server
git commit -m "feat: expose balance transfer permission"
```

## Task 4: Balance Transfer Service And User Endpoints

**Files:**
- Modify: `backend/internal/service/redeem_service.go`
- Modify: `backend/internal/handler/redeem_handler.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `backend/internal/repository/redeem_code_repo.go`
- Test: `backend/internal/service/redeem_service_balance_transfer_test.go`
- Test: `backend/internal/server/api_contract_test.go`

**Step 1: Write failing service and route tests**

Service tests:

- unauthorized user cannot generate and balance is unchanged.
- disabled user cannot generate.
- insufficient balance creates no code.
- success deducts amount and creates an unused `balance` code with `created_by` and `source = user_balance_transfer`.
- generated list returns only the current user's transfer codes.

API contract tests:

- `POST /api/v1/redeem/generate`
- `GET /api/v1/redeem/generated`

**Step 2: Run the tests to verify they fail**

```bash
go test ./internal/service -run TestRedeemServiceBalanceTransfer -count=1
go test ./internal/server -run TestAPIContract -count=1
```

**Step 3: Implement service, handler, and routes**

Add:

```go
type GenerateBalanceTransferCodeInput struct {
  Amount        float64
  ExpiresInDays int
  Notes         string
}
```

Add service methods:

```go
GenerateBalanceTransferCode(ctx context.Context, userID int64, input GenerateBalanceTransferCodeInput) (*RedeemCode, error)
ListGeneratedBalanceTransferCodes(ctx context.Context, userID int64, limit int) ([]RedeemCode, error)
```

Routes:

```go
redeem.POST("/generate", h.Redeem.GenerateBalanceTransferCode)
redeem.GET("/generated", h.Redeem.GetGenerated)
```

Use an ent transaction when available. The transaction must check permission, deduct via `AdjustBalance`, create the code, commit, then invalidate creator caches.

**Step 4: Run the tests to verify they pass**

```bash
go test ./internal/service -run TestRedeemServiceBalanceTransfer -count=1
go test ./internal/server -run TestAPIContract -count=1
```

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/handler backend/internal/server backend/internal/repository
git commit -m "feat: add user balance transfer redeem API"
```

## Task 5: Skip Affiliate Rebate For Transfer Codes

**Files:**
- Modify: `backend/internal/service/redeem_service.go`
- Test: `backend/internal/service/redeem_service_redeem_test.go`

**Step 1: Write failing test**

Add `TestRedeemSkipsAffiliateForBalanceTransferCode`. It should redeem a positive `balance` code whose source is `user_balance_transfer`, credit the recipient, and assert the affiliate rebate stub is not called.

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/service -run TestRedeemSkipsAffiliateForBalanceTransferCode -count=1
```

**Step 3: Implement skip logic**

Skip redeem-level affiliate rebate when:

```go
code.Source == RedeemCodeSourceUserBalanceTransfer
```

Keep existing `ContextSkipRedeemAffiliate` behavior.

**Step 4: Run the test to verify it passes**

```bash
go test ./internal/service -run TestRedeemSkipsAffiliateForBalanceTransferCode -count=1
```

**Step 5: Commit**

```bash
git add backend/internal/service
git commit -m "fix: skip affiliate rebate for balance transfer codes"
```

## Task 6: Frontend API Types

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/redeem.ts`
- Modify: `frontend/src/api/admin/users.ts`
- Create or Modify: `frontend/src/api/__tests__/redeem.spec.ts`
- Test: `frontend/src/api/__tests__/admin.users.spec.ts`

**Step 1: Write failing API tests**

Cover:

- `redeemAPI.generateBalanceTransferCode({ amount: 5, expires_in_days: 30 })` posts to `/redeem/generate`.
- `redeemAPI.getGenerated()` gets `/redeem/generated`.
- Admin user request types accept `balance_redeem_code_enabled`.

**Step 2: Run the tests to verify they fail**

```bash
pnpm exec vitest run src/api/__tests__/redeem.spec.ts src/api/__tests__/admin.users.spec.ts
```

**Step 3: Implement API/types**

Add `balance_redeem_code_enabled?: boolean` to `User`. Add transfer request/response types and API functions in `frontend/src/api/redeem.ts`. Add the flag to admin create/update request types.

**Step 4: Run the tests to verify they pass**

```bash
pnpm exec vitest run src/api/__tests__/redeem.spec.ts src/api/__tests__/admin.users.spec.ts
```

**Step 5: Commit**

```bash
git add frontend/src/types frontend/src/api
git commit -m "feat: add balance transfer frontend API"
```

## Task 7: Admin UI Toggle

**Files:**
- Modify: `frontend/src/components/admin/user/UserCreateModal.vue`
- Modify: `frontend/src/components/admin/user/UserEditModal.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/overview.ts`
- Modify: `frontend/src/i18n/locales/en/admin/overview.ts`
- Test: `frontend/src/components/admin/user/__tests__/UserCreateModal.balanceTransfer.spec.ts`
- Test: `frontend/src/components/admin/user/__tests__/UserEditModal.balanceTransfer.spec.ts`

**Step 1: Write failing component tests**

Test that create/edit modals initialize the toggle and send `balance_redeem_code_enabled`.

**Step 2: Run the tests to verify they fail**

```bash
pnpm exec vitest run src/components/admin/user/__tests__/UserCreateModal.balanceTransfer.spec.ts src/components/admin/user/__tests__/UserEditModal.balanceTransfer.spec.ts
```

**Step 3: Implement UI**

Add a checkbox/toggle with i18n keys:

```ts
admin.users.form.balanceRedeemCodeEnabled
admin.users.form.balanceRedeemCodeEnabledHint
```

Include the flag in create and update payloads.

**Step 4: Run the tests to verify they pass**

```bash
pnpm exec vitest run src/components/admin/user/__tests__/UserCreateModal.balanceTransfer.spec.ts src/components/admin/user/__tests__/UserEditModal.balanceTransfer.spec.ts
```

**Step 5: Commit**

```bash
git add frontend/src/components/admin/user frontend/src/i18n
git commit -m "feat: add admin balance transfer toggle"
```

## Task 8: User Redeem Page Generator

**Files:**
- Modify: `frontend/src/views/user/RedeemView.vue`
- Modify: `frontend/src/i18n/locales/zh/redeem.ts`
- Modify: `frontend/src/i18n/locales/en/redeem.ts`
- Test: `frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`

**Step 1: Write failing component tests**

Test that:

- Users without permission do not see the generator.
- Users with permission see amount and expiry inputs.
- Submitting calls `redeemAPI.generateBalanceTransferCode`, refreshes profile, refreshes generated list, and displays copyable code.
- The generated list renders status and code prefix.

**Step 2: Run the test to verify it fails**

```bash
pnpm exec vitest run src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts
```

**Step 3: Implement UI**

Add a full-width panel below the redeem form, visible only when:

```ts
user.value?.balance_redeem_code_enabled === true
```

Use amount input, expiry days input defaulting to 30, generate button, success code display with copy button, and generated list. Use existing `brand-surface` and compact form/list styles.

**Step 4: Run the test to verify it passes**

```bash
pnpm exec vitest run src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts
```

**Step 5: Commit**

```bash
git add frontend/src/views/user frontend/src/i18n
git commit -m "feat: add user balance transfer redeem UI"
```

## Task 9: Final Verification

**Step 1: Backend focused tests**

```bash
go test ./internal/service -run 'TestRedeemServiceBalanceTransfer|TestRedeemSkipsAffiliateForBalanceTransferCode|TestAdminService.*User|Test.*BalanceRedeem' -count=1
go test -tags=integration ./internal/repository -run 'TestMigrationsSchema|TestUserRepoSuite|TestRedeemCodeRepoSuite' -count=1
go test ./internal/server -run TestAPIContract -count=1
```

**Step 2: Frontend focused tests**

```bash
pnpm exec vitest run src/api/__tests__/redeem.spec.ts src/api/__tests__/admin.users.spec.ts src/components/admin/user/__tests__/UserCreateModal.balanceTransfer.spec.ts src/components/admin/user/__tests__/UserEditModal.balanceTransfer.spec.ts src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts src/views/user/__tests__/RedeemView.batchSingleUse.spec.ts src/views/user/__tests__/RedeemView.checkinReward.spec.ts src/views/admin/__tests__/UsersView.spec.ts
```

**Step 3: Build/typecheck**

```bash
go build ./cmd/server
pnpm run typecheck
pnpm run build
```

Expected: all focused tests and builds pass. If full backend `go test ./...` is rerun, keep the baseline failures documented separately unless fixed by unrelated upstream changes.
