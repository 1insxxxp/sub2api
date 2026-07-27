# Zero-Downtime Blue-Green Deployment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Release version `0.1.166` without interrupting active production API traffic.

**Architecture:** Make the pending schema changes backward compatible, prepare and test a green container beside the current blue container, then atomically reload Nginx for both production API domains. Preserve blue until the post-cutover observation is complete.

**Tech Stack:** Go, PostgreSQL, Redis, Docker Compose, Nginx, shell/SSH

---

### Task 1: Make migration 188 online-safe

**Files:**
- Modify: `backend/migrations/188_allow_live_usage_request_type.sql`
- Create: `backend/migrations/live_request_type_migration_test.go`

1. Add a regression test requiring `NOT VALID` on the replacement check constraint.
2. Run the focused migration test and confirm it fails before the SQL change.
3. Add `NOT VALID` to migration 188.
4. Run the focused migration test, migration package tests, and full backend tests.
5. Commit and push only the migration, test, and deployment documents.

### Task 2: Revalidate production before mutation

1. Confirm the current container, PostgreSQL, Redis, proxy, disk, memory, and listening ports.
2. Confirm production schema migrations stop at 184 and migration checksums match.
3. Confirm no conflicting green container or port exists.
4. Record the current image identifier and proxy configuration for rollback.

### Task 3: Back up production

1. Create a timestamped custom-format PostgreSQL backup on the server.
2. Verify `pg_restore --list` can read the backup.
3. Preserve/tag the current blue image.

### Task 4: Build and stage green

1. Transfer a clean tracked source archive at the pushed commit.
2. Build the `linux/amd64` production image on the server without touching blue.
3. Apply migrations through a controlled one-shot process with lock and statement timeouts.
4. Verify schema migration records and required columns/constraints.
5. Start green on `127.0.0.1:18101` using the same PostgreSQL and Redis.

### Task 5: Verify green before cutover

1. Wait for green health checks to pass.
2. Test the public page and health endpoint directly on port 18101.
3. Test login/admin endpoints and one controlled model API request.
4. Inspect green logs, database locks, CPU, memory, and error counts.

### Task 6: Cut over and observe

1. Update every application upstream in both `api.passionapi.com` and `direct.passionapi.com` Nginx site configurations from 18100 to 18101.
2. Validate with `nginx -t`, then reload Nginx atomically.
3. Verify public domains, model API traffic, status codes, latency, and logs.
4. Keep blue running while existing requests drain and observe for at least 30 minutes.
5. Switch Nginx back to 18100 immediately if error or latency thresholds regress.
6. After observation, stop blue gracefully but preserve its image and backup.
