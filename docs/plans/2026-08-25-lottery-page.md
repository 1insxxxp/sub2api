# 抽奖活动页面 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build one globally configured lottery activity for all authenticated users, with an admin switch/configuration page, atomic attempt and prize delivery, automatic balance credit, custom-product delivery, and user history.

**Architecture:** Add normalized lottery tables for activity, prizes, prize inventory, and draw snapshots. Keep only the exposure switch in the existing settings system, and execute each draw in one database transaction with an idempotency key so attempt consumption, inventory claiming, balance adjustment, and history creation succeed or fail together. Expose a user API and page at `/lottery`, plus admin APIs/page for configuring the single activity.

**Tech Stack:** Go, Gin, Ent, PostgreSQL SQL migrations, existing user/balance repositories and setting service, Vue 3, TypeScript, Pinia, Vue Router, Tailwind-style utility classes, Vitest.

---

### Task 1: Add lottery schema and migration

**Files:**
- Create: `backend/ent/schema/lottery_activity.go`
- Create: `backend/ent/schema/lottery_prize.go`
- Create: `backend/ent/schema/lottery_prize_item.go`
- Create: `backend/ent/schema/lottery_draw.go`
- Create: `backend/migrations/231_lottery.sql`
- Modify: `backend/internal/repository/migrations_schema_integration_test.go`
- Modify: generated files under `backend/ent/` via the repository Ent generator

**Step 1: Write the failing migration/schema assertions**

Add migration integration assertions for the four tables, key columns, the activity status/attempt indexes, the prize and inventory foreign keys, the user history index, and the unique idempotency key. Include the invariant that at most one activity can be active.

**Step 2: Run the focused migration test**

Run: `cd backend && go test ./internal/repository -run 'Test.*Migration|Migration' -count=1`

Expected: FAIL because the lottery tables and schema do not exist.

**Step 3: Implement the schema and SQL migration**

Create the four Ent schemas with `decimal(20,8)` for balance amounts and `timestamptz` for timestamps. Add enums/constants at the service/domain layer for activity status, attempt mode, prize type, and inventory status. In `231_lottery.sql`, create the tables, foreign keys, check constraints, indexes, and a partial unique index for one active activity. Make the migration idempotent and preserve draw snapshots after prizes are edited or deleted.

**Step 4: Regenerate Ent code and rerun the test**

Run: `cd backend && make generate && go test ./internal/repository -run 'Test.*Migration|Migration' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/ent/schema backend/ent backend/migrations/231_lottery.sql backend/internal/repository/migrations_schema_integration_test.go
git commit -m "feat: add lottery persistence schema"
```

### Task 2: Add the lottery feature switch to settings

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/settings_view.go`
- Modify: `backend/internal/service/setting_service.go`
- Modify: `backend/internal/service/setting_public.go`
- Modify: `backend/internal/handler/dto/settings.go`
- Modify: `backend/internal/handler/setting_handler.go`
- Modify: `backend/internal/handler/admin/setting_handler.go`
- Modify: `backend/internal/handler/dto/public_settings_injection_schema_test.go`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/utils/featureFlags.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/en/admin/settings.ts`
- Test: `backend/internal/service/setting_service_public_test.go`
- Test: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

**Step 1: Add failing settings contract tests**

Test that the new setting defaults to `false`, is included in `SystemSettings` and `PublicSettings`, survives admin update/read, and is present in the SSR injection payload. Test that the SettingsView renders a labeled toggle and sends the value in its save payload.

**Step 2: Run focused tests**

Run: `cd backend && go test ./internal/service ./internal/handler -run 'Setting|PublicSettings' -count=1` and `cd frontend && pnpm vitest run src/views/admin/__tests__/SettingsView.spec.ts`

Expected: FAIL because the setting key, DTO field, and UI control do not exist.

**Step 3: Implement the setting plumbing**

Add `lottery_enabled` as an opt-in setting with a safe default of false. Thread it through setting load, default initialization, admin update validation/audit diff, public DTO serialization, and SSR injection. Register it as `FeatureFlags.lottery` with opt-in semantics. Add the SettingsView toggle and a configure link that points to the admin lottery page.

**Step 4: Rerun focused tests**

Run the commands from Step 2.

