# Local Release Blue-Green Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Verify the local Sub2API tree, commit and push the intended responsive check-in change, then build and stage the resulting revision on the server using a health-gated blue-green deployment.

**Architecture:** Keep the currently active server container serving traffic while validating the local revision. Commit only the intended tracked frontend change and its test, push the current branch, export the exact pushed commit to a server release directory, build an immutable image tag, and start it on the inactive port. Verify the inactive color and record the cutover commands, but do not switch Nginx as part of this preparation task; retain the old color for rollback.

**Tech Stack:** Git, Vue 3, TypeScript, Vitest, Vite, Go, Docker, Nginx, PostgreSQL, Redis.

---

### Task 1: Validate the local working tree

**Files:**
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Test: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`
- Exclude: `package.json`, `package-lock.json`

1. Run the focused AppHeader test.
2. Run frontend type checking and production build.
3. Run backend tests and migration compatibility tests.
4. Run `git diff --check` and confirm only intended files are staged.

### Task 2: Commit and push the verified revision

**Files:**
- Commit: `frontend/src/components/layout/AppHeader.vue`
- Commit: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`
- Commit: `docs/plans/2026-08-03-local-release-blue-green.md`

1. Stage only the listed files.
2. Review the staged diff.
3. Commit with a focused message.
4. Push `codex/server-700d295e` to `origin`.
5. Verify the remote branch points to the new commit.

### Task 3: Stage the inactive server color

1. Determine the active Nginx upstream and choose the other local port.
2. Review every migration added since the active commit and require backward-compatible, idempotent expand-only changes.
3. Create and verify a timestamped PostgreSQL backup before starting the inactive color.
4. Export the exact pushed commit into a new `/opt/sub2api-releases/<commit>` directory.
5. Reuse the validated runtime environment without printing secrets.
6. Build an immutable Docker image tagged with the commit.
7. Start a new container on the inactive port while leaving the active container untouched.

### Task 4: Verify and prepare cutover

1. Wait for Docker health to become `healthy` with zero restarts.
2. Verify `/health` and `/` directly on the inactive port.
3. Check startup logs for migration, panic, fatal, and initialization errors.
4. Record the exact Nginx cutover and rollback targets.
5. Leave Nginx on the current active color and do not cut traffic.
6. Do not claim readiness unless all fresh checks pass.
