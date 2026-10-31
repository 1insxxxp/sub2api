# User Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an individually authorized support page that grants one lifetime 15-credit trial and shows balance plus ten safe usage/error records.

**Architecture:** A dedicated service/repository owns a transactional, durable trial grant and explicit staff authorization. Dedicated DTOs contain only fixed safe fields; the new page never consumes broad admin account, usage, or Ops responses. Historical used admin_balance +15 rows are recognized as already granted. Existing app and deployment stay intact.

**Tech Stack:** Go/Gin, PostgreSQL, existing wallet/cache services, Vue 3/Pinia, Vitest.

---

### Task 1: Transactional support repository and safe service

Files: create `backend/internal/service/user_support.go`, `backend/internal/repository/user_support_repo.go`, focused unit/integration tests, and `backend/migrations/255_user_support.sql` plus migration tests. Register repository/service providers only after integration.

1. Write failing tests for grant uniqueness, historic admin +15, rollback and wallet invariants; run focused tests and record expected failures.
2. Add access and permanent trial-grant tables. Implement exact account lookup and fresh active admin or explicitly enabled sub_admin permission checks. Lock the target wallet, detect lifetime claim/history, and atomically credit gift balance + write ledger + permanent claim. Do not call ordinary balance adjustment or redeem services that accrue affiliate rebate.
3. Add safe scalar query DTOs and fixed newest-first LIMIT 10 queries scoped to user_id. Use only known public requested models, no internal fallback. Normalize error phase/type to fixed public categories. No arbitrary internal strings in serialized DTOs.
4. Add service orchestration/cache invalidation and explicit per-panel failure state after committed grant. Assert unknown failures cannot return error text. Run unit and real isolated PostgreSQL integration tests.

### Task 2: Protected handlers and runtime wiring

Files: create `backend/internal/handler/admin/user_support_handler.go` and tests; update admin route registration, handler structs/providers, repository/service providers and generated server wire; document endpoints in AGENTS.md.

1. First add failing tests for access denial/revocation, exact lookup payload, fixed output schemas and errors, unsupported amount/pagination, permissions management by main admin only.
2. Register `GET /admin/user-support/access`, `POST /admin/user-support/lookup`, and main-admin `GET/PUT /admin/users/:id/support-access`.
3. Match service contracts, require actor identity, no raw `ErrorFrom` on unexpected support errors, apply panel rate limiting/audit/no-store headers. Do not reuse broad admin data endpoints.
4. Run route/handler/middleware tests, regenerate wire, compile server.

### Task 3: Support page, access control and UI

Files: create `frontend/src/api/admin/userSupport.ts`, dedicated access store/helper and `frontend/src/components/admin/workbench/UserSupportPanel.vue`; modify router/sidebar and user edit modal; add locale strings and focused tests.

1. Write and observe failing Vitest tests for unauthorized route, explicitly submitted grant, ten-row rendering, no arbitrary server text, repeated grant status and independent list failure.
2. Add safe typed client models matching API, no raw HTML/error string rendering. Render compact account lookup, gift/balance results, usage and fixed-category errors with mobile layout and dark mode.
3. Embed the support tab in AdminWorkbenchView for main admins and explicitly authorized secondary admins; ordinary users remain denied at every layer. Remove standalone navigation and ordinary-user auth exceptions. Add a main-admin editor with three independent capabilities: commission, affiliate leaderboard and support. Guard every report/leaderboard endpoint, default new secondary admins to no sensitive capability, and preserve existing secondary-admin visibility in migration 256 without changing shared commission computation.
4. Run targeted UI/i18n tests, typecheck, lint changed files, production build; render desktop/mobile/light/dark with isolated fixtures.

### Task 4: Integration and independent review

1. Spec review against design, then security/quality review focusing on non-leakage and money invariants. Fix all findings and rerun affected tests.
2. Run appropriate broader backend tests and frontend checks; verify git diff and gofmt.
3. Report verified local result and material limits. No production financial mutations, push, upstream merge or deployment in this implementation phase.

### Local-preview correction

Only admin/sub_admin may use the workbench. Support lives in its own workbench tab. Ordinary users must be rejected even with stale support permissions. Add migration 256 for explicit commission and leaderboard access, preserve existing secondary-admin permissions, and default newly promoted accounts to false. Verify the corrected local app with an authorized secondary-admin fixture, an ordinary user carrying stale access, and both denied sensitive report endpoints.
