# Available Channels Unified Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the fragmented available-channels navigation, filters, and channel summary cards with one continuous desktop rail and one unified content header while preserving all catalog behavior and pricing logic.

**Architecture:** Keep `AvailableChannelCatalog` as the owner of channel selection and responsive navigation. Move the selected-channel summary into `AvailableChannelsToolbar` through explicit presentation props so the right header is a single component, while the catalog renders one left rail container spanning its heading and list. `AvailableChannelsView` continues to own API data and filtering and only passes selected presentation data through the catalog toolbar slot.

**Tech Stack:** Vue 3 SFCs, TypeScript, Tailwind CSS, Vue Test Utils, Vitest, vue-i18n.

---

### Task 1: Lock the unified layout contract with failing tests

**Files:**
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts`
- Modify: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`

**Step 1: Write the failing catalog layout test**

Assert that the desktop layout uses a `240px` rail, has exactly one `channel-navigation-shell`, does not render the old `channel-navigation-heading`, and places the toolbar and detail in the right column.

**Step 2: Run the catalog test to verify RED**

Run:

```bash
cd frontend && npx vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: FAIL because the current catalog still renders a separate heading card and `260px` rail.

**Step 3: Write the failing toolbar composition test**

Mount the toolbar with selected channel name, description, platforms, group count, and model count. Assert one `channel-context` region appears before `channel-filter-row`, with all controls retained.

**Step 4: Run the toolbar test to verify RED**

Run:

```bash
cd frontend && npx vitest run src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
```

Expected: FAIL because the toolbar accepts no channel context props.

**Step 5: Write the failing view integration assertion**

Assert the page renders the selected channel context inside the toolbar and no separate detail header card.

**Step 6: Run the view test to verify RED**

Run:

```bash
cd frontend && npx vitest run src/views/user/__tests__/AvailableChannelsView.spec.ts
```

Expected: FAIL because the context is currently rendered by `AvailableChannelCatalog` below the filters.

**Step 7: Commit tests**

```bash
git add frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts
git commit -m "test: define unified available channels layout"
```

### Task 2: Build the continuous desktop channel rail

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Step 1: Replace the four-cell desktop grid with a two-column shell**

Use `xl:grid-cols-[240px_minmax(0,1fr)]`. Render a single `aside[data-testid="channel-navigation-shell"]` containing the title/count header and listbox. Keep sticky positioning, max-height, keyboard behavior, and independent scrolling.

**Step 2: Flatten channel rows**

Use one consistent compact two-line row for channel name, platform badges, group count, and model count. Keep `role="option"`, `aria-selected`, focus ring, keyboard navigation, and `aria-controls` unchanged.

**Step 3: Remove the separate selected-channel header**

Keep the right detail region and model/group rendering, but delete the old bordered channel summary card. Expose the selected channel to the toolbar slot using scoped slot props:

```vue
<slot name="toolbar" :selected-channel="selectedChannel" />
```

**Step 4: Preserve empty/loading/fallback states**

Keep the toolbar visible in no-results mode, keep rate-fallback and refreshing status semantics, and retain mobile `AvailableChannelPicker` behavior.

**Step 5: Run the catalog test to verify GREEN**

```bash
cd frontend && npx vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelCatalog.vue frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
git commit -m "refactor: flatten available channel navigation"
```

### Task 3: Merge channel context and filters into one right header

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelsToolbar.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts`
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`
- Modify: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`

**Step 1: Add presentation-only channel props**

Add optional props for `channelName`, `channelDescription`, `channelPlatforms`, `groupCount`, and `modelCount`. These props display catalog-derived values only and perform no aggregation or pricing.

**Step 2: Render the unified header**

Use one bordered surface with a context row and a filter row separated by a subtle rule. The context row contains name/description, platform chips, and compact statistics. The filter row keeps search flexible, platform select stable, priced-only compact, and refresh 44px.

**Step 3: Pass scoped slot values from the page**

Update the page toolbar slot to receive `selectedChannel` and pass its fields to `AvailableChannelsToolbar`. In no-data/no-results states, render neutral page context without fabricating channel data.

**Step 4: Implement responsive behavior**

At desktop widths keep the filter row aligned. Below `sm`, place search on the first row and platform/toggle/refresh on the second. Ensure `min-w-0`, wrapping, and `[overflow-wrap:anywhere]` prevent horizontal overflow.

**Step 5: Run toolbar and integration tests to verify GREEN**

```bash
cd frontend && npx vitest run src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelsToolbar.vue frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts frontend/src/views/user/AvailableChannelsView.vue frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts
git commit -m "feat: unify available channel context and filters"
```

### Task 4: Verify behavior, accessibility, and visual layout

**Files:**
- Modify only if a regression test first demonstrates a problem.

**Step 1: Run focused catalog suites**

```bash
cd frontend && npx vitest run \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelListView.spec.ts \
  src/views/user/__tests__/AvailableChannelsView.spec.ts
```

Expected: PASS.

**Step 2: Run static verification**

```bash
cd frontend && npm run lint:check && npm run typecheck && npm run build
```

Expected: all commands exit 0.

**Step 3: Run full frontend tests**

```bash
cd frontend && npx vitest run
```

Expected: all tests pass, with only documented TODO tests allowed.

**Step 4: Perform browser QA**

Inspect 1440px, 768px, 390px, and 320px widths. Confirm one continuous left rail on desktop, one unified right header, aligned model content, usable mobile picker, and `scrollWidth === clientWidth` on narrow screens.

**Step 5: Commit any test-first QA corrections**

```bash
git add <only corrected source and regression test files>
git commit -m "fix: harden available channels responsive layout"
```
