# Registration Abuse Control Design

## Goal

Reduce fake-user batches without requiring invitation-code registration. Keep normal email registration available, keep the production service online, and make abuse evidence queryable from the database.

## Approach

Use backend-first controls:

- Keep `invitation_code_enabled` out of the protection path.
- Persist source metadata on users: registration IP, registration user agent, last login IP, and last login user agent.
- Store a global auth IP blacklist in settings and enforce it before high-risk auth actions.
- Reuse the existing IP utility so single IP and CIDR rules behave like current API-key ACLs.

## Data Flow

Auth handlers derive the real client IP through `ip.GetClientIP(c)` and trim the request `User-Agent`. Registration passes that metadata into `AuthService`, then `UserRepository.Create` writes it to `users`. Login updates last-login metadata after password and status checks pass.

Before sending verification codes, registering, logging in, or completing OAuth account creation, handlers call the blacklist check. Blocked clients receive a forbidden response and never reach email sending or account creation.

## Admin Surface

The first phase adds small admin APIs for reading and replacing the global auth IP blacklist. A richer UI can be added later under the risk-control/admin settings page, but the backend API is enough for deployment and server-side operation.

## Testing

Add unit tests for:

- IP blacklist parsing and matching.
- Auth handler rejection when `CF-Connecting-IP` matches a blacklisted IP.
- Email registration stores source IP and user agent.
- Login updates last-login source metadata.
