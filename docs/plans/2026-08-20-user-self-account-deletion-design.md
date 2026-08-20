# User Self Account Deletion Design

## Context

Users can edit their profile, change their password, unlink sign-in providers, delete API keys, passkeys, notification emails, and generated redeem codes. They cannot currently delete or cancel their own account. Administrators can delete non-admin users through the admin user service, which soft-deletes the user, deletes that user's API keys, and invalidates API key auth cache entries.

## Approaches

### Recommended: Dedicated User-Service Deletion Flow

Add a user-facing account deletion method in `UserService`. The flow verifies the current password, rejects admin self-deletion, lists and deletes the user's API keys, soft-deletes the user through the existing repository path, invalidates API key auth cache, and lets the handler revoke all refresh/access token sessions through `AuthService`.

This keeps user-specific safeguards out of the admin service, reuses existing repository soft-delete behavior, and gives the frontend a clear endpoint for self-service cancellation.

### Alternative: Call Admin Delete From User Handler

The user handler could call the admin service deletion path after checking the password. This would reuse more code but would couple user settings to admin operations and make it easier to skip user-specific confirmation requirements later.

### Alternative: Only Hide/Disable The Account

The endpoint could set `status=disabled` without soft-deleting the user. This is simpler but does not match the existing deletion semantics and leaves credentials and API keys around unless extra cleanup is added.

## Design

### Backend

Add `DELETE /api/v1/user/account` under authenticated user routes. The request body contains `password`.

The handler:
- reads the authenticated subject,
- validates the JSON body,
- calls `UserService.DeleteOwnAccount`,
- calls `AuthService.RevokeAllUserTokens` when auth service is configured,
- returns a success message.

The service:
- loads the current user,
- verifies the password with the existing password hash helper,
- rejects `role == "admin"`,
- lists the user's API keys in pages,
- deletes each key with `DeleteWithAudit`,
- soft-deletes the user with `UserRepository.Delete`,
- invalidates API key auth cache by key and by user.

The API key cleanup should be optional in the service wiring so existing tests and minimal service construction keep working. Production wiring will attach the API key repository to the user service.

### Frontend

Add `deleteOwnAccount(password)` to `frontend/src/api/user.ts`.

In `ProfileView.vue`, add a danger-zone section to the security column. The section asks for the current password in a modal/confirmation panel, calls the new API, clears auth state, and sends the user to `/login`.

Use the existing profile styling and translation system instead of adding a new route. The interface should be explicit and restrained: red danger styling, clear irreversible copy, password confirmation, loading and error states.

### Data And Safety

Deletion remains a soft delete through `deleted_at`. Balances, orders, usage logs, and historical records remain for audit and accounting. Users lose access immediately because the row is soft-deleted from normal lookups, refresh tokens are revoked, and owned API keys are deleted.

Admin accounts cannot self-delete. Admin deletion remains available from the admin panel.

### Testing

Backend:
- service test: wrong password returns `PASSWORD_INCORRECT` and does not delete,
- service test: admin user cannot self-delete,
- service test: normal user deletion deletes API keys and invalidates caches,
- handler/route test: authenticated `DELETE /user/account` exists and requires a password.

Frontend:
- API test: `deleteOwnAccount` sends `DELETE /user/account` with password body,
- profile view test: danger-zone deletion calls API, logs out, and redirects on success.
