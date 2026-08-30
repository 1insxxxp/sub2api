# Dujiao Sub2 Shared Login Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let users sign in to Dujiao-Next with their sub2 email/password while preserving Dujiao's existing local login.

**Architecture:** sub2 exposes a shared-secret-protected internal credential verification endpoint that returns safe user profile data and no token. Dujiao keeps local login first, then falls back to sub2 verification only when local credentials are invalid, binds or creates a local user with `provider = "sub2"`, and issues its own Dujiao user JWT.

**Tech Stack:** Go, Gin, Ent/Postgres in sub2, GORM/SQLite-or-Postgres in Dujiao-Next, bcrypt, existing Dujiao `user_oauth_identities` table.

---

## Constraints

- Work in two repositories:
  - sub2: `/Users/alien/Documents/sub2`
  - Dujiao-Next: `/Users/alien/Documents/独角兽/api`
- Do not share balances, orders, wallet data, API keys, or sub2 JWTs.
- Do not copy sub2 password hashes into Dujiao.
- Do not change Dujiao frontend login form unless a backend response contract changes.
- Keep each commit narrow. There are already unrelated local changes in the sub2 worktree, so stage exact files only.

## Task 1: Add sub2 Password-Only Verification Service

**Files:**
- Modify: `/Users/alien/Documents/sub2/backend/internal/config/config.go`
- Modify: `/Users/alien/Documents/sub2/backend/internal/service/auth_service.go`
- Test: `/Users/alien/Documents/sub2/backend/internal/service/auth_service_dujiao_login_test.go`

**Step 1: Write the failing service tests**

Create tests for:

- Active user with valid password returns safe user data.
- Wrong password returns `ErrInvalidCredentials`.
- Disabled user returns `ErrUserNotActive`.
- User with `TotpEnabled` returns a new fail-closed error.

Example shape:

```go
func TestAuthServiceVerifyExternalCredentialRejectsTotpUser(t *testing.T) {
    repo := &authRepoStub{
        byEmail: &User{
            ID: 1, Email: "user@example.com", Status: StatusActive,
            PasswordHash: mustHashPassword(t, "secret-123"),
            TotpEnabled: true,
        },
    }
    svc := &AuthService{userRepo: repo}

    _, err := svc.VerifyExternalCredential(context.Background(), "user@example.com", "secret-123")
    require.ErrorIs(t, err, ErrExternalLogin2FARequired)
}
```

**Step 2: Run the focused failing test**

Run:

```bash
cd /Users/alien/Documents/sub2/backend
go test ./internal/service -run 'TestAuthServiceVerifyExternalCredential' -count=1
```

Expected: FAIL because the method and error do not exist.

**Step 3: Add the minimal service implementation**

Add a method that verifies credentials without generating a token:

```go
var ErrExternalLogin2FARequired = infraerrors.Forbidden(
    "EXTERNAL_LOGIN_2FA_REQUIRED",
    "external login is not available for users with two-factor authentication enabled",
)

type ExternalCredentialUser struct {
    ID       int64  `json:"id"`
    Email    string `json:"email"`
    Username string `json:"username"`
    Role     string `json:"role"`
    Status   string `json:"status"`
}

func (s *AuthService) VerifyExternalCredential(ctx context.Context, email, password string) (*ExternalCredentialUser, error) {
    user, err := s.userRepo.GetByEmail(ctx, strings.TrimSpace(email))
    if err != nil {
        if errors.Is(err, ErrUserNotFound) {
            return nil, ErrInvalidCredentials
        }
        return nil, ErrServiceUnavailable
    }
    if !s.CheckPassword(password, user.PasswordHash) {
        return nil, ErrInvalidCredentials
    }
    if !user.IsActive() {
        return nil, ErrUserNotActive
    }
    if user.TotpEnabled {
        return nil, ErrExternalLogin2FARequired
    }
    return &ExternalCredentialUser{
        ID: user.ID, Email: user.Email, Username: user.Username,
        Role: user.Role, Status: user.Status,
    }, nil
}
```

