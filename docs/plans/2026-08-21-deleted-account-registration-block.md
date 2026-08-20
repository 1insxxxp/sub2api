# Deleted Account Registration Block Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent a soft-deleted account email, including alias variants, from being used to register again.

**Architecture:** Keep normal user lookups soft-delete aware, but add registration-only include-deleted checks in the auth registration path and concrete user repository. Verification-code requests and final create guards reuse the same policy so users get a consistent `EMAIL_EXISTS` failure before and during registration.

**Tech Stack:** Go service and repository code, unit tests with existing stubs, optional local API verification through Gin server.

---

### Task 1: Service-Level TDD

**Files:**
- Modify: `backend/internal/service/auth_service_register_test.go`
- Modify: `backend/internal/service/registration_email_alias.go`
- Modify: `backend/internal/service/user_service.go`

**Step 1: Write failing tests**

Add tests that configure a `userRepoStub` with include-deleted email collisions and assert:
- `Register` returns `ErrEmailExists` for an exact deleted email,
- `Register` returns `ErrEmailExists` for a deleted alias collision,
- `SendVerifyCode` returns `ErrEmailExists` for a deleted email before sending.

**Step 2: Run test to verify it fails**

Run:

```bash
go test -tags=unit ./internal/service -run 'TestAuthService_Register_BlocksDeleted|TestAuthService_SendVerifyCode_BlocksDeleted' -count=1
```

Expected: FAIL because the service does not know about include-deleted registration identity checks.

**Step 3: Implement minimal service code**

Add a narrow interface:

```go
type RegistrationEmailIdentityRepository interface {
    ExistsByEmailOrAliasIncludeDeleted(ctx context.Context, email string) (bool, error)
}
```

Update `AuthService.existsByEmailOrAlias` to prefer that interface and fall back to the existing `ExistsByEmail` + `ExistsByEmailAlias` path for old stubs.

**Step 4: Run test to verify it passes**

Run the same service test command. Expected: PASS.

### Task 2: Repository Include-Deleted Guard

**Files:**
- Modify: `backend/internal/repository/user_repo.go`
- Test: `backend/internal/repository/user_repo_email_lookup_unit_test.go` or `backend/internal/repository/user_repo_integration_test.go`

**Step 1: Write failing repository test**

Add a focused test showing include-deleted registration lookup returns true for a soft-deleted exact email or alias variant.

**Step 2: Run test to verify it fails**

Run the narrow repository test command that matches the chosen test file. Expected: FAIL because the method does not exist or still filters deleted users.

**Step 3: Implement minimal repository code**

Add `ExistsByEmailOrAliasIncludeDeleted(ctx, email)` and a helper that runs exact and alias checks under `mixins.SkipSoftDelete(ctx)`.

Update `CreateWithEmailAliasGuard`'s final checks to include deleted rows before creating the user.

**Step 4: Run repository test to verify it passes**

Run the same repository command. Expected: PASS.

### Task 3: Regression Verification

**Files:**
- All changed backend files.

**Step 1: Run targeted backend tests**

```bash
go test -tags=unit ./internal/service -run 'TestAuthService_Register_BlocksDeleted|TestAuthService_SendVerifyCode_BlocksDeleted|TestExistsByEmailOrAlias' -count=1
go test -tags=unit ./internal/repository -run 'DeletedAccountRegistration|EmailLookup' -count=1
```

**Step 2: Run account deletion tests**

```bash
go test -tags=unit ./internal/service -run 'TestDeleteOwnAccount|TestAuthService_Register' -count=1
go test -tags=unit ./internal/handler ./internal/server -run 'TestUserHandlerDeleteOwnAccount|TestAPIContracts/DELETE_/api/v1/user/account' -count=1
```

**Step 3: Build backend**

```bash
go build ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./cmd/server
```

### Task 4: Local API Verification

**Files:**
- None unless a defect appears.

**Step 1: Restart local backend**

Use the existing local config:

```bash
DATA_DIR=/Users/alien/Documents/sub2/backend SERVER_PORT=18083 go run ./cmd/server
```

**Step 2: Verify endpoint behavior**

Use a unique test email. Register, call `DELETE /api/v1/user/account` with the token and password, then try registering the same email again.

Expected: the second registration returns an `EMAIL_EXISTS` response.

### Task 5: Commit And Push

**Files:**
- All changed files.

**Step 1: Clean diff checks**

```bash
gofmt -w backend/internal/service/registration_email_alias.go backend/internal/service/user_service.go backend/internal/service/auth_service_register_test.go backend/internal/repository/user_repo.go
git diff --check
git status --short
```

**Step 2: Commit**

```bash
git add backend
git add -f docs/plans/2026-08-21-deleted-account-registration-block-design.md docs/plans/2026-08-21-deleted-account-registration-block.md
git commit -m "fix: block registration for deleted accounts"
```

**Step 3: Push**

```bash
git push origin codex/balance-transfer-redeem-codes
```
