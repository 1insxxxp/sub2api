# Dujiao Sub2 Shared Password Login Design

Date: 2026-08-23
Status: Approved by product owner

## Goal

Allow a user to sign in to Dujiao-Next with the same email and password they use in sub2, while keeping Dujiao's existing local email/password login available.

## Non-Goals

- Do not share balances.
- Do not share orders, recharge records, wallet records, subscriptions, or API keys.
- Do not copy password hashes between systems.
- Do not issue or reuse sub2 JWTs inside Dujiao.
- Do not replace Dujiao's local login, Telegram login, Google login, or 2FA behavior.

## Options Considered

### Option A: Shared database tables

Both systems read/write the same `users` table. This is risky because the schemas, JWT claims, password lifecycle, 2FA, soft delete behavior, and migrations differ between the two systems.

Decision: rejected.

### Option B: Dujiao calls the public sub2 login endpoint

Dujiao would submit the user's email and password to `/api/v1/auth/login`. This is simpler but produces normal sub2 login side effects, may trigger captcha/backend-mode behavior, may create sub2 tokens that Dujiao does not need, and couples Dujiao to sub2 frontend-login semantics.

Decision: rejected.

### Option C: Internal credential verification endpoint

sub2 exposes a server-to-server credential verification endpoint protected by a shared secret. Dujiao keeps its local login first. If local login fails, Dujiao verifies the same email/password against sub2. On success, Dujiao binds or creates a local user and issues its own session token.

Decision: selected.

## Architecture

sub2 remains the authority for sub2 passwords. Dujiao remains the authority for Dujiao sessions, orders, wallet, and storefront activity.

The integration uses a new server-to-server endpoint in sub2:

`POST /api/v1/internal/dujiao/auth/verify`

The endpoint requires a shared secret header and accepts only the minimum request body:

```json
{
  "email": "user@example.com",
  "password": "plain-text password from login form"
}
```

The endpoint returns no sub2 token and no financial data:

```json
{
  "ok": true,
  "user": {
    "id": 123,
    "email": "user@example.com",
    "username": "Alice",
    "status": "active"
  }
}
```

If credentials are invalid, the response must not reveal whether the email exists.

## Dujiao Login Flow

1. User submits Dujiao login form with email and password.
2. Dujiao tries its existing local `LoginStep1` flow.
3. If local login succeeds, behavior is unchanged.
4. If local login fails with invalid credentials, Dujiao calls the sub2 internal verification endpoint when the integration is enabled.
5. If sub2 verification fails, Dujiao returns the normal invalid-login error.
6. If sub2 verification succeeds, Dujiao finds or creates a local user:
   - Prefer an existing `user_oauth_identities` row where `provider = "sub2"` and `provider_user_id = sub2 user id`.
   - If not found, look up an active, non-deleted Dujiao user with the same email.
   - If found, bind that Dujiao user to the sub2 identity.
   - If not found, create a Dujiao user with the sub2 email and a random local password hash.
7. Dujiao issues its own JWT using the existing login response shape.

## Binding Rules

- Provider name: `sub2`.
- Provider user ID: decimal string of `sub2.users.id`.
- Auto-bind is allowed only to an active, non-deleted Dujiao user with the same normalized email.
- Disabled or deleted Dujiao users must not be auto-bound.
- If a sub2 identity is already bound to a different Dujiao user, login fails closed.
- If an email collision is ambiguous, login fails closed and logs an internal reason.

## Security

- The shared secret must be configured separately in sub2 and Dujiao.
- The endpoint should be used only over localhost/private network or HTTPS.
- Dujiao must never log the submitted password.
- sub2 must return a generic 401 for invalid credentials.
- sub2 must rate-limit or otherwise protect the internal verification endpoint, even though it is server-to-server.
- sub2 2FA accounts should fail closed until a separate sub2 2FA bridge is explicitly designed.

## Configuration

sub2 should add an internal login integration config:

```yaml
dujiao_login:
  enabled: true
  shared_secret: "..."
```

Dujiao should add:

```yaml
sub2_login:
  enabled: true
  base_url: "http://127.0.0.1:18081"
  shared_secret: "..."
  timeout_seconds: 3
```

## Testing

sub2 tests:

- Invalid secret returns 401.
- Invalid credentials return 401 without revealing existence.
- Disabled user is rejected.
- User with enabled 2FA is rejected.
- Active email/password user returns only safe profile fields.

Dujiao tests:

- Existing local login still wins and does not call sub2.
- Local invalid credentials fall back to sub2 when enabled.
- sub2 verified identity binds to existing active same-email user.
- sub2 verified identity creates a Dujiao user when no same-email user exists.
- Disabled/deleted same-email Dujiao user is not auto-bound.
- sub2 outage returns normal invalid-login behavior to the user.

## Rollout

1. Deploy sub2 internal verification endpoint with integration disabled until secret is configured.
2. Deploy Dujiao fallback integration with `sub2_login.enabled = false`.
3. Configure shared secret and base URL on both servers.
4. Enable Dujiao fallback login.
5. Test a known sub2 account that does not exist in Dujiao.
6. Test an existing Dujiao local account to ensure local login still works first.
