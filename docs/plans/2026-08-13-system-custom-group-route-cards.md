# System Custom Group Route Cards Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove the model-list visual overlap and add bulk selection for every model route in the currently selected sources.

**Architecture:** Keep `selectedSourceIDs`, `visibleRoutes`, and `RouteDraft` as the only business state. Render each selected source as a normal-flow card inside the existing independent model scroller, disable scroll anchoring on that scroller, and derive a tri-state select-all control from `visibleRoutes`.

**Tech Stack:** Vue 3 Composition API, TypeScript, Tailwind CSS, Vue Test Utils, Vitest.

---

### Task 1: Add failing layout and bulk-selection contracts

**Files:**
- Modify: `frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts`

**Step 1: Write the failing card-layout test**

Select two sources and assert that the model scroller has `[overflow-anchor:none]`, every source section has card styling, and no source header uses `sticky`.

**Step 2: Write the failing select-all tests**

Assert that the control is disabled with no selected source, selects every visible route, becomes indeterminate after one route is cleared, and then cancels all visible routes. Also assert that routes belonging to an unselected source are unchanged.

**Step 3: Run tests to verify RED**

Run:

```bash
pnpm --dir frontend vitest run src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts -t "route cards|selects all visible model routes"
```

Expected: FAIL because the card contract and select-all control do not exist.

### Task 2: Implement route cards and tri-state select-all

**Files:**
- Modify: `frontend/src/components/admin/groups/SystemCustomGroupDialog.vue`

**Step 1: Replace sticky source sections with normal-flow cards**

Give each source section a rounded border and clipped overflow. Move the source title into a normal header row with its own subtle background and border. Keep model rows inside a padded body.

**Step 2: Disable browser scroll anchoring**

Add Tailwind arbitrary property `[overflow-anchor:none]` to the right model scroll container.

**Step 3: Add derived select-all state**

Create `allVisibleRoutesSelected` and `someVisibleRoutesSelected` computed values. Bind a checkbox's `checked` and DOM `indeterminate` state, and implement `toggleAllVisibleRoutes` to update only `visibleRoutes`.

**Step 4: Run focused tests to verify GREEN**

Run:

```bash
pnpm --dir frontend vitest run src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts
```

Expected: all dialog tests PASS.

### Task 3: Verify, commit, and refresh the local application

**Files:**
- Modify: `frontend/src/components/admin/groups/SystemCustomGroupDialog.vue`
- Modify: `frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts`

**Step 1: Run frontend gates**

```bash
pnpm --dir frontend vitest run \
  src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts \
  src/views/admin/__tests__/GroupsView.spec.ts \
  src/api/__tests__/admin.groups.systemCustom.spec.ts
pnpm --dir frontend run typecheck:tests
pnpm --dir frontend run typecheck
pnpm --dir frontend run lint:check
pnpm --dir frontend run build
```

Expected: every command exits `0`.

**Step 2: Commit the focused change**

```bash
git add docs/plans/2026-08-13-system-custom-group-route-cards-design.md \
  docs/plans/2026-08-13-system-custom-group-route-cards.md \
  frontend/src/components/admin/groups/SystemCustomGroupDialog.vue \
  frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts
git commit -m "fix: polish custom group model routing"
```

**Step 3: Rebuild only the local app container**

Build from this worktree's compose file, recreate only `sub2api`, then verify `/health` and confirm PostgreSQL/Redis container IDs remain unchanged.
