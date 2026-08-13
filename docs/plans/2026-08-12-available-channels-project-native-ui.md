# Available Channels Project-Native UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rework the available-channels presentation into a compact project-native 240px rail and model-card experience without changing data, pricing, visibility, or billing behavior.

**Architecture:** Keep `AvailableChannelsView.vue` as the data and filter owner. Refine the existing catalog, toolbar, picker, model-list, and offering-card presentation components; continue consuming the normalized catalog and shared price/rate formatters as the only display data sources. Preserve the current responsive selection state and all API contracts.

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Tailwind CSS, Vue I18n, Vue Test Utils, Vitest.

---

### Task 1: Align the desktop catalog shell and navigation rail

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Step 1: Write the failing tests**

Add behavioral assertions that the desktop layout:

- retains the exact `240px + minmax(0,1fr)` grid;
- places rail and content in the same grid row and uses matching surface tokens;
- does not use the fixed `max-h-[640px]` navigation limit;
- exposes visible hover and `focus-visible` states on channel options;
- keeps vertical ArrowUp/ArrowDown/Home/End behavior.

**Step 2: Run the tests and verify RED**

Run:

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts --poolOptions.threads.singleThread
```

Expected: FAIL on the fixed rail height and new shared-surface/interaction assertions.

**Step 3: Implement the minimal layout change**

In `AvailableChannelCatalog.vue`:

- keep `xl:grid-cols-[240px_minmax(0,1fr)]`;
- align rail and content at the same top edge;
- use the project `border-slate-200/90 bg-white shadow-sm` surface tokens consistently;
- replace the fixed rail height with content-led scrolling owned by the page;
- keep `xl:sticky` only where it does not create a nested scroll trap;
- strengthen hover, selected, active, and `focus-visible` contrast without adding motion transforms.

Do not alter channel keys, selection fallback, model projection, or keyboard selection logic.

**Step 4: Run tests and verify GREEN**

Run the Task 1 Vitest command again. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelCatalog.vue \
  frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
git commit -m "fix: align available channel catalog shell"
```

### Task 2: Unify the channel header and filters

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelsToolbar.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts`

**Step 1: Write the failing tests**

Assert that:

- the header and filter row share one surface;
- search, platform selector, and refresh control all use 44px controls;
- search has `aria-label`, `name="available-channel-search"`, and `autocomplete="off"`;
- search and refresh controls have visible `focus-visible` styles;
- channel metadata remains visible and long descriptions wrap.

**Step 2: Run the tests and verify RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts --poolOptions.threads.singleThread
```

Expected: FAIL because the search accessibility attributes and project-native focus treatment are absent.

**Step 3: Implement the minimal toolbar change**

- keep the channel context as the first row and filters as the second row;
- use a stable responsive grid: search grows, platform selector has a bounded desktop width, refresh remains 44px;
- keep mobile controls wrapping without horizontal overflow;
- add the required form attributes and focus states;
- retain shared `Select.vue` and existing emits.

**Step 4: Run tests and verify GREEN**

Run the Task 2 Vitest command again. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelsToolbar.vue \
  frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
git commit -m "fix: refine available channel toolbar"
```

### Task 3: Simplify the model cards and price hierarchy

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelModelList.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelModelListView.spec.ts`
- Modify only if copy is missing: `frontend/src/locales/zh-CN/dashboard.ts`
- Modify only if copy is missing: `frontend/src/locales/en-US/dashboard.ts`

**Step 1: Write the failing tests**

Add assertions that model cards:

- have no gradient accent bar and no hover translate transform;
- use the project card surface and a single price comparison panel;
- keep the three-column price dimension grid with tabular numbers;
- expose a `content-visibility` optimization class or scoped CSS hook;
- show `查看渠道方案（N）`/`收起渠道方案` semantics and a decorative chevron;
- retain stable expansion by `entry.key` and price-range rendering.

**Step 2: Run the tests and verify RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelModelListView.spec.ts --poolOptions.threads.singleThread
```

Expected: FAIL on gradient/transform, content visibility, and action semantics.

**Step 3: Implement the minimal card redesign**

- remove the decorative gradient top bar and hover lift;
- use border-color and shadow changes only for hover;
- keep brand icon, model name, and platform badges compact;
- render site price as the strongest price, official price and savings adjacent, and dimensions beneath one divider;
- replace the current full-weight action with a compact footer action and chevron;
- add scoped `content-visibility: auto` and a conservative `contain-intrinsic-size` to model cards;
- continue using `summarizeOfferingPrice`, `formatCatalogMoney`, `formatCatalogMoneyRange`, and `compareOfferingPrice` without recalculating pricing.

**Step 4: Run tests and verify GREEN**

Run the Task 3 Vitest command again. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelModelList.vue \
  frontend/src/components/channels/__tests__/AvailableChannelModelListView.spec.ts \
  frontend/src/locales/zh-CN/dashboard.ts frontend/src/locales/en-US/dashboard.ts
git commit -m "fix: simplify available model cards"
```

