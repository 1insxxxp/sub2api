# Available Channel Platform Select Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the native available-channel platform selector with the project's shared Select component while preserving filtering behavior and responsive layout.

**Architecture:** `AvailableChannelsToolbar` will adapt its existing platform strings into `SelectOption[]` and bridge the shared component's `update:modelValue` event back to the unchanged `update:platform` contract. A local deep style will align the shared trigger with the toolbar's 44px controls without changing global Select styles.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vue Test Utils, Vitest.

---

### Task 1: Specify the shared selector behavior

**Files:**
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts`

**Step 1: Update the synchronization test**

Import the shared `Select` component, locate it with `getComponent(Select)`, assert that its `modelValue` is the current platform, and emit `update:modelValue` with `anthropic`. Keep the existing assertions for search and priced-only updates.

**Step 2: Add a shared-style regression test**

Assert that the platform filter:

- is the shared `Select` component rather than a native `select`;
- receives `searchable: false` and the translated platform aria-label;
- receives options `全部平台`, `anthropic`, and `openai` with values `''`, `anthropic`, and `openai`;
- keeps the wrapper's responsive grid placement and a local 44px trigger style.

**Step 3: Run the test to verify RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
```

Expected: FAIL because the toolbar still renders a native `select` and has no shared Select props.

### Task 2: Replace the native selector

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelsToolbar.vue`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts`

**Step 1: Build platform options**

Import `computed`, `Select`, and `SelectOption`. Store `withDefaults(defineProps(...))` in `props` and define:

```ts
const platformOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('availableChannels.catalog.allPlatforms') },
  ...props.platforms.map(value => ({ value, label: value })),
])

function updatePlatform(value: SelectOption['value']) {
  emit('update:platform', typeof value === 'string' ? value : '')
}
```

**Step 2: Render the shared Select**

Replace the native element with:

```vue
<Select
  data-testid="platform-filter"
  class="platform-filter min-w-0"
  :model-value="platform"
  :options="platformOptions"
  :searchable="false"
  :aria-label="t('availableChannels.catalog.platformFilter')"
  @update:model-value="updatePlatform"
/>
```

**Step 3: Align the trigger locally**

Add scoped styling for `.platform-filter :deep(.select-trigger)` so the trigger is `h-11`, has the toolbar's rounded-xl/slate border/background/padding, and remains full width. Do not modify `components/common/Select.vue`.

**Step 4: Run the focused test to verify GREEN**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
```

Expected: PASS.

**Step 5: Run channel regressions and static checks**

```bash
cd frontend
npx vitest run src/components/channels/__tests__
npx vue-tsc --noEmit --pretty false
npx eslint src/components/channels/AvailableChannelsToolbar.vue src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
```

Expected: all commands exit 0.

**Step 6: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelsToolbar.vue frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts
git commit -m "fix: unify available channel platform selector"
```