Expected: PASS, including the public-settings injection drift test.

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/handler frontend/src/types/index.ts frontend/src/api/admin/settings.ts frontend/src/utils/featureFlags.ts frontend/src/views/admin/SettingsView.vue frontend/src/i18n/locales
git commit -m "feat: add lottery feature setting"
```

### Task 3: Implement lottery repository and service rules

**Files:**
- Create: `backend/internal/repository/lottery_repo.go`
- Create: `backend/internal/service/lottery_service.go`
- Create: `backend/internal/service/lottery_service_test.go`
- Modify: `backend/internal/service/user_service.go` or the existing balance repository interface only if a transaction-capable balance method is required

**Step 1: Write failing service tests**

Cover these cases with repository/service fakes or an Ent test database: feature disabled; no active activity; activity time boundaries; daily and total attempt limits; zero/negative configuration rejection; weighted candidate selection; product inventory single-claim behavior; balance credit; history snapshots; and repeated idempotency key returning the original result without a second reward.

**Step 2: Run the focused service tests**

Run: `cd backend && go test ./internal/service -run Lottery -count=1`

Expected: FAIL because the service and repository do not exist.

**Step 3: Implement repository operations**

Provide methods for loading the active activity, loading/saving admin configuration, counting user attempts for the current daily window or activity lifetime, creating a draw, finding an idempotent draw, and claiming one available product item with a row lock. Keep all user-visible history fields as immutable snapshots.

**Step 4: Implement transactional draw orchestration**

In `LotteryService.Draw`, validate the setting and activity, lock the relevant attempt scope, enforce the configured user limit, select a prize server-side from enabled prizes, retry selection against available product inventory, claim the product item if needed, adjust the user balance for balance prizes, create the draw snapshot, and commit. Generate or accept a request idempotency key and return the existing draw for a duplicate key. Do not consume an attempt when validation, inventory selection, balance adjustment, or persistence fails.

**Step 5: Rerun the focused service tests**

Run: `cd backend && go test ./internal/service -run Lottery -count=1`

Expected: PASS, including concurrent product claims and duplicate requests.

**Step 6: Commit**

```bash
git add backend/internal/repository/lottery_repo.go backend/internal/service/lottery_service.go backend/internal/service/lottery_service_test.go backend/internal/service/user_service.go
git commit -m "feat: implement lottery draw service"
```

### Task 4: Add user and admin lottery APIs and wire dependencies

**Files:**
- Create: `backend/internal/handler/lottery_handler.go`
- Create: `backend/internal/handler/admin/lottery_handler.go`
- Create: `backend/internal/handler/lottery_handler_test.go`
- Create: `backend/internal/handler/admin/lottery_handler_test.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/server/routes/admin.go`

**Step 1: Write failing HTTP contract tests**

Test authenticated user activity, draw, and history endpoints; admin activity read/update/enable/disable and product inventory endpoints; request validation; feature-off behavior; auth/role checks; stable business error codes; and idempotency response shape.

**Step 2: Run the focused handler tests**

Run: `cd backend && go test ./internal/handler ./internal/server/routes -run Lottery -count=1`

Expected: FAIL because routes and handlers are not wired.

**Step 3: Implement DTOs, handlers, and routes**

Register authenticated user routes under `/api/v1/lottery` and admin routes under `/api/v1/admin/lottery`. User activity responses must expose only public prize metadata and the current user's remaining attempts; never expose unclaimed product inventory. Admin responses may include inventory counts and item status. Add the handler constructors to `handler.go` and the Wire provider set in `wire.go`.

**Step 4: Rerun the focused handler tests**

Run: `cd backend && go test ./internal/handler ./internal/server/routes -run Lottery -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler backend/internal/server/routes
git commit -m "feat: expose lottery APIs"
```

### Task 5: Build the admin lottery configuration page

**Files:**
- Create: `frontend/src/api/admin/lottery.ts`
- Create: `frontend/src/views/admin/LotteryView.vue`
- Create: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/api/index.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue` if an admin navigation entry is needed by the existing settings pattern
- Modify: `frontend/src/i18n/locales/zh/common.ts`
- Modify: `frontend/src/i18n/locales/en/common.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/en/admin/settings.ts`

**Step 1: Write failing admin page tests**

Test loading the single activity, switching daily/total mode, validating attempt limits and prize weights, editing balance amounts, parsing product inventory one item per line, showing inventory counts, and saving/enabling/disabling the activity.