**Step 4: Run the service tests**

Run:

```bash
cd /Users/alien/Documents/sub2/backend
go test ./internal/service -run 'TestAuthServiceVerifyExternalCredential' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
cd /Users/alien/Documents/sub2
git add backend/internal/service/auth_service.go backend/internal/service/auth_service_dujiao_login_test.go
git commit -m "feat: add external credential verification"
```

## Task 2: Add sub2 Internal Dujiao Verification Endpoint

**Files:**
- Create: `/Users/alien/Documents/sub2/backend/internal/handler/internal_dujiao_auth_handler.go`
- Test: `/Users/alien/Documents/sub2/backend/internal/handler/internal_dujiao_auth_handler_test.go`
- Modify: `/Users/alien/Documents/sub2/backend/internal/handler/handler.go`
- Modify: `/Users/alien/Documents/sub2/backend/internal/handler/wire.go`
- Create: `/Users/alien/Documents/sub2/backend/internal/server/routes/internal.go`
- Test: `/Users/alien/Documents/sub2/backend/internal/server/routes/internal_routes_test.go`
- Modify: `/Users/alien/Documents/sub2/backend/internal/server/router.go`
- Modify: `/Users/alien/Documents/sub2/backend/internal/config/config.go`

**Step 1: Write the failing handler tests**

Cover:

- Missing secret returns 401.
- Wrong secret returns 401 using constant-time comparison.
- Disabled integration returns 404 or 403.
- Valid request returns safe user JSON.
- Invalid credential returns generic 401.

Expected request:

```http
POST /api/v1/internal/dujiao/auth/verify
X-Sub2-Internal-Secret: test-secret
Content-Type: application/json
```

Body:

```json
{"email":"user@example.com","password":"secret-123"}
```

Expected success body:

```json
{"success":true,"data":{"user":{"id":1,"email":"user@example.com","username":"Alice","status":"active"}}}
```

**Step 2: Run the failing tests**

Run:

```bash
cd /Users/alien/Documents/sub2/backend
go test ./internal/handler ./internal/server/routes -run 'Dujiao|Internal' -count=1
```

Expected: FAIL because the handler and route do not exist.

**Step 3: Add config**

Add:

```go
type DujiaoLoginConfig struct {
    Enabled      bool   `mapstructure:"enabled"`
    SharedSecret string `mapstructure:"shared_secret"`
}
```

Add to `Config`:

```go
DujiaoLogin DujiaoLoginConfig `mapstructure:"dujiao_login"`
```

Add defaults:

```go
viper.SetDefault("dujiao_login.enabled", false)
viper.SetDefault("dujiao_login.shared_secret", "")
```

Validate that if enabled, `shared_secret` is at least 32 bytes after trimming.

**Step 4: Add handler**

The handler should depend on an interface, not concrete `AuthService`:

```go
type DujiaoCredentialVerifier interface {
    VerifyExternalCredential(ctx context.Context, email, password string) (*service.ExternalCredentialUser, error)
}
```

Secret check:

```go
func internalSecretMatches(expected, actual string) bool {
    expected = strings.TrimSpace(expected)
    actual = strings.TrimSpace(actual)
    if expected == "" || actual == "" {
        return false
    }
    expectedBytes := []byte(expected)
    actualBytes := []byte(actual)
    if len(expectedBytes) != len(actualBytes) {
        return false
    }
    return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1
}
```

**Step 5: Register route**

Add `InternalDujiaoAuth *InternalDujiaoAuthHandler` to `handler.Handlers`, wire it in `handler/wire.go`, and register:

```go
internal := v1.Group("/internal")
registerInternalDujiaoAuthRoutes(internal, h, cfg)
```

**Step 6: Run tests**

Run:

```bash
cd /Users/alien/Documents/sub2/backend
go test ./internal/handler ./internal/server/routes ./internal/config -run 'Dujiao|Internal|Config' -count=1
```

