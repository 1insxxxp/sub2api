# Zero-Downtime Blue-Green Deployment Design

## Goal

Deploy `0.1.166` to the production server without interrupting active API traffic or returning deployment-induced 5xx responses.

## Architecture

Keep the current blue container on `127.0.0.1:18100` while preparing the database and starting a green container on `127.0.0.1:18101`. Nginx continues routing `api.passionapi.com` and `direct.passionapi.com` to blue until green passes direct health and smoke tests. Both site configurations are then changed together, validated with `nginx -t`, and applied with an atomic Nginx reload. Blue remains available during an observation window so routing can be reverted immediately.

## Database Safety

Take a PostgreSQL backup before any schema change. Apply only backward-compatible migrations while blue is serving. Migration `188_allow_live_usage_request_type.sql` must create its replacement check constraint as `NOT VALID`; validating all existing `usage_logs` rows is deferred until a low-traffic maintenance operation. The new constraint still checks all new writes immediately.

## Cutover And Rollback

Green shares the existing PostgreSQL and Redis only after compatible migrations are present. Direct smoke tests cover health, public site, authentication, admin API, and a controlled model request. Cutover changes all relevant Nginx upstream references from port 18100 to 18101 and reloads Nginx without restarting its workers abruptly. Existing blue requests are allowed to drain; green failures trigger an upstream switch back to blue, without reversing additive database migrations.

## Limits

This prevents application-deployment interruption on the current host. It does not provide availability for whole-host, Docker daemon, PostgreSQL, Redis, or network failure; that requires a second host and redundant data services.
