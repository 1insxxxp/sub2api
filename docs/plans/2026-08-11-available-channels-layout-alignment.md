# Available Channels Layout Alignment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Align the available-channel navigation, filters, and model content on one responsive grid without changing model cards, catalog filtering, API behavior, or price calculations.

**Architecture:** Keep filter state and data loading in `AvailableChannelsView.vue`, extract the filter controls into a presentational toolbar, and pass that toolbar through a named slot into `AvailableChannelCatalog.vue`. The catalog owns the shared `260px + minmax(0, 1fr)` desktop grid so the left navigation heading/rail and right toolbar/detail use identical tracks; below `xl`, the layout becomes a single column and the toolbar uses deterministic responsive rows.

**Tech Stack:** Vue 3 Composition API, TypeScript, Tailwind CSS, Vue I18n, Vue Test Utils, Vitest.

---

### Task 1: Add the responsive filter toolbar

**Files:**
- Create: `frontend/src/components/channels/AvailableChannelsToolbar.vue`
- Create: `frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts`

**Step 1: Write the failing component tests**

Cover these observable contracts:

```ts
it('keeps search, platform, and priced-only values synchronized', async () => {
  const wrapper = mountToolbar({ platforms: ['anthropic', 'openai'] })
  await wrapper.get('[data-testid="channel-search"]').setValue('claude')
  await wrapper.get('[data-testid="platform-filter"]').setValue('anthropic')
  await wrapper.get('[data-testid="priced-only-filter"]').setValue(true)
  expect(wrapper.emitted('update:search')).toEqual([['claude']])
  expect(wrapper.emitted('update:platform')).toEqual([['anthropic']])
  expect(wrapper.emitted('update:pricedOnly')).toEqual([[true]])
})

it('emits refresh and disables the 44px refresh control while loading', async () => {
  const wrapper = mountToolbar({ loading: true })
  const refresh = wrapper.get('[data-testid="channel-refresh"]')
  expect(refresh.attributes('disabled')).toBeDefined()
  expect(refresh.classes()).toContain('h-11')
})
```

Also assert the root toolbar has the mobile single-column and desktop no-wrap/grid classes, the search control is full-width, and “全部平台” remains when `platforms` is empty.

**Step 2: Run the test to verify RED**

Run:

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
```

Expected: FAIL because `AvailableChannelsToolbar.vue` does not exist.

**Step 3: Implement the toolbar**

Create a presentational component with this API:

```ts
const props = withDefaults(defineProps<{
  search: string
  platform: string
  pricedOnly: boolean
  platforms: string[]
  loading?: boolean
}>(), { loading: false })

const emit = defineEmits<{
  'update:search': [value: string]
  'update:platform': [value: string]
  'update:pricedOnly': [value: boolean]
  refresh: []
}>()
```

Use a deterministic responsive layout:

- Root: single column on small screens, `sm:grid-cols-[minmax(0,1fr)_minmax(10rem,13rem)_auto_auto]` when space permits.
- Search input: `h-11 w-full min-w-0` with leading search icon.
- Platform select: `h-11 w-full min-w-0` and ellipsis-safe content.
- Priced-only control: compact bordered switch/pill with `min-h-11`, explicit checkbox, and visible focus state.
- Refresh: `h-11 w-11 shrink-0`, accessible title/label, disabled/loading state, and spin animation.

Do not call APIs or transform prices in this component.

**Step 4: Run the focused test to verify GREEN**

Run:

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
```

Expected: all toolbar tests PASS.

**Step 5: Commit Task 1**

```bash
git add frontend/src/components/channels/AvailableChannelsToolbar.vue \
  frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
git commit -m "feat: add aligned available channel filters"
```

### Task 2: Move filters into the catalog's shared grid

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Step 1: Write failing layout tests**

Add tests that mount the catalog with a `toolbar` slot and assert:

```ts
expect(wrapper.get('[data-testid="channel-catalog-layout"]').classes())
  .toContain('xl:grid-cols-[260px_minmax(0,1fr)]')
expect(wrapper.get('[data-testid="channel-navigation-heading"]').exists()).toBe(true)
expect(wrapper.get('[data-testid="channel-toolbar-region"]').text()).toContain('toolbar fixture')
expect(wrapper.get('[data-testid="channel-toolbar-region"]').classes()).toContain('xl:col-start-2')
expect(wrapper.get('[data-testid="channel-navigation"]').classes()).toContain('xl:col-start-1')
expect(wrapper.get('[data-testid="channel-detail"]').classes()).toContain('xl:col-start-2')
```

Retain assertions that the picker is the only channel selector below `xl`, empty/loading states render, the first channel is selected, and keyboard navigation works.

