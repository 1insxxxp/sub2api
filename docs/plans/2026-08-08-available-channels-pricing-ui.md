# Available Channels Pricing UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebuild the user-facing Available Channels page as a responsive channel navigator and price catalog that permanently compares official and site prices for every model and source group.

**Architecture:** Keep the existing `/channels/available`, `/groups/rates`, and public-settings contracts. Normalize them once in a pure TypeScript catalog module, then render the same normalized channel/group/model entries through a desktop two-pane catalog and mobile channel picker plus price cards. Preserve channel selection during background refreshes, surface partial user-rate failures, and keep all pricing calculations out of Vue templates.

**Tech Stack:** Vue 3 Composition API, TypeScript 5.6, Tailwind CSS, Pinia, Vue I18n, Vitest, Vue Test Utils, Vite.

---

## Pre-implementation constraints

- Work on `dev`; do not modify the user-owned untracked root `package.json` or `package-lock.json`.
- Do not modify gateway billing or backend pricing behavior.
- Treat `available_channels_price_cny_multiplier` as the official-price-to-site-CNY conversion factor, then multiply it by the resolved group rate.
- Resolve the normal rate as `userGroupRates[group.id] ?? group.rate_multiplier ?? 1`.
- Peak token price uses `normalRate * group.peak_rate_multiplier`; image/per-request pricing does not receive the peak factor, matching `backend/internal/service/group.go`.
- Do not merge same-name models across groups.
- Do not add a “saving percentage” because official and site prices use different currencies.
- Keep the old `AvailableChannelsTable.vue` until the new catalog passes component, type, build, and visual checks; remove it only in the final cleanup task.

### Task 1: Create the normalized catalog and pricing view model

**Files:**
- Create: `frontend/src/components/channels/availableChannelCatalog.ts`
- Create: `frontend/src/components/channels/__tests__/availableChannelCatalog.spec.ts`
- Read: `frontend/src/api/channels.ts`
- Read: `frontend/src/constants/channel.ts`
- Read: `frontend/src/utils/pricing.ts`

**Step 1: Write failing tests for price normalization**

Create fixtures covering token, per-request, image, tiered, and unpriced models. Assert at least:

```ts
expect(entry.normalRate).toBe(0.5)
expect(entry.prices.input?.official).toBe(5)
expect(entry.prices.input?.site).toBe(18) // 5 * 7.2 * 0.5
expect(entry.prices.input?.peakSite).toBe(36) // normal site * peak factor 2
expect(imageEntry.prices.request?.peakSite).toBeNull()
expect(unpricedEntry.hasPricing).toBe(false)
```

Add a duplicate-model fixture in two groups and assert that both entries remain present with distinct `groupKey` and site prices.

**Step 2: Run the focused test and verify it fails**

Run:

```bash
pnpm --dir frontend exec vitest run src/components/channels/__tests__/availableChannelCatalog.spec.ts
```

Expected: FAIL because `availableChannelCatalog.ts` does not exist.

**Step 3: Define catalog types and pure helpers**

Implement explicit view types similar to:

```ts
export interface CatalogPriceValue {
  official: number | null
  site: number | null
  peakSite: number | null
}

export interface CatalogModelEntry {
  key: string
  groupKey: string
  name: string
  platform: string
  billingMode: BillingMode
  hasPricing: boolean
  normalRate: number
  peakFactor: number | null
  prices: {
    input: CatalogPriceValue | null
    output: CatalogPriceValue | null
    cacheWrite: CatalogPriceValue | null
    cacheRead: CatalogPriceValue | null
    imageInput: CatalogPriceValue | null
    imageOutput: CatalogPriceValue | null
    request: CatalogPriceValue | null
  }
  intervals: CatalogPricingInterval[]
}

export interface CatalogGroupEntry {
  key: string
  id: number
  name: string
  platform: string
  subscriptionType: string
  isExclusive: boolean
  normalRate: number
  defaultRate: number
  userRate: number | null
  peak: { enabled: boolean; start: string; end: string; factor: number } | null
  models: CatalogModelEntry[]
}

export interface CatalogChannelEntry {
  key: string
  name: string
  description: string
  platforms: string[]
  groups: CatalogGroupEntry[]
  groupCount: number
  modelCount: number
}
```

Export pure functions:

```ts
export function buildAvailableChannelCatalog(
  rows: UserAvailableChannel[],
  userGroupRates: Record<number, number>,
  cnyMultiplier: number,
): CatalogChannelEntry[]

export function filterAvailableChannelCatalog(
  catalog: CatalogChannelEntry[],
  filters: { search: string; platform: string; pricedOnly: boolean },
): CatalogChannelEntry[]
```

Rules:

- Iterate channel → platform section → group → `group.supported_models`.
- If group-scoped model arrays are unavailable, use the section fallback models without duplicating the section across groups.
- Use stable keys composed from channel index/name, platform, group ID, model name, and model occurrence.
- Preserve model entries per group; only de-duplicate exact duplicates within the same group.
- Site value is `officialValue * cnyMultiplier * normalRate`.
- Token-only peak value is `siteValue * peakFactor`; request/image peak value is `null`.
- Convert interval prices using the same rate rules.
- Never coerce missing prices to zero.

**Step 4: Add failing tests for filtering and counts**

Assert:

```ts
expect(filterAvailableChannelCatalog(catalog, {
  search: 'opus', platform: 'all', pricedOnly: false,
})[0].groups[0].models).toHaveLength(1)

expect(filterAvailableChannelCatalog(catalog, {
  search: '', platform: 'anthropic', pricedOnly: true,
})[0].modelCount).toBe(2)
```

Also cover searches by channel name, description, group name, and model name. Channel or group metadata matches must preserve all descendant models; model-only matches must narrow descendants.

**Step 5: Run tests and verify they pass**

Run:

```bash
pnpm --dir frontend exec vitest run src/components/channels/__tests__/availableChannelCatalog.spec.ts
```

Expected: PASS.

**Step 6: Commit the catalog layer**

```bash
git add frontend/src/components/channels/availableChannelCatalog.ts \
  frontend/src/components/channels/__tests__/availableChannelCatalog.spec.ts
git commit -m "feat: add available channel pricing catalog model"
```

### Task 2: Build the shared official-versus-site price renderer

**Files:**
- Create: `frontend/src/components/channels/AvailableChannelModelPrice.vue`
- Create: `frontend/src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`
- Reuse: `frontend/src/utils/pricing.ts`
- Reuse: `frontend/src/utils/platformColors.ts`

**Step 1: Write failing component tests**

Mount one entry for each billing mode and assert:

```ts
expect(wrapper.get('[data-testid="official-price"]').text()).toContain('$')
expect(wrapper.get('[data-testid="site-price"]').text()).toContain('¥')
expect(wrapper.text()).toContain('输入')
expect(wrapper.text()).toContain('输出')
```

Also assert:

- Official and site prices render before any click.
- Token entry expands cache and peak price rows.
- Tiered entry displays the first tier with the “starting from” label and expands every interval.
- Image/per-request entry uses `/ 张` or `/ 次` rather than `/ 1M Token`.
- Unpriced entry renders “暂未定价” without tooltip-only content.
- Toggle button has `aria-expanded` and references the detail region.

**Step 2: Run the focused test and verify it fails**

```bash
pnpm --dir frontend exec vitest run src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts
```

Expected: FAIL because the component does not exist.

**Step 3: Implement the responsive price row/card**

The component accepts one `CatalogModelEntry` and renders:

- Model name, billing-mode badge, and effective rate.
- A muted official-price column and emphasized site-price column.
- Input/output pairs for token billing.
- One price and its unit for per-request/image billing.
- A real button for details with `aria-expanded`.
- An inline detail region containing cache, image token, intervals, normal site price, and peak site price as applicable.

Use desktop grid classes at `lg` and a two-column price comparison card below `lg`. Use `font-mono tabular-nums` only for numeric values, not all body copy.

Do not import `SupportedModelChip.vue`; the new component must make the main price visible without popovers.

**Step 4: Add translations**

Add matching Chinese and English keys under `availableChannels`, including:

```ts
catalog: {
  officialPrice: '官方价',
  sitePrice: '本站价',
  effectiveRate: '实际倍率',
  startingFrom: '起',
  peakPrice: '高峰价',
  regularPrice: '常规价',
  showDetails: '展开价格详情',
  hideDetails: '收起价格详情',
  unpriced: '暂未定价'
}
```

**Step 5: Run component tests**

```bash
pnpm --dir frontend exec vitest run src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts
```

