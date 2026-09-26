# Ops SLA Grouped Locator Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a grouped SLA failure locator beside the SLA card details action, using the same dense group/model/account table as the upstream error locator while keeping final SLA semantics accurate.

**Architecture:** Add a dedicated repository/service/handler summary path for client-visible, non-business-limited final failures. Return the existing nested summary contract so the frontend can share the upstream summary table presentation, then wire a dedicated SLA modal into `OpsDashboard` and `OpsDashboardHeader`.

**Tech Stack:** Go, PostgreSQL, Gin, Vue 3 `<script setup>`, TypeScript, Vitest, Go `testing`/`sqlmock`, existing `BaseDialog` and admin ops components.

---

### Task 1: Extend the ops summary contract and repository interface

**Files:**
- Modify: `backend/internal/service/ops_models.go`
- Modify: `backend/internal/service/ops_port.go`
- Modify: `backend/internal/service/ops_repo_mock_test.go`
- Test: `backend/internal/service/ops_sla_summary_test.go`

**Step 1: Write the failing service contract tests**

Add tests for the new `GetSLAErrorSummary` service method: monitoring disabled returns `ErrOpsDisabled`, a nil repository returns an empty normalized summary, and a repository error is passed through unchanged.

**Step 2: Run the focused tests to verify they fail**

Run: `go test ./backend/internal/service -run 'TestOpsService.*SLAErrorSummary'`

Expected: compile failure because the repository method and service method do not exist.

**Step 3: Add the interface method and use the existing summary response shape**

Add `GetSLAErrorSummary(ctx, filter)` to `OpsRepository`, add the matching function hook to the test mock, and expose the existing `OpsUpstreamErrorSummary` nested contract as the response type for the SLA endpoint. Add a service method that applies the monitoring guard, nil-repository empty result, and repository delegation.

**Step 4: Run the focused tests**

Run: `go test ./backend/internal/service -run 'TestOpsService.*SLAErrorSummary'`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/ops_models.go backend/internal/service/ops_port.go backend/internal/service/ops_repo_mock_test.go backend/internal/service/ops_sla_summary_test.go
git commit -m "feat: add SLA summary service contract"
```

### Task 2: Implement final SLA failure aggregation in the repository

**Files:**
- Modify: `backend/internal/repository/ops_repo.go`
- Create: `backend/internal/repository/ops_sla_error_summary_test.go`

**Step 1: Write the failing repository aggregation tests**

Using the existing SQL mock style, cover: final status `>=400` is included, business-limited rows are excluded, recovered provider rows with a successful final status are excluded, rows aggregate by group/model/account/reason, counts and latest timestamps sort descending, and empty results return initialized arrays.

**Step 2: Run the focused tests to verify they fail**

Run: `go test ./backend/internal/repository -run 'TestGetSLAErrorSummary'`

Expected: compile failure because `GetSLAErrorSummary` is not implemented.

**Step 3: Implement the repository query and aggregation**

Add a dedicated query next to `GetUpstreamErrorSummary`. Reuse the shared time/platform/group/model/status/query predicates where safe, but force the SLA predicates `COALESCE(e.status_code,0) >= 400` and `COALESCE(e.is_business_limited,false) = false`; do not opt into recovered provider rows and do not add provider-owner filters. Group by configured group, requested/effective model, account, final status, sanitized error message and error type. Build the same bounded nested summary, representative error IDs, counts and sort order used by the upstream summary helpers.

**Step 4: Run focused and existing summary tests**

Run: `go test ./backend/internal/repository -run 'TestGet(SLA|Upstream)ErrorSummary'`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/repository/ops_repo.go backend/internal/repository/ops_sla_error_summary_test.go
git commit -m "feat: aggregate SLA failures by group"
```

### Task 3: Add the admin endpoint and route parsing

**Files:**
- Modify: `backend/internal/handler/admin/ops_handler.go`
- Modify: `backend/internal/server/routes/admin.go`
- Create: `backend/internal/handler/admin/ops_sla_error_summary_test.go`

**Step 1: Write the failing handler tests**

Assert that `GET /admin/ops/request-errors/summary` parses time, platform, group, model, status and query filters, forces the SLA filter semantics, returns the summary payload, rejects invalid IDs/status parameters, and maps disabled monitoring/repository errors as existing ops endpoints do.

**Step 2: Run the focused tests to verify they fail**

Run: `go test ./backend/internal/handler/admin -run 'Test.*SLA.*Summary'`

Expected: compile or route failure because the handler and route are absent.

**Step 3: Implement the handler and route**

Add a parser dedicated to request-error summaries so provider recovery options cannot leak into the SLA scope. Add `SummarySLAErrors` to `OpsHandler` and register it at `/admin/ops/request-errors/summary`; require monitoring before querying and return the existing success/error response envelopes.

**Step 4: Run focused handler tests and route tests**