**Step 2: Run the catalog test to verify RED**

Run:

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: new slot/track/region assertions FAIL against the old `280px` layout.

**Step 3: Implement the shared catalog grid**

Refactor the non-empty catalog section so its desktop grid contains four aligned cells:

1. `channel-navigation-heading`, column 1 row 1.
2. `channel-toolbar-region`, column 2 row 1, rendering `<slot name="toolbar" />`.
3. Existing `channel-navigation`, column 1 row 2.
4. Existing `channel-detail`, column 2 row 2.

Use one grid definition:

```html
class="... xl:grid xl:grid-cols-[260px_minmax(0,1fr)] xl:gap-x-4 xl:gap-y-4"
```

The left heading displays the existing channel-navigation label plus channel count. Use the same border, radius, padding, and surface colors as the toolbar region. Keep warning/status rows spanning both columns. Keep the mobile picker above the detail content and hide both desktop-left cells below `xl`.

Do not change selection, model projection, pricing, or card components.

**Step 4: Run catalog regression tests**

Run:

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelListView.spec.ts
```

Expected: all tests PASS.

**Step 5: Commit Task 2**

```bash
git add frontend/src/components/channels/AvailableChannelCatalog.vue \
  frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
git commit -m "refactor: align available channel catalog columns"
```

### Task 3: Wire the toolbar into the page

**Files:**
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`
- Modify: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`

**Step 1: Write failing page integration assertions**

Update the integration test to assert that:

```ts
expect(wrapper.find('[data-testid="legacy-page-filters"]').exists()).toBe(false)
expect(wrapper.get('[data-testid="channel-toolbar-region"]').exists()).toBe(true)
expect(wrapper.get('[data-testid="channel-search"]').exists()).toBe(true)
```

Then drive the new test IDs to verify search, platform, priced-only and refresh still update the real catalog. Preserve the existing tests for old-content refresh, no-results/no-data, rate fallback, selection fallback, and mobile picker.

**Step 2: Run the view test to verify RED**

Run:

```bash
cd frontend
npx vitest run src/views/user/__tests__/AvailableChannelsView.spec.ts
```

Expected: new toolbar-region assertions FAIL before page wiring.

**Step 3: Implement page wiring**

- Remove the `TablePageLayout #filters` block.
- Import `AvailableChannelsToolbar`.
- Render it through `AvailableChannelCatalog #toolbar`.
- Bind the existing refs with explicit `:search`/`@update:search` bindings (or equivalent writable `v-model` arguments), pass `availablePlatforms` and `loading`, and route `@refresh` to `loadChannels`.
- Keep `TablePageLayout #table`, all computed catalog/filter data, API concurrency, error reporting, and loading semantics unchanged.

**Step 4: Run page and component regression tests**

Run:

```bash
cd frontend
npx vitest run src/views/user/__tests__/AvailableChannelsView.spec.ts \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: all tests PASS.

**Step 5: Commit Task 3**

```bash
git add frontend/src/views/user/AvailableChannelsView.vue \
  frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts
git commit -m "feat: unify available channel page layout"
```

### Task 4: Verify responsive behavior and production readiness

**Files:**
- Modify only if a verified regression requires it.

**Step 1: Run the complete relevant test set**

```bash
cd frontend
npx vitest run src/views/user/__tests__/AvailableChannelsView.spec.ts \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelListView.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelPrice.spec.ts
```

Expected: all tests PASS.

**Step 2: Run static verification**

```bash
cd frontend
npx eslint src/components/channels/AvailableChannelsToolbar.vue \
  src/components/channels/AvailableChannelCatalog.vue \
  src/views/user/AvailableChannelsView.vue \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/views/user/__tests__/AvailableChannelsView.spec.ts
npm run typecheck
npm run build
git diff --check
```

Expected: all commands exit 0.

**Step 3: Perform local visual QA**

Run the frontend from the feature worktree and inspect the available-channels route at approximately 1440px, 1024px, 768px, 390px, and 320px widths. Verify:

- Desktop left heading/rail and right toolbar/detail share exact column edges.
- Search/platform/toggle/refresh remain one aligned desktop row.
- Tablet and phone controls form deterministic rows with no horizontal overflow.
- Existing model cards and all official/site price values are visually unchanged.
- Mobile channel picker, focus states, dark mode, loading, refresh, empty, and fallback states remain usable.

**Step 4: Commit any verified QA-only fixes**

```bash
git add <only-files-changed-by-qa>
git commit -m "fix: polish available channel responsive alignment"
```

Skip this commit if visual QA requires no changes.
