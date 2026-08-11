# Remove Priced-Only Filter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the misleading “仅显示有价模型” control from the available-channels toolbar without changing channel data or pricing behavior.

**Architecture:** Remove the control at the toolbar component boundary and delete its parent view state/binding. Keep the catalog filter capability internal and pass `pricedOnly: false` so the change remains UI-only. Resize the toolbar grid to fit search, platform, and refresh controls.

**Tech Stack:** Vue 3, TypeScript, Vitest, Vue Test Utils, Tailwind CSS

---

### Task 1: Lock the removed-control behavior with tests

**Files:**
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts`
- Modify: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`

**Step 1: Write the failing tests**

- Remove `pricedOnly` from toolbar test props.
- Change the synchronization test to assert search and platform events still emit and `priced-only-filter` does not exist.
- Change layout expectations to mobile two-column and desktop three-column classes.
- Change the view integration test to assert the removed control is absent while platform filtering still works.

**Step 2: Run tests to verify they fail**

Run:

```bash
pnpm --dir frontend exec vitest run \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/views/user/__tests__/AvailableChannelsView.spec.ts --reporter=dot
```

Expected: FAIL because the old control still renders and the old grid classes remain.

### Task 2: Remove the UI control and dead bindings

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelsToolbar.vue`
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`

**Step 1: Write the minimal implementation**

- Delete the checkbox label from the toolbar template.
- Delete the `pricedOnly` prop and `update:pricedOnly` event.
- Change the toolbar grid to `grid-cols-[minmax(0,1fr)_auto]` and `sm:grid-cols-[minmax(0,1fr)_minmax(10rem,13rem)_auto]`.
- Change the search shell span from two/three rows to `col-span-2 sm:col-span-1`.
- Delete the view’s `v-model:priced-only` binding and `pricedOnly` ref.
- Keep catalog filtering behavior explicit with `pricedOnly: false`.

**Step 2: Run focused tests**

Run:

```bash
pnpm --dir frontend exec vitest run \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/components/channels/__tests__/availableChannelCatalogModel.spec.ts \
  src/views/user/__tests__/AvailableChannelsView.spec.ts --reporter=dot
```

Expected: all focused tests PASS.

**Step 3: Run static and production verification**

Run:

```bash
pnpm --dir frontend typecheck
pnpm --dir frontend build
```

Expected: both commands exit 0.

**Step 4: Review the diff**

Run:

```bash
git diff --check
git diff -- frontend/src/components/channels/AvailableChannelsToolbar.vue \
  frontend/src/views/user/AvailableChannelsView.vue \
  frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts
```

Expected: only the priced-only UI/state removal, responsive grid adjustment, and matching tests are present.