Run: `go test ./backend/internal/handler/admin ./backend/internal/server -run 'Test.*(SLA|Request).*Summary'`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/admin/ops_handler.go backend/internal/handler/admin/ops_sla_error_summary_test.go backend/internal/server/routes/admin.go
git commit -m "feat: expose SLA summary endpoint"
```

### Task 4: Add frontend API types and summary request tests

**Files:**
- Modify: `frontend/src/api/admin/ops.ts`
- Modify: `frontend/src/api/admin/__tests__/opsApi.spec.ts`

**Step 1: Write the failing API test**

Add a test that `getSLAErrorSummary` strips pagination fields and calls `/admin/ops/request-errors/summary` with the current time, platform, group, model and error filters.

**Step 2: Run the focused test to verify it fails**

Run: `cd frontend && pnpm vitest run src/api/admin/__tests__/opsApi.spec.ts -t 'SLA'`

Expected: FAIL because the API method is absent.

**Step 3: Implement the typed API method**

Add `OpsSLAErrorSummary` as an alias or shared structural type for the existing nested summary contract and implement `getSLAErrorSummary` with the same pagination stripping behavior as `getUpstreamErrorSummary`.

**Step 4: Run the focused test**

Run: `cd frontend && pnpm vitest run src/api/admin/__tests__/opsApi.spec.ts -t 'SLA'`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/ops.ts frontend/src/api/admin/__tests__/opsApi.spec.ts
git commit -m "feat: add SLA summary API client"
```

### Task 5: Reuse the summary table in an SLA modal

**Files:**
- Create: `frontend/src/views/admin/ops/components/OpsSlaErrorSummaryModal.vue`
- Modify: `frontend/src/views/admin/ops/components/OpsUpstreamErrorSummaryModal.vue`
- Create: `frontend/src/views/admin/ops/components/__tests__/OpsSlaErrorSummaryModal.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/ops.ts`
- Modify: `frontend/src/i18n/locales/en/admin/ops.ts`

**Step 1: Write failing component tests**

Cover the SLA modal loading, empty and error states, default expanded group sections, model/account/reason table rows, SLA-specific title and explanation, and representative error detail event.

**Step 2: Run the focused component test to verify it fails**

Run: `cd frontend && pnpm vitest run src/views/admin/ops/components/__tests__/OpsSlaErrorSummaryModal.spec.ts`

Expected: FAIL because the modal is absent.

**Step 3: Extract or share the existing dense summary table rendering**

Move the common summary header, bounded group sections, flattened rows, status styling, timestamp formatting and detail action into a small presentational component or shared composable. Keep each modal responsible for its own API loading and title/description, with the SLA modal calling `getSLAErrorSummary` and the upstream modal retaining its existing endpoint.

**Step 4: Implement SLA-specific copy and accessibility**

Add translations for “SLA 分组定位”, the final-failure-only explanation, loading, retry and empty states in Chinese and English. Preserve the existing neutral dialog appearance, keyboard-accessible buttons and mobile horizontal table behavior.

**Step 5: Run focused component tests**

Run: `cd frontend && pnpm vitest run src/views/admin/ops/components/__tests__/OpsSlaErrorSummaryModal.spec.ts src/views/admin/ops/components/__tests__/OpsUpstreamErrorSummaryModal.spec.ts`

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/views/admin/ops/components frontend/src/i18n/locales/zh/admin/ops.ts frontend/src/i18n/locales/en/admin/ops.ts
git commit -m "feat: add SLA grouped locator modal"
```

### Task 6: Wire the SLA locator into the dashboard

**Files:**
- Modify: `frontend/src/views/admin/ops/components/OpsDashboardHeader.vue`
- Modify: `frontend/src/views/admin/ops/OpsDashboard.vue`
- Modify: `frontend/src/views/admin/ops/components/__tests__/OpsErrorScopeCharts.spec.ts`

**Step 1: Write the failing dashboard interaction test**

Assert that the SLA card renders “分组定位” beside “明细” outside fullscreen mode, emits a dedicated event, passes current time/platform/group filters into the SLA modal, and closes the modal before opening a normal detail modal.

**Step 2: Run the focused test to verify it fails**

Run: `cd frontend && pnpm vitest run src/views/admin/ops/components/__tests__/OpsErrorScopeCharts.spec.ts -t 'SLA'`

Expected: FAIL because the button/event/modal wiring is absent.

**Step 3: Implement the dashboard wiring**

Add `openSlaSummary` emit to `OpsDashboardHeader`, place the button next to the existing SLA details action, add modal state and filter construction in `OpsDashboard`, mount `OpsSlaErrorSummaryModal`, and route representative IDs to `OpsErrorDetailModal` with `errorDetailsType='request'`.

**Step 4: Run focused dashboard tests**

Run: `cd frontend && pnpm vitest run src/views/admin/ops/components/__tests__/OpsErrorScopeCharts.spec.ts src/views/admin/ops/components/__tests__/OpsSlaErrorSummaryModal.spec.ts`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/ops/OpsDashboard.vue frontend/src/views/admin/ops/components/OpsDashboardHeader.vue frontend/src/views/admin/ops/components/__tests__/OpsErrorScopeCharts.spec.ts
git commit -m "feat: add SLA grouped locator entry"
```

### Task 7: Run the complete validation suite

**Files:**
- No source changes expected.

**Step 1: Run backend tests**

Run: `go test ./backend/internal/service ./backend/internal/repository ./backend/internal/handler/admin ./backend/internal/server`

Expected: PASS.

**Step 2: Run frontend tests and build**

Run: `cd frontend && pnpm vitest run src/api/admin/__tests__/opsApi.spec.ts src/views/admin/ops/components && pnpm build`

Expected: PASS with a production bundle generated successfully.

**Step 3: Inspect the final diff and status**

Run: `git diff --check && git status --short`

Expected: no whitespace errors; only the intended commits are present.

**Step 4: Commit any test-only corrections**

```bash
git add <corrected-files>
git commit -m "test: verify SLA grouped locator"
```