Expected: PASS.

**Step 7: Commit**

```bash
cd /Users/alien/Documents/sub2
git add backend/internal/config/config.go backend/internal/handler/internal_dujiao_auth_handler.go backend/internal/handler/internal_dujiao_auth_handler_test.go backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/server/routes/internal.go backend/internal/server/routes/internal_routes_test.go backend/internal/server/router.go
git commit -m "feat: expose dujiao credential verification"
```

## Task 3: Add Dujiao Config and sub2 Verification Client

**Files:**
- Modify: `/Users/alien/Documents/独角兽/api/internal/config/config.go`
- Modify: `/Users/alien/Documents/独角兽/api/config.yml.example`
- Create: `/Users/alien/Documents/独角兽/api/internal/modules/identity/userauth/application/sub2_login.go`
- Create: `/Users/alien/Documents/独角兽/api/internal/modules/identity/userauth/infrastructure/sub2auth/client.go`
- Test: `/Users/alien/Documents/独角兽/api/internal/modules/identity/userauth/infrastructure/sub2auth/client_test.go`

**Step 1: Write failing client tests**

Use `httptest.Server` to verify:

- Sends `X-Sub2-Internal-Secret`.
- Posts to `/api/v1/internal/dujiao/auth/verify`.
- Parses success user.
- Maps 401 to invalid credentials.
- Maps timeout/network errors to a fallback-unavailable error.

**Step 2: Run failing tests**

Run:

```bash
cd /Users/alien/Documents/独角兽/api
go test ./internal/modules/identity/userauth/infrastructure/sub2auth -count=1
```

Expected: FAIL because the package does not exist.

**Step 3: Add config**

Add:

```go
type Sub2LoginConfig struct {
    Enabled        bool   `mapstructure:"enabled"`
    BaseURL        string `mapstructure:"base_url"`
    SharedSecret   string `mapstructure:"shared_secret"`
    TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}
```

Add to `Config`:

```go
Sub2Login Sub2LoginConfig `mapstructure:"sub2_login"`
```

Defaults:

```go
viper.SetDefault("sub2_login.enabled", false)
viper.SetDefault("sub2_login.base_url", "")
viper.SetDefault("sub2_login.shared_secret", "")
viper.SetDefault("sub2_login.timeout_seconds", 3)
```

**Step 4: Add application interface**

```go
const Sub2OAuthProvider = "sub2"

type Sub2VerifiedUser struct {
    ID       string
    Email    string
    Username string
    Status   string
}

type Sub2CredentialVerifier interface {
    Verify(ctx context.Context, email, password string) (*Sub2VerifiedUser, error)
    Enabled() bool
}
```

**Step 5: Add HTTP client**

The client should never log the password. It should use a short timeout and fail closed.

**Step 6: Run tests**

Run:

```bash
cd /Users/alien/Documents/独角兽/api
go test ./internal/modules/identity/userauth/infrastructure/sub2auth ./internal/config -run 'Sub2' -count=1
```

Expected: PASS.

**Step 7: Commit**

```bash
cd /Users/alien/Documents/独角兽/api
git add internal/config/config.go config.yml.example internal/modules/identity/userauth/application/sub2_login.go internal/modules/identity/userauth/infrastructure/sub2auth/client.go internal/modules/identity/userauth/infrastructure/sub2auth/client_test.go
git commit -m "feat: add sub2 login verifier client"
```

## Task 4: Add Dujiao Login Fallback and Identity Binding

**Files:**
- Modify: `/Users/alien/Documents/独角兽/api/internal/constants/constants.go`
- Modify: `/Users/alien/Documents/独角兽/api/internal/modules/identity/userauth/application/service.go`
- Modify: `/Users/alien/Documents/独角兽/api/internal/modules/identity/userauth/application/ports.go`
- Test: `/Users/alien/Documents/独角兽/api/internal/modules/identity/userauth/integrationtest/sub2_login_test.go`
- Modify: `/Users/alien/Documents/独角兽/api/internal/app/container/services_foundation.go`

