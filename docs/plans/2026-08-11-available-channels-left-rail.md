# Available Channels Left Rail Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restore a precisely aligned 240px desktop channel rail beside the available-model content while preserving mobile selection and all pricing behavior.

**Architecture:** Keep channel selection state in `AvailableChannelCatalog.vue` and change only its desktop shell from horizontal tabs to a two-column CSS grid. Reuse the existing channel data, badges, selection watcher, model projection, and mobile picker; tests define the layout and keyboard contract before production changes.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vue Test Utils, Vitest.

---

### Task 1: Lock the left-rail layout contract

**Files:**
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Step 1: Write the failing tests**

- Expect the populated desktop layout to use `xl:grid-cols-[240px_minmax(0,1fr)]`.
- Expect a vertical desktop channel rail with `aria-orientation="vertical"`.
- Expect toolbar and detail content to occupy the right column.
- Expect keyboard navigation to use ArrowUp/ArrowDown while retaining Home/End.
- Expect loading and empty shells to preserve the two-column desktop alignment where their respective content is available.

**Step 2: Run tests to verify RED**

Run:

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts --pool=threads --poolOptions.threads.singleThread=true
```

Expected: FAIL because the current component renders horizontal top tabs and no 240px desktop grid.

### Task 2: Implement the 240px desktop rail

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`

**Step 1: Build the desktop grid**

- Apply `xl:grid xl:grid-cols-[240px_minmax(0,1fr)]` to the populated layout.
- Render the desktop channel navigation in column one as a vertically scrolling card.
- Place rate fallback, refresh status, toolbar, and channel detail in column two.
- Preserve the mobile `AvailableChannelPicker` below `xl`.

**Step 2: Align behavior**

- Switch listbox orientation to vertical.
- Update roving keyboard handling to ArrowUp/ArrowDown.
- Keep selection, focus restoration, and model projection unchanged.

**Step 3: Run focused tests to verify GREEN**

Run the Task 1 Vitest command.

Expected: PASS.

### Task 3: Verify regressions and local HMR

**Files:**
- No production files expected beyond Tasks 1-2.

**Step 1: Run related suites**

```bash
cd frontend
npx vitest run \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts \
  src/views/user/__tests__/AvailableChannelsView.spec.ts \
  --pool=threads --poolOptions.threads.singleThread=true
```

Expected: PASS.

**Step 2: Run static verification**

```bash
cd frontend
npx eslint src/components/channels/AvailableChannelCatalog.vue src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
npx vue-tsc --noEmit
git diff --check
```

Expected: all commands exit 0.

**Step 3: Confirm local dev server**

- Confirm `http://127.0.0.1:3000` returns HTTP 200.
- Confirm Vite applies the layout without a rebuild.