Expected: PASS.

**Step 6: Commit the price renderer**

```bash
git add frontend/src/components/channels/AvailableChannelModelPrice.vue \
  frontend/src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts \
  frontend/src/i18n/locales/zh/dashboard.ts \
  frontend/src/i18n/locales/en/dashboard.ts
git commit -m "feat: compare official and site model prices"
```

### Task 3: Build group sections and the desktop channel navigator

**Files:**
- Create: `frontend/src/components/channels/AvailableChannelGroupSection.vue`
- Create: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Create: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`
- Reuse: `frontend/src/components/common/GroupBadge.vue`
- Reuse: `frontend/src/components/common/PlatformIcon.vue`
- Reuse: `frontend/src/utils/peak-rate.ts`

**Step 1: Write failing tests for catalog selection**

Mount a two-channel fixture and assert:

- The first channel is selected by default.
- Clicking the second channel updates `aria-selected` and the right detail panel.
- Each navigation item exposes channel name, platform, group count, and model count.
- A selected channel remains selected after rows refresh if its stable key still exists.
- If filtering removes the current selection, the first filtered channel becomes active.
- Duplicate model names in separate groups produce two price entries.

Example assertion:

```ts
expect(wrapper.get('[data-testid="channel-nav-item-0"]').attributes('aria-selected')).toBe('true')
await wrapper.get('[data-testid="channel-nav-item-1"]').trigger('click')
expect(wrapper.get('[data-testid="channel-detail-title"]').text()).toBe('Backup channel')
```

**Step 2: Run the focused test and verify it fails**

```bash
pnpm --dir frontend exec vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: FAIL because the catalog components do not exist.

**Step 3: Implement the group section**

`AvailableChannelGroupSection.vue` receives one `CatalogGroupEntry` and renders:

- `GroupBadge` with default and user rate.
- Exclusive/public and subscription badges.
- Peak window and factor using existing server-timezone formatting.
- Desktop price-column headings once per group.
- One `AvailableChannelModelPrice` per model.
- A collapsible group body below `lg`; the desktop body remains expanded.

Use real buttons and set `aria-expanded`/`aria-controls` on mobile group toggles.

**Step 4: Implement the catalog shell**

`AvailableChannelCatalog.vue` accepts:

```ts
defineProps<{
  channels: CatalogChannelEntry[]
  loading: boolean
  refreshing: boolean
  rateFallback: boolean
}>()
```

It owns the selected channel key and emits no pricing calculations. Desktop layout:

- `lg:grid lg:grid-cols-[280px_minmax(0,1fr)]`.
- Sticky, independently scrollable channel navigation.
- Listbox/tab semantics with visible focus rings.
- Right channel header and group sections.
- Skeleton, no-channel, filter-empty, and rate-fallback message slots or props.

Do not use a five-column table or render model chips.

**Step 5: Run the catalog tests**

```bash
pnpm --dir frontend exec vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: PASS.

**Step 6: Commit desktop catalog components**

```bash
git add frontend/src/components/channels/AvailableChannelGroupSection.vue \
  frontend/src/components/channels/AvailableChannelCatalog.vue \
  frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
git commit -m "feat: add available channel catalog layout"
```

### Task 4: Add the mobile channel picker bottom sheet

**Files:**
- Create: `frontend/src/components/channels/AvailableChannelPicker.vue`
- Create: `frontend/src/components/channels/__tests__/AvailableChannelPicker.spec.ts`
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`

**Step 1: Write failing mobile picker tests**

Assert:

- The picker is hidden at desktop surface and available at mobile surface through responsive classes.
- Opening renders a teleported dialog with `role="dialog"` and `aria-modal="true"`.
- Channel search narrows options.
- Selecting an option emits its key and closes the sheet.
- Escape, backdrop, and close button close it.
- Opening saves focus and closing restores focus to the trigger.
- The sheet body is scrollable and the document body is locked while open.

**Step 2: Run the focused test and verify it fails**

```bash
pnpm --dir frontend exec vitest run src/components/channels/__tests__/AvailableChannelPicker.spec.ts
```

Expected: FAIL because the picker does not exist.

**Step 3: Implement the bottom sheet**

Use the established mobile-dialog pattern:

```html
<div class="fixed inset-0 z-[70] flex items-end bg-slate-950/55" role="dialog" aria-modal="true">
  <section class="max-h-[calc(100dvh-3rem)] w-full overflow-hidden rounded-t-3xl ...">
    <!-- sticky header + search -->
    <!-- scrollable channel options -->
  </section>
</div>
```

Each channel option shows name, platform badges, group count, and model count. Keep the picker search local so it does not mutate the page model search.

**Step 4: Wire the picker into the catalog**

Below `lg`, replace the desktop left rail with one 44px minimum-height trigger showing the selected channel. Selecting a channel updates the same selected key used by desktop navigation.

When model search filters the active channel out, close the picker and select the first remaining channel.

**Step 5: Run picker and catalog tests**

```bash
pnpm --dir frontend exec vitest run \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: PASS.

**Step 6: Commit mobile navigation**

```bash
git add frontend/src/components/channels/AvailableChannelPicker.vue \
  frontend/src/components/channels/AvailableChannelCatalog.vue \
  frontend/src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  frontend/src/i18n/locales/zh/dashboard.ts \
  frontend/src/i18n/locales/en/dashboard.ts
git commit -m "feat: add mobile available channel picker"
```

### Task 5: Integrate filters, resilient refresh, and partial-rate warnings

**Files:**
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`
- Create: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`
- Use: `frontend/src/components/channels/availableChannelCatalog.ts`
- Use: `frontend/src/components/channels/AvailableChannelCatalog.vue`

**Step 1: Write failing view tests**

Mock `getAvailable()` and `getUserGroupRates()` and assert:

- Both load concurrently.
- First load shows catalog skeleton.
- Search filters channel/group/model data through the pure catalog helper.
- Platform filter and “priced only” filter update the catalog.
- A subsequent refresh leaves the existing catalog mounted while the refresh icon spins.
- Channel API refresh failure retains prior data and shows an error.
- Group-rate failure renders default-rate prices and a visible partial-data warning.
- Successful later refresh clears the warning.

Example:

```ts
expect(wrapper.get('[data-testid="available-channel-catalog"]').exists()).toBe(true)
await wrapper.get('[data-testid="refresh-channels"]').trigger('click')
expect(wrapper.get('[data-testid="available-channel-catalog"]').exists()).toBe(true)
expect(wrapper.get('[data-testid="refresh-channels"] svg').classes()).toContain('animate-spin')
```

**Step 2: Run the focused test and verify it fails**

```bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/AvailableChannelsView.spec.ts
```

Expected: FAIL because the view still uses `AvailableChannelsTable` and clears loading state globally.

**Step 3: Replace the old table integration**

In `AvailableChannelsView.vue`:

- Replace `AvailableChannelsTable` with `AvailableChannelCatalog`.
- Add `selectedPlatform` and `pricedOnly` refs.
- Derive platforms from normalized catalog.
- Compute `normalizedCatalog` and `filteredCatalog` using the pure helpers.
- Keep `loading` for first load and add `refreshing` for later refreshes.
- Keep last successful channel data on refresh failure.
- Track `rateFallback` separately from fatal channel errors.
- Keep refresh available in fatal and nonfatal states.

Use a compact responsive toolbar. The platform control may be a native/select-style project control, but must fit at 390px without forcing horizontal scroll.

**Step 4: Add view translations**

Add matching keys for:

- all platforms
- priced only
- select channel
- search channels in picker
- groups count
- models count
- failed channel load and retry
- failed user-rate load warning
- no filter results

**Step 5: Run the view and all new channel tests**

```bash
pnpm --dir frontend exec vitest run \
  src/views/user/__tests__/AvailableChannelsView.spec.ts \
  src/components/channels/__tests__/availableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts
```

Expected: PASS.

**Step 6: Commit page integration**

```bash
git add frontend/src/views/user/AvailableChannelsView.vue \
  frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts \
  frontend/src/i18n/locales/zh/dashboard.ts \
  frontend/src/i18n/locales/en/dashboard.ts
