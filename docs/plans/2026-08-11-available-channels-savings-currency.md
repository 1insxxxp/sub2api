# Available Channels Currency-Correct Savings Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Calculate Available Channels savings only after converting official USD prices to CNY with an independently configurable rate that defaults to 7.

**Architecture:** Add a persisted/public setting dedicated to the official USD-to-CNY benchmark. Thread it through the existing catalog builder, retain the existing site-price formula, and let the shared price comparison helper compare two CNY values. A zero or invalid benchmark rate suppresses savings without affecting prices or billing.

**Tech Stack:** Go settings service and Gin DTOs, Vue 3/TypeScript, Pinia public settings, Vitest, Go testing.

---

### Task 1: Add the backend setting contract

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/setting_parse.go`
- Modify: `backend/internal/service/settings_view.go`
- Modify: `backend/internal/service/setting_update.go`
- Modify: `backend/internal/service/setting_public.go`
- Modify: `backend/internal/handler/dto/settings.go`
- Modify: `backend/internal/handler/setting_handler.go`
- Modify: `backend/internal/handler/admin/setting_handler.go`
- Modify: `backend/internal/handler/admin/setting_handler_update.go`
- Test: `backend/internal/service/setting_service_public_test.go`

**Step 1:** Add failing tests for default `7` and public exposure of a configured rate.

**Step 2:** Run the focused Go tests and confirm they fail because the setting does not exist.

**Step 3:** Add the setting key, default, parsing, update, public/admin DTO and handler mappings using the existing CNY multiplier path as the template.

**Step 4:** Run the focused Go tests and confirm they pass.

### Task 2: Add the admin and public frontend contracts

**Files:**
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/stores/app.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/en/admin/settings.ts`
- Test: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

**Step 1:** Add a failing settings test that expects the independent rate control and default value.

**Step 2:** Run the focused test and confirm the missing field/control failure.

**Step 3:** Add the typed field, form state, payload mapping, input, preview and bilingual labels.

**Step 4:** Run the focused test and confirm it passes.

### Task 3: Correct the savings calculation

**Files:**
- Modify: `frontend/src/components/channels/availableChannelCatalog.ts`
- Modify: `frontend/src/components/channels/availableChannelPriceDisplay.ts`
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`
- Test: `frontend/src/components/channels/__tests__/availableChannelCatalogModel.spec.ts`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelModelListView.spec.ts`
- Test: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`

**Step 1:** Add failing tests for `$5`, `¥0.60`, rate `7` → `98%`, and rate `0` → no savings.

**Step 2:** Run the focused Vitest suites and verify the current mixed-currency formula fails.

**Step 3:** Pass the official USD/CNY rate into the catalog builder, store the converted official CNY benchmark, and compare the site price against that benchmark.

**Step 4:** Run the focused Vitest suites and confirm they pass.

### Task 4: Verify the complete change

**Files:**
- Verify all files above.

**Step 1:** Run focused Go tests for settings parsing/public exposure/update validation.

**Step 2:** Run all related Available Channels and settings Vitest suites.

**Step 3:** Run frontend ESLint and `vue-tsc --noEmit`.

**Step 4:** Run `git diff --check`, inspect the final diff, and verify the local frontend/backend health endpoints.
