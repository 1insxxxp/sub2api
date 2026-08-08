# Upstream Sync Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Merge the latest `upstream/main` into `dev` without losing custom production behavior or local second-development features.

**Architecture:** Preserve both histories with a normal merge commit. Use a pre-merge backup ref and three-way conflict review, treating custom billing, empty-response compensation, custom groups, account routing, mobile UI, pricing, and deployment behavior as protected areas. Verify the merged tree with targeted custom regressions followed by full backend and frontend checks.

**Tech Stack:** Git, Go 1.26, PostgreSQL migrations, Vue 3, TypeScript, Vitest, pnpm, Docker build.

---

### Task 1: Establish a recoverable baseline

**Files:**
- Inspect: repository root and all `AGENTS.md` files
- Create: backup branch at the pre-merge `dev` commit

**Steps:**

1. Confirm `dev`, `origin/dev`, tracked status, remotes, and existing untracked files.
2. Create `backup/pre-upstream-20260808-<timestamp>` at the current `dev` commit.
3. Fetch `origin` and `upstream` without changing the worktree.
4. Verify the backup ref points to the original `dev` commit.

### Task 2: Review upstream changes before merging

**Files:**
- Inspect: `git diff --stat dev...upstream/main`
- Inspect: commits in `dev..upstream/main`
- Inspect: files changed on both sides since the merge base

**Steps:**

1. Record the upstream commit range, version tags, and commit subjects.
2. Classify upstream changes by backend, frontend, migrations, generated Ent code, and deployment files.
3. Identify overlapping files with custom billing, routing, model pricing, custom groups, mobile layout, and empty-response compensation.
4. Review high-risk overlapping diffs before starting the merge.

### Task 3: Merge upstream and resolve conflicts

**Files:**
- Modify: only files reported by `git merge --no-ff upstream/main`
- Preserve: all custom functionality already reachable from `dev`

**Steps:**

1. Run `git merge --no-ff upstream/main`.
2. Enumerate every unmerged path with `git diff --name-only --diff-filter=U`.
3. For each conflict, compare merge-base, upstream, and custom versions.
4. Combine compatible behavior; do not resolve generated Ent files independently of their schema sources.
5. Search the entire worktree for unresolved conflict markers.
6. Review the staged diff against both parents and confirm protected custom files remain present.

### Task 4: Verify the merged code

**Files:**
- Test: backend packages affected by conflicts and protected custom features
- Test: frontend components affected by conflicts and protected custom features

**Steps:**

1. Run `gofmt` on conflict-resolved Go files and `git diff --check`.
2. Run targeted tests for billing, custom groups, gateway routing, pricing, empty-response compensation, and migrations.
3. Run full backend unit tests: `go test -tags=unit -count=1 ./...`.
4. Run frontend tests, typecheck, lint, and production build.
5. Confirm root `package.json` and `package-lock.json` remain untracked and untouched.

### Task 5: Finalize the merge

**Files:**
- Commit: merge result and this plan

**Steps:**

1. Inspect the final staged diff and compare protected feature entry points with the backup branch.
2. Commit the merge only after all verification commands pass.
3. Report upstream changes, conflict decisions, verification evidence, and the backup ref.
4. Do not deploy or update `master` unless separately authorized.
