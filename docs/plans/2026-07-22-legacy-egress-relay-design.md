# Legacy Server OpenAI Egress Relay Design

## Goal

Restore OpenAI connectivity for the new production server while keeping the
new application, database, Redis, DNS, and inbound traffic unchanged.

## Design

The old server provides a private authenticated proxy endpoint. The new
server sends only application upstream traffic through that endpoint. OpenAI
therefore sees the old server IPv4, which currently returns the expected 401
response for unauthenticated API requests instead of the new IPv4's regional
403 response.

The proxy must bind to a restricted interface or firewall allow only the new
server IPv4. It must not become a public open proxy. Existing user traffic,
PostgreSQL, Redis, Nginx, and DNS remain on the new server.

## Deployment

1. Verify old-to-OpenAI and new-to-old connectivity.
2. Install and configure a minimal authenticated proxy on the old server.
3. Restrict the proxy port to the new server IPv4.
4. Register the proxy in the application and associate OpenAI accounts with it.
5. Restart only the application container if configuration requires it.

## Validation

- The proxy endpoint is unreachable from unauthorized sources.
- A request through the proxy exits as `176.122.172.176`.
- OpenAI's unauthenticated endpoint changes from regional 403 to normal 401.
- The account connection test succeeds for a known account.
- Normal application health and database connectivity remain healthy.

## Rollback

Remove account-to-proxy associations, stop and disable the proxy service on
the old server, and remove its firewall rule. No database rollback or DNS
change is required.
