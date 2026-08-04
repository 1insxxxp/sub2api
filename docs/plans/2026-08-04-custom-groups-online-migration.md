# Custom Groups Online Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make migration 194 safe for a zero-downtime production rollout and deploy the resulting build through the inactive blue-green slot.

**Architecture:** The transactional migration creates schema objects and unvalidated constraints, while a separate non-transactional migration creates indexes concurrently. Production is backed up before a clean source build is staged and verified in the inactive slot, followed by an atomic Nginx cutover.

**Tech Stack:** Go migration tests, PostgreSQL 18, Docker, Nginx, SSH

---

### Task 1: Lock in online-safe migration requirements

**Files:**
- Create: `backend/migrations/user_custom_groups_migration_test.go`
- Modify: `backend/migrations/194_user_custom_model_groups.sql`
- Create: `backend/migrations/194a_user_custom_model_group_indexes_notx.sql`

1. Write a failing test requiring no `CREATE INDEX` in migration 194, `NOT VALID` on its new constraints, and concurrent idempotent index statements in migration 194a.
2. Run `go test ./migrations -run TestMigration194 -count=1` from `backend` and confirm it fails for the unsafe migration.
3. Remove index creation from 194, make the constraints unvalidated and idempotent, and add the `_notx.sql` concurrent-index migration.
4. Re-run the focused test and all migration tests.

### Task 2: Verify and publish the corrected release

**Files:**
- Include the migration files, tests, design, and plan from Task 1.

1. Run `go test ./...` from `backend`.
2. Run `pnpm typecheck && pnpm vitest run && pnpm build` from `frontend`.
3. Run `git diff --check`, commit the online-migration fix, and push `codex/server-700d295e`.

### Task 3: Back up and stage production

1. Record the active Nginx port, active image, container health, database sessions, disk, and memory.
2. Create a timestamped PostgreSQL custom-format dump and verify it with `pg_restore --list`.
3. Transfer a clean `git archive` at the pushed commit to a timestamped release directory.
4. Build a new image tagged with the release slot and commit without modifying the active container.

### Task 4: Start and verify the inactive slot

1. Start the new container on the inactive loopback port with the same runtime configuration, volumes, PostgreSQL, Redis, and Docker network as the active slot.
2. Wait for Docker health to report healthy and confirm the new migrations are recorded.
3. Verify the public settings endpoint, authentication endpoint behavior, frontend document, container logs, database locks, and resource usage directly on the inactive port.

### Task 5: Cut over and observe

1. Back up both active Nginx site files.
2. Replace every Sub2API upstream port in both sites with the inactive slot port.
3. Run `nginx -t`, reload Nginx, and verify both production domains.
4. Confirm API traffic, status codes, latency, and new-container logs after cutover.
5. Keep the old active container running for rollback and report its exact rollback port and image.