**Step 2: Run the focused frontend test**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/LotteryView.spec.ts`

Expected: FAIL because the API module and page do not exist.

**Step 3: Implement the admin API and page**

Use the existing admin page shell and controls. Keep one activity visible at a time, make status and active-window state obvious, and provide an explicit enable action that validates the activity before publishing. Product inventory should use a textarea with one delivery item per line and show available/claimed counts. Keep destructive inventory removal behind confirmation and never remove claimed history data.

**Step 4: Add the route and settings configure link**

Add an authenticated admin route such as `/admin/lottery`, a page title, and a link from the lottery feature toggle. Preserve direct-route behavior when the feature switch is off so administrators can still configure it.

**Step 5: Rerun focused tests**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/LotteryView.spec.ts src/views/admin/__tests__/SettingsView.spec.ts`

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/api frontend/src/views/admin frontend/src/router/index.ts frontend/src/components/layout/AppSidebar.vue frontend/src/i18n/locales
git commit -m "feat: add lottery admin configuration"
```

### Task 6: Build the authenticated user lottery page

**Files:**
- Create: `frontend/src/api/lottery.ts`
- Create: `frontend/src/views/user/LotteryView.vue`
- Create: `frontend/src/views/user/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/api/index.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/i18n/locales/zh/common.ts`
- Modify: `frontend/src/i18n/locales/en/common.ts`
- Modify: `frontend/src/i18n/locales/zh/user.ts` or the existing user locale module
- Modify: `frontend/src/i18n/locales/en/user.ts` or the existing user locale module

**Step 1: Write failing user page tests**

Test activity loading, remaining-attempt display, disabled/loading button state, balance result, product result with copy action, history pagination, feature-off route behavior, and mobile-friendly state rendering.

**Step 2: Run the focused frontend test**

Run: `cd frontend && pnpm vitest run src/views/user/__tests__/LotteryView.spec.ts`

Expected: FAIL because the page and API client do not exist.

**Step 3: Implement the API client and page**

Create a compact activity-first layout consistent with existing user pages. Generate an idempotency key per draw click, disable the button until the request settles, update the balance in the auth store after a balance win, display product content only in the winner result/history, and refresh activity/history after a successful draw. Add a clipboard action with a visible success/error state.

**Step 4: Add feature-gated route and navigation**

Add `/lottery` as a login-required route. Register `FeatureFlags.lottery` as opt-in and attach it to the user navigation item. Direct access should show the existing feature-disabled/not-found behavior when the setting is false; no prize data should be fetched before authentication and feature checks pass.

**Step 5: Rerun focused tests**

Run: `cd frontend && pnpm vitest run src/views/user/__tests__/LotteryView.spec.ts src/router/__tests__/feature-access.spec.ts src/components/layout/__tests__/AppHeader.spec.ts`

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/api frontend/src/views/user frontend/src/router/index.ts frontend/src/components/layout/AppSidebar.vue frontend/src/i18n/locales frontend/src/types/index.ts
git commit -m "feat: add user lottery page"
```

### Task 7: Add end-to-end regression coverage and local verification

**Files:**
- Modify: `backend/internal/repository/migrations_schema_integration_test.go` if any migration assertions need final names
- Create or modify: focused backend integration tests under `backend/internal/service` and `backend/internal/handler`
- Modify: `frontend/src/views/user/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`

**Step 1: Run backend package tests**

Run: `cd backend && go test ./internal/service ./internal/handler ./internal/repository ./internal/server/routes -count=1`

Expected: PASS.

**Step 2: Run frontend focused and type checks**

Run: `cd frontend && pnpm vitest run src/views/user/__tests__/LotteryView.spec.ts src/views/admin/__tests__/LotteryView.spec.ts src/views/admin/__tests__/SettingsView.spec.ts && pnpm type-check`

Expected: PASS.

**Step 3: Run the complete frontend test suite**

Run: `cd frontend && pnpm test:run`

Expected: PASS with no feature-flag or route regressions.

**Step 4: Run local smoke verification**

Start the existing local dev services, open `/admin/settings` to enable the lottery switch, configure one daily and one product/balance prize scenario, then verify `/lottery` on desktop and a narrow mobile viewport. Confirm: off means hidden/blocked, on means available, one draw decrements the correct counter, balance changes immediately, product content is visible only to the winner, refresh preserves history, duplicate clicks do not duplicate rewards, and changing the activity does not delete historical draws.

**Step 5: Review the diff and commit verification fixes**

Run: `git diff --check`, `git status --short`, and inspect the final diff for accidental changes to existing user work. Commit only lottery-related changes.

## Handoff

Plan complete and saved to `docs/plans/2026-08-25-lottery-page.md`. Implementation should use `superpowers:executing-plans` task-by-task, with a review checkpoint after the schema/service/API foundation and another after both frontend pages are complete.