git commit -m "feat: rebuild available channels price catalog"
```

### Task 6: Remove obsolete presentation code and close regressions

**Files:**
- Delete: `frontend/src/components/channels/AvailableChannelsTable.vue`
- Delete or rewrite: `frontend/src/components/channels/__tests__/AvailableChannelsTable.spec.ts`
- Keep: `frontend/src/components/channels/SupportedModelChip.vue` if admin or another page still imports it
- Keep: `frontend/src/components/channels/PricingRow.vue` if `SupportedModelChip.vue` still imports it
- Inspect: all imports returned by `rg "AvailableChannelsTable|SupportedModelChip|PricingRow" frontend/src`

**Step 1: Prove the old table is no longer referenced**

Run:

```bash
rg -n "AvailableChannelsTable" frontend/src
```

Expected before deletion: only the component and its old test remain.

If any live view still imports it, stop and migrate that caller before deleting.

**Step 2: Remove the obsolete table and old table-specific test**

Delete only the table and tests that assert the five-column/mobile duplicate surfaces. Do not delete `SupportedModelChip` or `PricingRow` while another caller uses them.

**Step 3: Run lint and type checking**

```bash
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
```

Expected: both exit 0 without modifying files.

If lint reports formatting-only errors, fix them explicitly; do not run the repository-wide `lint --fix` because it may rewrite unrelated files.

**Step 4: Run the focused regression suite**

```bash
pnpm --dir frontend exec vitest run \
  src/views/user/__tests__/AvailableChannelsView.spec.ts \
  src/components/channels/__tests__/availableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  src/components/channels/__tests__/PricingRow.spec.ts
```

Expected: PASS.

**Step 5: Build production frontend**

```bash
pnpm --dir frontend run build
```

Expected: `vue-tsc -b` and `vite build` both exit 0.

**Step 6: Commit cleanup**

```bash
git add -u frontend/src/components/channels/AvailableChannelsTable.vue \
  frontend/src/components/channels/__tests__/AvailableChannelsTable.spec.ts
git commit -m "refactor: retire available channels table UI"
```

Before committing, run `git status --short` and ensure the root untracked `package.json` and `package-lock.json` remain untracked and unstaged.

### Task 7: Perform browser-level responsive and accessibility verification

**Files:**
- Modify only files implicated by an observed issue.
- Do not create screenshot artifacts inside the repository.

**Step 1: Start or reuse the local frontend**

```bash
pnpm --dir frontend run dev --host 127.0.0.1
```

Expected: Vite serves the local application, normally on `http://127.0.0.1:3000` or the next available port.

**Step 2: Verify the authenticated page at 1440px**

Check:

- Desktop left navigation remains sticky and independently scrollable.
- Selected channel and keyboard focus are visually distinct.
- Official and site prices are visible without hover.
- Long names truncate or wrap without covering prices.
- Group and model details expand with mouse and keyboard.
- Background refresh does not blank the page.

**Step 3: Verify 768px and 390px**

Check:

- Desktop channel rail is absent.
- Mobile channel picker opens from the bottom and fits within `100dvh`.
- Picker search, selection, Escape/backdrop close, and focus restoration work.
- First group is expanded and search matches auto-expand.
- Price cards show official and site columns without horizontal scrolling.
- Every interactive control has at least a 44px touch target.

**Step 4: Verify dark theme and long-data stress cases**

Check contrast, borders, focus rings, unpriced state, tiered details, peak pricing, and a channel with many groups/models.

**Step 5: Run the final verification set**

```bash
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend exec vitest run \
  src/views/user/__tests__/AvailableChannelsView.spec.ts \
  src/components/channels/__tests__/availableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  src/components/channels/__tests__/PricingRow.spec.ts
pnpm --dir frontend run build
```

Expected: all commands exit 0.

**Step 6: Record the final verification commit**

If browser verification required fixes:

```bash
# Inspect first, then stage only the exact files changed for this feature.
git diff --name-only
git add <exact-feature-files>
git commit -m "fix: polish available channel catalog responsiveness"
```

If no files changed, do not create an empty commit.

## Final acceptance checklist

- Official and site prices are visible before interaction.
- Same-name models stay separate across source groups.
- Normal, user-specific, peak token, image, per-request, and tiered pricing match the established billing sequence.
- Desktop uses a channel rail and price detail panel.
- Mobile uses a bottom channel picker and full-width price cards.
- Search, platform, and priced-only filters work together.
- Refresh preserves existing content and selection.
- Partial user-rate failure is visible and does not masquerade as final pricing.
- No page-level horizontal overflow at 390px, 768px, or 1440px.
- Light/dark themes, keyboard controls, ARIA state, and focus restoration pass manual checks.
- Focused tests, lint, typecheck, and production build pass.
