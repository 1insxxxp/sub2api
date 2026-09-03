# Lottery Page Layout Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce unused whitespace on the user lottery page and make prize details and draw history read as separate, balanced sections across desktop and mobile widths.

**Architecture:** Keep all existing lottery data, draw interactions, and localization unchanged. Update the page composition and utility classes in `LotteryView.vue`: contain the prize section, use responsive three/two/one-column grids, compact card internals, and add explicit spacing before history. Add a focused class-level assertion only if it improves regression coverage; do not introduce new runtime components or APIs.

**Tech Stack:** Vue 3 `<script setup>`, Tailwind utility classes, Vitest, Vue Test Utils, pnpm/Vite.

---

### Task 1: Add stable layout hooks to the prize and history sections

**Files:**
- Modify: `frontend/src/views/user/LotteryView.vue`

**Step 1: Inspect the existing section boundaries**

Confirm the prize details section, prize grid, prize card, and history section are the only markup that needs layout changes. Preserve all existing bindings and event handlers.

**Step 2: Add semantic layout classes**

Add stable classes for the contained prize panel, its grid, and the separated history panel so the visual rules are easy to reason about and future tests do not depend on generated utility strings.

**Step 3: Review the diff**

Run `git diff -- frontend/src/views/user/LotteryView.vue` and confirm no data or interaction code changed.

### Task 2: Implement the responsive compact prize layout

**Files:**
- Modify: `frontend/src/views/user/LotteryView.vue`

**Step 1: Write the layout change**

Contain the prize heading and cards in a lightly tinted rounded section, switch the grid to `sm:grid-cols-2 lg:grid-cols-3`, reduce card padding, and use a flex column with the value/inventory footer pushed to the bottom. Remove the fixed minimum description height that causes unnecessary vertical whitespace while retaining safe text wrapping.

**Step 2: Keep responsive and theme behavior intact**

Ensure the draw button remains full-width on phones, cards stack at the smallest breakpoint, and every new background/border class has a dark-mode counterpart.

**Step 3: Check the rendered structure statically**

Run `rg -n "lottery-prize-section|lottery-prize-grid|lottery-history-section|grid-cols" frontend/src/views/user/LotteryView.vue` and confirm the intended hooks and breakpoints are present.

### Task 3: Add or update focused regression coverage

**Files:**
- Modify: `frontend/src/views/user/__tests__/LotteryView.spec.ts` (only if needed)

**Step 1: Add a stable class assertion**

Mount the view with the existing mocks and assert the prize section/grid hooks render when prizes exist. Keep the existing assertion that prize weights never appear in user-facing markup.

**Step 2: Run the focused test**

Run `pnpm exec vitest run frontend/src/views/user/__tests__/LotteryView.spec.ts`.

Expected: all focused lottery view tests pass.

### Task 4: Verify the frontend and commit

**Files:**
- Modify: `frontend/src/views/user/LotteryView.vue`
- Modify: `frontend/src/views/user/__tests__/LotteryView.spec.ts` (if changed)

**Step 1: Run frontend checks**

Run:

```bash
pnpm exec vitest run frontend/src/views/user/__tests__/LotteryView.spec.ts
pnpm run typecheck
pnpm run lint
pnpm run build
```

Expected: focused tests, typecheck, lint, and production build all pass. Review any lint auto-fixes before committing.

**Step 2: Check repository hygiene**

Run `git diff --check` and `git status --short`.

Expected: no whitespace errors and only the intended lottery layout files plus the committed plan are present.

**Step 3: Commit the implementation**

```bash
git add frontend/src/views/user/LotteryView.vue frontend/src/views/user/__tests__/LotteryView.spec.ts
git commit -m "fix: improve lottery page layout"
```
