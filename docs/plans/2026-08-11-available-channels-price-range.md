# Available Channels Price Range Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Display site prices, savings, and effective multipliers as ranges derived from the configured `0.16–0.20` recharge-ratio range.

**Architecture:** Keep the existing minimum multiplier key for backward compatibility and add a maximum multiplier key to the settings KV pipeline. Normalize both endpoints into the catalog once, then let shared display helpers and cards render either a single value or a collapsed range without recalculating billing data in components.

**Tech Stack:** Go settings service and Gin DTOs, Vue 3/TypeScript, Pinia, Vue I18n, Vitest, Testify.

---

### Task 1: Add the maximum recharge-ratio setting to the backend

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/setting_parse.go`
- Modify: `backend/internal/service/settings_view.go`
- Modify: `backend/internal/service/setting_public.go`
- Modify: `backend/internal/service/setting_update.go`
- Modify: `backend/internal/handler/dto/settings.go`
- Modify: `backend/internal/handler/setting_handler.go`
- Modify: `backend/internal/handler/admin/setting_handler.go`
- Modify: `backend/internal/handler/admin/setting_handler_update.go`
- Test: `backend/internal/service/setting_service_public_test.go`
- Test: `backend/internal/server/api_contract_test.go`

**Step 1: Write failing backend tests**

Add tests proving that configured minimum `0.16` and maximum `0.20` are exposed by public settings and injection payloads, while a missing maximum defaults to `0.20`. Cover explicit zero and a maximum below the minimum.

**Step 2: Run the focused tests and confirm RED**

```bash
cd backend
go test -tags=unit ./internal/service ./internal/server -run 'AvailableChannelsPriceCNYMultiplier|OfficialUSDToCNYRate|APIContract' -count=1
```

Expected: FAIL because the maximum field and key do not exist.

**Step 3: Implement the backend setting pipeline**

Add:

```go
SettingKeyAvailableChannelsPriceCNYMultiplierMax = "available_channels_price_cny_multiplier_max"
```

Parse the pair with these invariants:

```go
min := parseNonNegativeFloatSetting(rawMin)
max := parseOptionalNonNegativeFloat(rawMax, 0.20)
if max < min {
    max = min
}
```

Thread `AvailableChannelsPriceCNYMultiplierMax` through service views, admin/public DTOs, handlers, update persistence, public-key loading, and injection payloads. Missing max resolves to `0.20`; the frontend separately falls back to min only when connected to an old backend that omits the field.

**Step 4: Run focused and package tests**

```bash
cd backend
go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin ./internal/server -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/handler backend/internal/server/api_contract_test.go
git commit -m "feat: configure available channel price ranges"
```

### Task 2: Add the maximum ratio to admin and public frontend settings

**Files:**
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/stores/app.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/en/admin/settings.ts`
- Test: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

**Step 1: Write a failing settings-view test**

Load `0.16` and `0.20`, assert both inputs render, change them, submit, and assert both fields are included in the update payload.

**Step 2: Run the test and confirm RED**

```bash
cd frontend
npx vitest run src/views/admin/__tests__/SettingsView.spec.ts --poolOptions.threads.singleThread
```

Expected: FAIL because the maximum input and payload field do not exist.

**Step 3: Implement the settings UI**

Expose adjacent numeric inputs labelled “最低充值比例” and “最高充值比例”. Clamp negatives to zero and normalize max to at least min before submission. Add the maximum field to admin/public types and use the minimum as runtime fallback when old public settings omit it.

**Step 4: Run the settings test, ESLint, and typecheck**

```bash
cd frontend
npx vitest run src/views/admin/__tests__/SettingsView.spec.ts --poolOptions.threads.singleThread
npx eslint src/api/admin/settings.ts src/types/index.ts src/stores/app.ts src/views/admin/SettingsView.vue src/views/admin/__tests__/SettingsView.spec.ts
npx vue-tsc --noEmit
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/settings.ts frontend/src/types/index.ts frontend/src/stores/app.ts frontend/src/views/admin frontend/src/i18n/locales
git commit -m "feat: manage available channel price range"
```

### Task 3: Normalize minimum and maximum site prices in the catalog

**Files:**
- Modify: `frontend/src/components/channels/availableChannelCatalog.ts`
- Test: `frontend/src/components/channels/__tests__/availableChannelCatalogModel.spec.ts`
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`
- Test: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`

