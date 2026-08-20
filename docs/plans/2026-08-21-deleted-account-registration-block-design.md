# Deleted Account Registration Block Design

## Context

User self-deletion soft-deletes the user row. Normal user lookups exclude soft-deleted rows, so the current registration checks do not see a deleted account with the same email. That means a user can delete an account and register a fresh one with the same email, which conflicts with the new policy.

## Decision

Registration must treat deleted users as occupying their email identity forever. This applies to:
- exact normalized email matches,
- inbox aliases handled by the existing alias-dedup rules,
- registration verification-code requests,
- the repository-level registration create guard that protects against races.

The user-facing error will remain `EMAIL_EXISTS`. This avoids exposing whether the email is active, deleted, or only an alias collision.

## Alternatives

### Recommended: Include Deleted Rows Only In Registration Guards

Add a registration-only repository capability that checks exact email and alias collisions while bypassing soft-delete filters. Use it from `AuthService.existsByEmailOrAlias` when available. Update the repository create guard to run its final exact and alias checks with soft-deleted users included.

This keeps login/profile/admin normal behavior unchanged while making registration policy strict.

### Alternative: Make All Email Lookups Include Deleted Users

Changing `GetByEmail` or `ExistsByEmail` globally would block re-registration, but it risks breaking login, email binding, and normal user flows that should continue to ignore deleted accounts.

### Alternative: Add A Tombstone Table

A separate email tombstone table would make the policy explicit and durable, but it adds schema and migration work that is unnecessary because soft-deleted user rows already keep the email identity.

## Backend Design

Add a narrow service-side interface, for example:

```go
type RegistrationEmailIdentityRepository interface {
    ExistsByEmailOrAliasIncludeDeleted(ctx context.Context, email string) (bool, error)
}
```

`AuthService.existsByEmailOrAlias` should prefer that interface. Unit-test stubs can keep the existing exact/alias methods.

In the concrete user repository:
- implement `ExistsByEmailOrAliasIncludeDeleted` with `mixins.SkipSoftDelete(ctx)`,
- reuse the current exact normalized email predicate,
- reuse the current alias-dedup probes,
- make the create-time `ensureNormalizedEmailAvailableWithClient` and alias check include deleted rows when `CreateWithEmailAliasGuard` is used for registration.

`SendVerifyCode`, `SendVerifyCodeAsync`, and `Register` already call `existsByEmailOrAlias`, so they inherit the new behavior.

## Testing

Add unit tests in `backend/internal/service/auth_service_register_test.go`:
- exact deleted-email collision blocks `Register`,
- deleted alias collision blocks `Register`,
- deleted email blocks `SendVerifyCode`.

Add repository unit tests if SQL mocking is practical, or cover the concrete query behavior through existing repository tests. The key automated guard is that the service prefers the include-deleted repository method and that registration create fallback returns `EMAIL_EXISTS`.

Manual local API verification should use a unique test email:
1. register a test user,
2. delete the account through `DELETE /api/v1/user/account`,
3. try to register the same email again,
4. expect `EMAIL_EXISTS`.
