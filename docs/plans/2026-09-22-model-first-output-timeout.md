# Model First Output Timeout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an administrator-configurable, model-aware semantic first-output timeout that prioritizes healthy fast upstream accounts and performs at most one serial account failover before semantic output, for all text streaming gateways.

**Architecture:** Store validated JSON settings in the existing admin settings repository with global/platform/model overrides and a 60-second process cache. Resolve a per-request first-output policy using the requested model, platform, recent account/model latency data, and configured defaults; keep image and non-streaming requests on their existing task/header timeout paths. Integrate the policy with the existing failover loop so a pre-output timeout is cancellable, sticky selection is escaped, and only one replacement attempt is made without retrying after semantic output.

**Tech Stack:** Go 1.27, Gin, existing SettingService/settings routes, PostgreSQL-backed setting repository, Vue 3/TypeScript admin settings, Vitest, Go unit tests.

---

### Task 1: Define and validate first-output policy settings

**Files:**
- Modify: `backend/internal/service/settings_view.go`
- Modify: `backend/internal/service/setting.go`
- Modify: `backend/internal/service/setting_features.go`
- Test: `backend/internal/service/model_first_output_timeout_test.go`

**Step 1: Write failing tests** for default Gemini Flash/Pro/Thinking policies, override precedence, validation, and disabled/zero semantics.

**Step 2: Run the focused Go test and verify it fails** because the policy type and service methods do not exist.

**Step 3: Implement minimal settings types and service methods** using the existing setting JSON pattern. Include enabled, target seconds, switch seconds, hard cap seconds, max switches (fixed to one for now), and global/platform/model override resolution. Reject negative values, switch greater than hard cap, and hard cap above a safe upper bound.

**Step 4: Run focused tests and then format.**

**Step 5: Commit** `feat: add configurable model first-output policies`.

### Task 2: Add admin API and configuration UI

**Files:**
- Modify: `backend/internal/handler/admin/setting_handler_runtime.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/handler/dto/settings.go` (or the existing settings DTO file)
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Test: handler tests and `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

**Step 1: Write failing handler/API and component tests** for GET/PUT, invalid ranges, platform/model rows, and save/reload behavior.

**Step 2: Verify the tests fail.**

**Step 3: Implement GET/PUT endpoints and a compact settings panel** with global defaults plus platform/model override rows. Save immediately to the settings repository; do not expose output-length limits. Explain that image generation uses its separate timeout.

**Step 4: Run backend handler tests and frontend focused tests.**

**Step 5: Commit** `feat: expose model first-output timeout settings`.

### Task 3: Integrate semantic first-output timeout with text gateway forwarding

**Files:**
- Modify: `backend/internal/service/gateway_forward.go`
- Modify: `backend/internal/service/gemini_messages_compat_service.go`
- Modify: `backend/internal/service/antigravity_gateway_claude.go` and any shared stream relay used by Gemini/Anthropic
- Modify: `backend/internal/handler/gateway_handler.go`
- Test: `backend/internal/service/model_first_output_timeout_stream_test.go`, handler failover tests

**Step 1: Write failing stream tests** proving headers, pings, metadata/message-start events do not satisfy the timer, semantic content does, cancellation occurs at the configured deadline, and the handler performs one serial switch only before semantic output.

**Step 2: Verify the tests fail.**

**Step 3: Implement a reusable pre-output guard** around the shared text streaming response path. Resolve the setting at request start, stop the timer on the first semantic content/tool result event, and return the existing `UpstreamFailoverError` with a distinct `first_output_timeout` kind when the deadline fires. Ensure the response body reader exits before the next account is selected and never retry after committed semantic output.

**Step 4: Update failover bookkeeping** to clear/escape sticky selection after this error, exclude the timed-out account, and cap this timeout reason at one account switch without changing normal error retry behavior.

**Step 5: Run focused stream/handler tests, then commit** `feat: fail over slow text upstreams before first output`.

### Task 4: Use recent account/model latency for fast-lane selection

**Files:**
- Modify: `backend/internal/service/gateway_scheduling.go`
- Modify: existing usage/TTFT observation code and repositories
- Test: `backend/internal/service/gateway_scheduling_fast_lane_test.go`

**Step 1: Write failing selection tests** for account+model recent samples, fast-lane preference over static priority, stale sticky escape, insufficient-sample fallback, and long-context bucket fallback.

**Step 2: Verify failure.**

**Step 3: Implement bounded recent statistics** (latest 100 valid semantic first-output observations per account/model), using P95/mean safety policy only for ranking and threshold adjustment. Keep configured priority as a tie-breaker and retain load/capacity filters.

**Step 4: Run scheduler tests and check for races.**

**Step 5: Commit** `feat: prefer healthy fast upstream accounts`.

### Task 5: Regression verification and release

**Files:**
- Modify: docs/settings documentation if required by existing conventions

**Step 1: Run Go focused tests, all unit tests, frontend typecheck/build/tests, gofmt, lint, and security checks available locally.**

**Step 2: Run the existing integration tests that cover gateway failover and admin settings.**

**Step 3: Review the complete diff for scope, billing safety, and no output-limit changes.**

**Step 4: Commit any verification fixes, push the branch, and wait for all required CI jobs.**

**Step 5: Build the image in the documented build environment, deploy to a new candidate pair, verify health and synthetic Flash/Pro/Thinking timeout behavior, then switch traffic only after checks pass. Preserve the prior pair for rollback.**