**Step 1: Write failing catalog tests**

For official price `5`, group rate `0.75`, minimum `0.16`, and maximum `0.20`, assert:

```ts
expect(price.site).toBeCloseTo(0.6)
expect(price.siteMax).toBeCloseTo(0.75)
expect(model.effectiveRate).toBeCloseTo(0.12)
expect(model.effectiveRateMax).toBeCloseTo(0.15)
```

Cover token, request, image, cache, interval, peak, equal endpoints, zero endpoints, and old-backend missing-max fallback.

**Step 2: Run the tests and confirm RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/availableChannelCatalogModel.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts --poolOptions.threads.singleThread
```

Expected: FAIL because maximum catalog fields and the new builder argument do not exist.

**Step 3: Implement range normalization**

Extend normalized values while keeping the existing minimum fields:

```ts
interface CatalogPriceValue {
  official: number | null
  officialCny: number | null
  site: number | null
  siteMax: number | null
  peakSite: number | null
  peakSiteMax: number | null
}
```

Add `effectiveRateMax` to group/model entries. Update `buildAvailableChannelCatalog` to accept the maximum multiplier, normalize it to at least the minimum, and compute both endpoints once. Pass the public setting from `AvailableChannelsView`, falling back to the minimum when absent.

**Step 4: Run focused tests and typecheck**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/availableChannelCatalogModel.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts --poolOptions.threads.singleThread
npx vue-tsc --noEmit
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/availableChannelCatalog.ts frontend/src/components/channels/__tests__/availableChannelCatalogModel.spec.ts frontend/src/views/user/AvailableChannelsView.vue frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts
git commit -m "feat: calculate available channel price ranges"
```

### Task 4: Render price, savings, and effective-rate ranges

**Files:**
- Modify: `frontend/src/components/channels/availableChannelPriceDisplay.ts`
- Modify: `frontend/src/components/channels/AvailableChannelModelList.vue`
- Modify: `frontend/src/components/channels/AvailableChannelModelPrice.vue`
- Modify: `frontend/src/components/channels/AvailableChannelOfferingCard.vue`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelModelListView.spec.ts`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts`

**Step 1: Write failing component tests**

Assert distinct endpoints render as `¥0.60–¥0.75`, equal endpoints collapse to one price, savings render in ascending order as `省 97%–98%`, and actual rate renders `0.12×–0.15×`. Cover zero and unpriced states.

**Step 2: Run the component tests and confirm RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelModelListView.spec.ts src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts --poolOptions.threads.singleThread
```

Expected: FAIL because current display helpers only accept one site value.

**Step 3: Implement shared range formatting**

Add helpers that format two finite values, deduplicate equal formatted endpoints, and never recalculate prices in Vue components. Savings calculates both endpoints against `officialCny`, discards non-positive savings, sorts integer percentages, and returns one number or a range.

**Step 4: Run component tests and accessibility regressions**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelModelListView.spec.ts src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts src/components/channels/__tests__/AvailableChannelCatalog.spec.ts --poolOptions.threads.singleThread
npx eslint src/components/channels
npx vue-tsc --noEmit
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/channels
git commit -m "feat: display available channel price ranges"
```

### Task 5: Verify the complete local feature

**Files:**
- No production files expected.

**Step 1: Run backend verification**

```bash
cd backend
go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin ./internal/server -count=1
```

Expected: PASS.

**Step 2: Run frontend verification**

```bash
cd frontend
npx vitest run src/views/admin/__tests__/SettingsView.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts src/components/channels/__tests__/availableChannelCatalogModel.spec.ts src/components/channels/__tests__/AvailableChannelModelListView.spec.ts src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts src/components/channels/__tests__/AvailableChannelCatalog.spec.ts --poolOptions.threads.singleThread
npx eslint src/api/admin/settings.ts src/types/index.ts src/stores/app.ts src/views/admin/SettingsView.vue src/views/user/AvailableChannelsView.vue src/components/channels
npx vue-tsc --noEmit
npm run build
```

Expected: PASS.

**Step 3: Check compatibility and worktree integrity**

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only intended feature files plus pre-existing user-owned files are present.

**Step 4: Restart local backend and verify settings**

Restart `go run ./cmd/server`, confirm `/health` is OK and `/api/v1/settings/public` exposes min/max/rate fields. Keep the Vite dev server running for user testing.