### Task 4: Flatten channel offering details

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelOfferingCard.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts`

**Step 1: Write the failing tests**

Assert that each offering:

- renders as a compact bordered row instead of a shadowed nested card;
- keeps channel name, group name, platform, group rate, and effective-rate range visible;
- keeps official/site/peak prices and tier ordering intact;
- uses a desktop aligned grid and mobile wrapping classes;
- has no additional card shadow.

**Step 2: Run the tests and verify RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts --poolOptions.threads.singleThread
```

Expected: FAIL on nested-card and compact-row structure assertions.

**Step 3: Implement the minimal flattened details**

- turn the root into a neutral compact row with a single border;
- align channel/group metadata and rate values in a responsive header grid;
- render direct and tiered prices in light neutral cells without shadows;
- preserve all price/rate helper calls and unpriced states.

**Step 4: Run tests and verify GREEN**

Run the Task 4 Vitest command again. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelOfferingCard.vue \
  frontend/src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts
git commit -m "fix: flatten available channel offerings"
```

### Task 5: Finish mobile picker accessibility and interaction states

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelPicker.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelPicker.spec.ts`

**Step 1: Write the failing tests**

Assert that:

- the trigger and options have hover and `focus-visible` feedback;
- picker search has `aria-label`, `name="available-channel-picker-search"`, and `autocomplete="off"`;
- the sheet uses overscroll containment and safe-area padding;
- existing Escape, backdrop, selection, focus restoration, and body-lock behaviors remain intact.

**Step 2: Run the tests and verify RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelPicker.spec.ts --poolOptions.threads.singleThread
```

Expected: FAIL on the missing form and interaction attributes.

**Step 3: Implement the minimal picker polish**

- add the required accessibility attributes;
- add visible hover/focus states without changing picker state logic;
- add `overscroll-contain` to the sheet/list scrolling surfaces;
- keep 44px hit targets and `motion-reduce` behavior.

**Step 4: Run tests and verify GREEN**

Run the Task 5 Vitest command again. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelPicker.vue \
  frontend/src/components/channels/__tests__/AvailableChannelPicker.spec.ts
git commit -m "fix: polish available channel mobile picker"
```

### Task 6: Integration, accessibility, and responsive verification

**Files:**
- Modify only if regression coverage is needed: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`
- Inspect: `frontend/src/views/user/AvailableChannelsView.vue`

**Step 1: Add any missing integration regression tests**

Cover the real catalog and toolbar integration for:

- default first-channel selection;
- search and platform filters;
- refresh preserving old content;
- no-data and no-results states;
- multiplier fallback warning;
- selected-channel removal fallback.

**Step 2: Run the focused suite**

```bash
cd frontend
npx vitest run \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelListView.spec.ts \
  src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  src/views/user/__tests__/AvailableChannelsView.spec.ts \
  --poolOptions.threads.singleThread
```

Expected: PASS with zero failed tests.

**Step 3: Run static validation**

```bash
cd frontend
npx eslint \
  src/components/channels/AvailableChannelCatalog.vue \
  src/components/channels/AvailableChannelsToolbar.vue \
  src/components/channels/AvailableChannelModelList.vue \
  src/components/channels/AvailableChannelOfferingCard.vue \
  src/components/channels/AvailableChannelPicker.vue
npx vue-tsc --noEmit
npm run build
```

Expected: all commands exit 0.

**Step 4: Perform browser verification**

Using the local hot-reload URL, verify:

- desktop at a wide viewport: 240px rail and right content align, no nested horizontal scroll;
- mobile at 320px and 390px: picker, toolbar, cards, prices, and expanded offerings fit without horizontal overflow;
- keyboard focus is visible on channel navigation, search, select, refresh, expansion, and picker controls;
- reduced-motion styles disable nonessential animation.

**Step 5: Review the final diff**

```bash
git diff --check
git status --short
```

Confirm no API, catalog pricing, billing, or unrelated user files changed.

**Step 6: Commit final integration-only changes if needed**

```bash
git add frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts
git commit -m "test: verify available channels ui integration"
```
