# Commission Exclusions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove request-level workbench browsing and support prospective customer exclusions from commission reports without changing billing.

**Architecture:** Persist immutable exclusion intervals and filter recent usage by its creation timestamp. Extend existing daily group rollups with a commission-only excluded-cost field, preserving the original cost for other reports. Admin-only paginated list/add/remove APIs back a compact management dialog; report data and hidden tab history load only when needed.

**Tech Stack:** Go, PostgreSQL, Gin, Vue 3, TypeScript, Vitest, Go integration tests.

---

### Task 1: Exclusion persistence and reporting

Files: `backend/migrations/249_sub_admin_commission_exclusions.sql`, `backend/internal/repository/sub_admin_commission_repo.go`, new `sub_admin_commission_exclusions.go`, `custom_group_usage_rollup_repo.go`, related repository and migration tests.

1. Add failing SQL/integration tests for interval boundaries, idempotent add/remove, list pagination, and unchanged unrelated group totals.
2. Run `go test -tags=integration ./internal/repository -run 'SubAdminCommission|GroupUsageRollup' -count=1` and capture failures for missing behavior.
3. Create table `sub_admin_commission_exclusions(id,user_id,started_at,ended_at,created_by)` with partial active unique index; preserve all closed intervals. Add `commission_excluded_cost DECIMAL(20,10) NOT NULL DEFAULT 0` to group daily rollups.
4. Implement `ListExclusions(ctx, pagination)`, `AddExclusion(ctx,userID,operatorID)`, `RemoveExclusion(ctx,userID)`; database time determines effect. Serialize per customer; repeat add returns existing interval without resetting start.
5. Filter live report rows by interval; compute excluded amount beside original group total in background rollup, subtract only in commission report historical queries. No source log backfill for prior usage.
6. Run affected unit/integration tests, including existing fixtures updated for new schema.

### Task 2: Service, APIs and permission boundaries

Files: `backend/internal/service/sub_admin_commission.go`, `backend/internal/handler/admin/sub_admin_commission_handler.go`, `backend/internal/server/routes/admin.go`, related tests, `AGENTS.md` API documentation.

1. Add failing handler/service/route tests covering validation, admin-only mutations and removed log route.
2. Expose `GET /api/v1/admin/sub-admin-commissions/exclusions`, `POST` with `{user_id}`, `DELETE /exclusions/:user_id`. DTO: `id,user_id,user_email,username,started_at`; paginated response uses existing conventions.
3. Retire commission-specific request-log route/call path while retaining normal usage management.
4. Document prospective `[start,end)` semantics and three new endpoints with placeholders for credentials.
5. Run `go test ./internal/service ./internal/handler/admin ./internal/server/routes -run 'SubAdminCommission|Workbench' -count=1`.

### Task 3: Workbench UI and fetching

Files: `frontend/src/api/admin/subAdminCommission.ts`, `frontend/src/components/admin/workbench/SubAdminCommissionManagement.vue`, new exclusions component, `SubAdminCommissionPanel.vue`, `SubAdminCommissionCalendar.vue`, `SubAdminCommissionDayDrawer.vue`, `frontend/src/views/admin/AdminWorkbenchView.vue`, all locale common files and related tests.

1. Add failing tests for absent log fetching, lazy transfer history, admin-only exclusion editor, add/remove/list failures and report refresh.
2. Remove drawer log UI and API use; preserve day group summary and keyboard behavior, ignore stale day responses.
3. Add paginated on-demand customer exclusions manager. Use debounced existing user search, stable user IDs, server effect times; explain prospective-only behavior.
4. Add management change events to refresh summary/calendar after successful changes and dismiss stale day dialog.
5. Load generated-code history only when balance-transfer tab becomes active, preserving subsequent filtering and refresh behavior.
6. Run targeted Vitest, frontend lint, typecheck, i18n completeness and build.

### Task 4: Review and integration

1. Independently review spec compliance, then code quality and accounting/permission invariants.
2. Run backend unit suite and relevant PostgreSQL integration tests, frontend targeted suites, lint/typechecks/build. Existing baseline has two dialog transition timing test failures; fix deterministic test waits where applicable and distinguish baseline from regressions.
3. Inspect final diff for external embedded page, billing or upstream changes (none allowed). Commit verified work locally and integrate into local `dev`; do not push or deploy without a new explicit request.
