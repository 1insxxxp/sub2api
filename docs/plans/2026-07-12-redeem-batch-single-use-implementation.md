# Redeem Batch Single-use Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an optional generation-time rule that lets each account redeem at most one balance, concurrency, or subscription code from the generated batch.

**Architecture:** Store an optional UUID batch identifier on restricted redeem codes and create a durable claim row inside the existing redemption transaction. A unique `(batch_id, user_id)` database constraint provides concurrency safety while additive API fields and Vue controls expose the rule to administrators and users.

**Tech Stack:** Go, Ent, PostgreSQL/SQLite tests, Gin, Vue 3, TypeScript, Vitest, vue-i18n, Vite, Docker Compose.

---

### Task 1: Add schema-level batch claims

**Files:**
- Modify: `backend/ent/schema/redeem_code.go`
- Create: `backend/ent/schema/redeem_batch_claim.go`
- Regenerate: `backend/ent/**`
- Test: `backend/internal/service/redeem_service_redeem_test.go`

**Step 1: Write a failing redemption test**

Create two unused redeem codes with the same batch ID. Redeem the first as one user, then assert redeeming the second returns `ErrRedeemBatchUserLimit`, leaves the second code unused, and grants no second benefit.

**Step 2: Run the focused test and confirm failure**

Run: `cd backend && go test -tags unit ./internal/service -run TestRedeemService_BatchSingleUse -count=1`

Expected: FAIL because `BatchID`, the claim entity, and the batch-limit error do not exist.

**Step 3: Add Ent schemas**

Add optional/nillable `batch_id` to `redeem_codes` with an index. Add `redeem_batch_claims` fields:

```text
batch_id       string, non-empty
user_id        int64
redeem_code_id int64
created_at     timestamptz
```

Create a unique index on `(batch_id, user_id)` and non-unique indexes on `user_id` and `redeem_code_id`. Do not add a foreign-key edge to the redeem code so claims survive hard deletion.

**Step 4: Regenerate Ent code**

Run: `cd backend && go generate ./ent`

Expected: generated entity, create/query/update/mutation files and client wiring include the new claim schema and redeem-code field.

### Task 2: Enforce the rule transactionally

**Files:**
- Modify: `backend/internal/service/redeem_code.go`
- Modify: `backend/internal/service/redeem_service.go`
- Modify: `backend/internal/repository/redeem_code_repo.go`
- Modify: repository/service test stubs implementing `RedeemCodeRepository`
- Test: `backend/internal/service/redeem_service_redeem_test.go`
- Test: `backend/internal/repository/redeem_code_repo_integration_test.go`

**Step 1: Map the new field through service and repository models**

Add `BatchID *string` to `service.RedeemCode`; persist it in create, bulk create, update, and entity mapping paths.

**Step 2: Add a stable service error**

Define HTTP 409 `REDEEM_BATCH_USER_LIMIT` with message `activity redeem codes are limited to one per user`.

**Step 3: Insert a claim inside the transaction**

After the transaction starts and before `Use`, insert a claim through `tx.Client().RedeemBatchClaim.Create()`. Convert only the unique `(batch_id, user_id)` constraint error to `ErrRedeemBatchUserLimit`; propagate other errors. A later failure must roll back the claim.

**Step 4: Add edge-case tests**

Cover unrestricted codes, different batches, different users, rollback after benefit failure, claim persistence after code deletion, and two concurrent attempts where exactly one succeeds.

**Step 5: Run focused backend tests**

Run: `cd backend && go test -tags unit ./internal/service -run 'TestRedeemService.*Batch' -count=1`

Run: `cd backend && go test -tags integration ./internal/repository -run 'RedeemCode.*Batch' -count=1`

Expected: PASS.

### Task 3: Generate restricted batches through the administrator API

**Files:**
- Modify: `backend/internal/service/admin_service.go`
- Modify: `backend/internal/service/admin_user.go`
- Modify: `backend/internal/handler/admin/redeem_handler.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `backend/internal/handler/admin/admin_service_stub_test.go`
- Test: relevant admin redeem handler/service tests

**Step 1: Write failing generation tests**

Assert `single_use_per_user: true` gives every code from one operation the same non-empty batch ID, a second operation receives another ID, false produces nil batch IDs, and invitation plus true is rejected.

**Step 2: Implement generation propagation**

Add `SingleUsePerUser bool` to request/input DTOs. In `GenerateRedeemCodes`, create one `uuid.NewString()` per restricted operation and assign it to every code before bulk creation.

**Step 3: Expose administrator-only metadata**

Return `batch_id` and `single_use_per_user` on `AdminRedeemCode`. Do not expose batch IDs in ordinary user history DTOs.

**Step 4: Run admin/service tests**

Run: `cd backend && go test -tags unit ./internal/service ./internal/handler/admin -run 'Redeem' -count=1`

Expected: PASS.

### Task 4: Add the administrator generation option

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/redeem.ts`
- Modify: `frontend/src/views/admin/RedeemView.vue`
- Modify: `frontend/src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts` or create a focused generation spec
- Modify: `frontend/src/i18n/locales/zh/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/en/admin/resources.ts`

**Step 1: Write failing component/API tests**

Assert the checkbox appears for balance, concurrency, and subscription; is hidden/reset for invitation; and sends `single_use_per_user: true`. Assert restricted rows display a compact badge.

**Step 2: Extend TypeScript contracts**

Add `batch_id`, `single_use_per_user`, and the generation request flag.

**Step 3: Implement the checkbox and badge**

Use the existing checkbox/toggle styling in the generation dialog. Default false and reset false after successful generation or switching to invitation.

**Step 4: Run focused administrator tests**

Run: `cd frontend && pnpm test:run src/views/admin/__tests__/RedeemView*.spec.ts`

Expected: PASS.

### Task 5: Show the specific user limit error

**Files:**
- Modify: `frontend/src/views/user/RedeemView.vue`
- Modify: `frontend/src/views/user/__tests__/RedeemView.checkinReward.spec.ts` or create a focused redemption-error spec
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`

**Step 1: Write a failing component test**

Mock redemption rejection with `{ code: 'REDEEM_BATCH_USER_LIMIT' }`. Assert the inline error and red toast both use the localized activity limit message.

**Step 2: Handle the flattened API error contract**

Inspect `error.code` first because the Axios interceptor returns a flattened error object. Preserve generic behavior for all other failures.

**Step 3: Run focused user tests**

Run: `cd frontend && pnpm test:run src/views/user/__tests__/RedeemView*.spec.ts`

Expected: PASS.

### Task 6: Complete verification and local runtime update

**Files:**
- Modify only planned files if verification reveals issues.

**Step 1: Run backend verification**

Run: `cd backend && go test -tags unit ./internal/service ./internal/handler ./internal/handler/admin -count=1`

Run repository integration tests when the configured integration database is available.

**Step 2: Run frontend verification**

Run: `cd frontend && pnpm test:run`

Run: `cd frontend && pnpm typecheck`

Run: `cd frontend && pnpm build`

Expected: all commands PASS.

**Step 3: Review and commit**

Run: `git diff --check && git status --short && git diff --stat`

Commit: `feat: limit activity redeem batches per user`

**Step 4: Update local runtime**

Confirm Vite HMR on `http://127.0.0.1:3000`. Rebuild/reinject the final backend binary and recreate only `sub2api-dev` with `--no-deps`; do not restart PostgreSQL or Redis.

Verify health, version metadata, generation payload, first redemption success, second same-batch rejection, and that the rejected code remains unused.
