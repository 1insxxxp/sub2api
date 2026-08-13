# Groups Toolbar Create Actions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure both group creation buttons remain visible at normal desktop widths.

**Architecture:** Keep the existing GroupsView toolbar and business handlers, but override its responsive breakpoint so filters and actions use separate rows until `2xl`. Group the two creation buttons into a dedicated right-aligned cluster so secondary actions cannot separate or hide them.

**Tech Stack:** Vue 3, Tailwind CSS, Vue Test Utils, Vitest, TypeScript.

---

### Task 1: Add the responsive toolbar regression test

**Files:**
- Test: `frontend/src/views/admin/__tests__/GroupsView.systemCustom.spec.ts`

**Step 1: Write the failing test**

Mount `GroupsView` and assert that:

- `groups-toolbar` is column-oriented at `lg` and row-oriented at `2xl`.
- `groups-toolbar-actions` is full-width at `lg` and auto-width at `2xl`.
- `groups-create-actions` contains both `system-custom-create` and `groups-create-btn`.

**Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir frontend exec vitest run src/views/admin/__tests__/GroupsView.systemCustom.spec.ts -t 'keeps both create actions visible'
```

Expected: FAIL because the responsive test IDs and layout classes do not yet exist.

### Task 2: Implement the minimal toolbar layout fix

**Files:**
- Modify: `frontend/src/views/admin/GroupsView.vue:6-116`

**Step 1: Adjust the toolbar breakpoints**

Keep the toolbar stacked through `lg`, restore a horizontal layout at `2xl`, and keep the actions row full-width until `2xl`.

**Step 2: Group creation actions**

Wrap both creation buttons in a right-aligned flex group and keep their labels on one line.

**Step 3: Run the focused test**

Run the command from Task 1.

Expected: PASS.

### Task 3: Verify and commit

**Files:**
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Test: `frontend/src/views/admin/__tests__/GroupsView.systemCustom.spec.ts`

**Step 1: Run related tests**

```bash
pnpm --dir frontend exec vitest run src/views/admin/__tests__/GroupsView.systemCustom.spec.ts src/views/admin/__tests__/GroupsView.spec.ts src/views/admin/__tests__/GroupsView.duplicate.spec.ts
```

**Step 2: Run frontend gates**

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run typecheck:tests
pnpm --dir frontend exec eslint src/views/admin/GroupsView.vue src/views/admin/__tests__/GroupsView.systemCustom.spec.ts
pnpm --dir frontend run build
```

**Step 3: Commit**

```bash
git add frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/__tests__/GroupsView.systemCustom.spec.ts
git commit -m "fix: keep group creation actions visible"
```