**Step 1: Write failing integration tests**

Cover:

- Local Dujiao login succeeds and does not call sub2.
- Local invalid credentials call sub2 when enabled.
- Verified sub2 user binds to existing active same-email Dujiao user.
- Verified sub2 user creates a new Dujiao user when none exists.
- Disabled same-email Dujiao user is not auto-bound.
- Existing `provider = "sub2"` identity logs in the bound Dujiao user.
- sub2 verifier outage returns normal invalid-login error.

**Step 2: Run failing tests**

Run:

```bash
cd /Users/alien/Documents/独角兽/api
go test ./internal/modules/identity/userauth/integrationtest -run 'Sub2Login' -count=1
```

Expected: FAIL because fallback login does not exist.

**Step 3: Add constants**

```go
const (
    UserOAuthProviderSub2 = "sub2"
    LoginLogSourceSub2    = "sub2"
)
```

**Step 4: Extend service dependencies**

Add to `Service`:

```go
sub2Verifier Sub2CredentialVerifier
```

Add setter:

```go
func (s *Service) SetSub2CredentialVerifier(verifier Sub2CredentialVerifier) {
    s.sub2Verifier = verifier
}
```

**Step 5: Modify `LoginStep1` fallback**

Only fall back when local login fails with invalid credentials. Do not fall back for disabled user, unverified email, malformed email, or internal DB errors.

Pseudo-flow:

```go
res, err := s.loginLocalStep1(normalized, password, rememberMe)
if err == nil || !errors.Is(err, ErrInvalidCredentials) {
    return res, err
}
if s.sub2Verifier == nil || !s.sub2Verifier.Enabled() {
    return nil, ErrInvalidCredentials
}
return s.LoginWithSub2Credential(context.Background(), normalized, password, rememberMe)
```

If refactoring existing `LoginStep1` into `loginLocalStep1`, keep public behavior unchanged.

**Step 6: Implement binding in a transaction**

Use `AuthUnitOfWork`:

1. Load existing `provider = "sub2"` identity by sub2 user ID.
2. If found, load that Dujiao user for update and require active status.
3. If not found, load Dujiao user by email for update.
4. If same-email user exists and active, create sub2 identity for that user.
5. If same-email user is disabled/deleted, fail closed.
6. If no user exists, create:

```go
user := &userdomain.User{
    Email: normalizedSub2Email,
    PasswordHash: randomBcryptHash,
    PasswordSetupRequired: true,
    DisplayName: resolvedDisplayName,
    Status: constants.UserStatusActive,
    EmailVerifiedAt: &now,
    CreatedAt: now,
    UpdatedAt: now,
}
```

7. Create identity:

```go
identity := &externalidentitydomain.Identity{
    UserID: user.ID,
    Provider: constants.UserOAuthProviderSub2,
    ProviderUserID: verified.ID,
    Username: verified.Email,
    AuthAt: &now,
    CreatedAt: now,
    UpdatedAt: now,
}
```

8. Issue Dujiao JWT with `IssueUserChallengeTokenForSource` if Dujiao user has TOTP, otherwise `GenerateUserJWT`.

**Step 7: Wire the verifier**

In `/Users/alien/Documents/独角兽/api/internal/app/container/services_foundation.go`, build the HTTP verifier from config and call:

```go
c.UserAuthService.SetSub2CredentialVerifier(sub2auth.NewClient(c.Config.Sub2Login))
```

If disabled or not configured, inject a disabled no-op verifier or nil.

**Step 8: Run tests**

Run:

```bash
cd /Users/alien/Documents/独角兽/api
go test ./internal/modules/identity/userauth/... ./internal/app/container -run 'Sub2|LoginStep1|Container' -count=1
```

Expected: PASS.

**Step 9: Commit**

