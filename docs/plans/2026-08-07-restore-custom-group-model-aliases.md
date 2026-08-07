# Restore Custom Group Model Aliases Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restore every previously completed custom-group model alias capability onto the current production branch without importing unrelated commits from the old feature branch.

**Architecture:** Cherry-pick the nine contiguous feature/fix commits that implement validation, uniqueness, alias resolution, request rewriting, usage attribution, frontend editing, and Gemini/multipart compatibility. Resolve conflicts against the newer pricing-admission and dashboard code in favor of preserving both behaviors. Validate migration safety, backend routing/billing tests, frontend component tests, and production builds before integrating the result into the release branch.

**Tech Stack:** Go, PostgreSQL migrations, Vue 3, TypeScript, Vitest, pnpm, Git.

---

### Task 1: Restore the audited alias commit chain

**Files:**
- Modify: `backend/internal/service/user_custom_group_service.go`
- Modify: `backend/internal/service/api_key_service.go`
- Modify: `backend/internal/service/composite_platform.go`
- Modify: `backend/internal/server/routes/gateway.go`
- Modify: `backend/internal/handler/gemini_v1beta_handler.go`
- Modify: `backend/internal/service/gateway_usage_billing.go`
- Modify: `backend/internal/service/openai_gateway_usage.go`
- Create: `backend/migrations/195_user_custom_group_call_name_casefold_notx.sql`
- Modify: `frontend/src/components/custom-groups/CustomGroupsManager.vue`
- Create: `frontend/src/components/custom-groups/modelAliases.ts`
- Test: associated custom-group, gateway, handler, migration, and frontend specs from commits `737ea490a` through `2d9798d9f`

**Step 1:** Cherry-pick, in order, `737ea490a f1aa3a94c 803464cb3 f33831580 578ef2ef7 174f51b83 4ec6f91c3 a22b79ca5 2d9798d9f`.

**Step 2:** Resolve conflicts minimally, retaining current-branch fail-closed pricing and newer upstream behavior.

**Step 3:** Run `git diff --check` and inspect the resulting commit/file boundary to ensure unrelated commits (`e2652eb85`, `e3e033bb3`, `7d38e6712`, `c123caddd`) were not imported.

### Task 2: Verify backend behavior and migration safety

**Step 1:** Run focused custom-group service and API-key tests.

**Step 2:** Run gateway route tests covering JSON, multipart, Gemini, and alias dispatch.

**Step 3:** Run usage billing tests covering requested alias preservation and concrete-model billing.

**Step 4:** Run migration tests and compile the backend server.

### Task 3: Verify frontend behavior

**Step 1:** Install locked frontend dependencies if the isolated worktree has no local modules.

**Step 2:** Run the custom-group component and alias helper specs.

**Step 3:** Run the production frontend build and confirm the emitted bundle contains the alias editor text.

### Task 4: Integrate and prepare release

**Step 1:** Merge the verified restoration branch into `codex/server-700d295e` without including the user's untracked root `package.json` files.

**Step 2:** Push the updated release branch.

**Step 3:** Build a new green/blue image, apply the online-safe migration through normal startup, verify health and alias UI, then switch traffic with the previous healthy slot retained for rollback.
