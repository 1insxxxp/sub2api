# Mobile Group Selector Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make long group names fully readable and keep group dropdowns inside the viewport on mobile screens.

**Architecture:** Add an opt-in wrapping presentation to the shared `GroupBadge`, enable it from `GroupOptionItem`, and apply responsive viewport sizing to the two custom API-key group dropdowns. Preserve desktop behavior and existing data flow.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest, Vue Test Utils

---

### Task 1: Specify Shared Mobile Wrapping

**Files:**
- Modify: `frontend/src/components/common/__tests__/GroupOptionItem.spec.ts`
- Modify: `frontend/src/components/common/GroupOptionItem.vue`
- Modify: `frontend/src/components/common/GroupBadge.vue`

**Step 1: Write the failing test**

Add a long group name case that asserts `GroupOptionItem` enables the badge wrapping contract and uses a stacked mobile layout with the desktop horizontal layout restored at `sm`.

**Step 2: Run the test to verify it fails**

Run: `pnpm vitest run src/components/common/__tests__/GroupOptionItem.spec.ts`

Expected: FAIL because the wrap contract and responsive classes do not exist.

**Step 3: Implement the minimal shared behavior**

Add a `wrapName` boolean prop to `GroupBadge`. When enabled, replace the compact badge's no-wrap/truncate behavior with full-width, `whitespace-normal`, and `overflow-wrap:anywhere` name styling. Enable it in `GroupOptionItem`, stack its rate area below the name on mobile, and retain the current desktop row at `sm` and above.

**Step 4: Run the focused test**

Run: `pnpm vitest run src/components/common/__tests__/GroupOptionItem.spec.ts`

Expected: PASS.

### Task 2: Constrain Custom API-Key Dropdowns

**Files:**
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/components/admin/user/UserApiKeysModal.vue`
- Test: `frontend/src/views/user/__tests__/KeysView.spec.ts`
- Create or modify: `frontend/src/components/admin/user/__tests__/UserApiKeysModal.spec.ts`

**Step 1: Write failing responsive contract tests**

Assert that custom dropdowns use a mobile viewport width with an 8-pixel gutter and cap at the existing desktop width. Assert selected mobile badges opt into wrapping.

**Step 2: Run tests to verify they fail**

Run: `pnpm vitest run src/views/user/__tests__/KeysView.spec.ts src/components/admin/user/__tests__/UserApiKeysModal.spec.ts`

Expected: FAIL on missing responsive classes or positioning behavior.

**Step 3: Implement responsive positioning**

Use `width: min(380px, calc(100vw - 16px))` for the user dropdown and the corresponding capped width for the admin dropdown. Clamp both left positions to the viewport gutter. Enable badge wrapping in the selected group surface where narrow cards currently truncate the group name.

**Step 4: Run focused tests**

Run: `pnpm vitest run src/views/user/__tests__/KeysView.spec.ts src/components/admin/user/__tests__/UserApiKeysModal.spec.ts`

Expected: PASS.

### Task 3: Verify Frontend And Mobile Rendering

**Files:**
- Verify only

**Step 1: Run focused component tests**

Run: `pnpm vitest run src/components/common/__tests__/GroupOptionItem.spec.ts src/views/user/__tests__/KeysView.spec.ts src/components/admin/user/__tests__/UserApiKeysModal.spec.ts`

Expected: PASS.

**Step 2: Run type checking**

Run: `pnpm typecheck`

Expected: exit 0.

**Step 3: Run the production build**

Run: `pnpm build`

Expected: exit 0, allowing existing non-fatal Vite warnings.

**Step 4: Verify in the local browser**

At approximately 390px width, open the API-key group selector and confirm long names wrap fully, rate badges remain visible, the dropdown stays within the viewport, and the page has no horizontal overflow.