```bash
cd /Users/alien/Documents/独角兽/api
git add internal/constants/constants.go internal/modules/identity/userauth/application/service.go internal/modules/identity/userauth/application/ports.go internal/modules/identity/userauth/integrationtest/sub2_login_test.go internal/app/container/services_foundation.go
git commit -m "feat: support sub2 password fallback login"
```

## Task 5: Add Cross-Service Contract Tests

**Files:**
- Test: `/Users/alien/Documents/sub2/backend/internal/server/routes/internal_routes_test.go`
- Test: `/Users/alien/Documents/独角兽/api/internal/modules/identity/userauth/infrastructure/sub2auth/client_test.go`

**Step 1: Add response compatibility tests**

Ensure Dujiao's client parses exactly the JSON shape produced by sub2's handler:

```json
{
  "success": true,
  "data": {
    "user": {
      "id": 123,
      "email": "user@example.com",
      "username": "Alice",
      "status": "active"
    }
  }
}
```

**Step 2: Run both contract tests**

Run:

```bash
cd /Users/alien/Documents/sub2/backend
go test ./internal/server/routes ./internal/handler -run 'Dujiao|Internal' -count=1

cd /Users/alien/Documents/独角兽/api
go test ./internal/modules/identity/userauth/infrastructure/sub2auth -run 'Contract|Response|Sub2' -count=1
```

Expected: PASS.

**Step 3: Commit**

Commit in the repository where files changed. If both changed, make one commit in each repository with a clear message.

## Task 6: Local Manual Smoke Test

**Files:**
- No source changes expected.

**Step 1: Configure local sub2**

Set in sub2 config or env:

```yaml
dujiao_login:
  enabled: true
  shared_secret: "local-dev-shared-secret-at-least-32-bytes"
```

**Step 2: Configure local Dujiao**

Set in Dujiao config:

```yaml
sub2_login:
  enabled: true
  base_url: "http://127.0.0.1:18081"
  shared_secret: "local-dev-shared-secret-at-least-32-bytes"
  timeout_seconds: 3
```

**Step 3: Start both backends**

Use the existing sub2 backend local port if already running. Start Dujiao with its normal local command from `/Users/alien/Documents/独角兽/api`.

**Step 4: Smoke cases**

- Dujiao local user logs in with Dujiao password.
- sub2-only user logs in on Dujiao login page.
- Dujiao creates a `user_oauth_identities` row with provider `sub2`.
- A wrong password returns the normal login error.
- A disabled sub2 user cannot log in.
- A sub2 user with 2FA enabled cannot log in through Dujiao until a separate 2FA bridge is designed.

## Task 7: Full Verification and Final Commit Hygiene

**Files:**
- No source changes expected.

**Step 1: Run sub2 focused tests**

```bash
cd /Users/alien/Documents/sub2/backend
go test ./internal/service ./internal/handler ./internal/server/routes ./internal/config -run 'Dujiao|ExternalCredential|Internal|Config' -count=1
```

Expected: PASS.

**Step 2: Run Dujiao focused tests**

```bash
cd /Users/alien/Documents/独角兽/api
go test ./internal/modules/identity/userauth/... ./internal/config ./internal/app/container -run 'Sub2|LoginStep1|Config|Container' -count=1
```

Expected: PASS.

**Step 3: Run broader backend tests if time permits**

```bash
cd /Users/alien/Documents/sub2/backend
go test ./internal/service ./internal/handler ./internal/server/routes ./internal/config

cd /Users/alien/Documents/独角兽/api
go test ./internal/modules/identity/... ./internal/config ./internal/app/...
```

Expected: PASS.

**Step 4: Check staged files exactly**

```bash
cd /Users/alien/Documents/sub2
git status --short

cd /Users/alien/Documents/独角兽/api
git status --short
```

Do not stage unrelated pre-existing local changes.

**Step 5: Final notes**

Record:

- sub2 commit hashes.
- Dujiao commit hashes.
- local test commands and results.
- whether manual smoke was completed.
