# Custom Groups Entry Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move the custom-group entry from the sidebar into the API-key toolbar.

**Architecture:** Keep the existing `/custom-groups` route and page. Change only navigation composition: remove sidebar items and add a router link beside the create-key action.

**Tech Stack:** Vue 3, Vue Router, TypeScript, Vitest.

---

### Task 1: Pin the navigation contract

**Files:**
- Modify: `frontend/src/views/user/__tests__/KeysViewLayout.spec.ts`
- Create: `frontend/src/components/layout/__tests__/AppSidebarCustomGroups.spec.ts`

**Step 1:** Add failing source-contract tests asserting that `KeysView.vue` links to `/custom-groups` and `AppSidebar.vue` does not add that route.

**Step 2:** Run `pnpm test --run src/views/user/__tests__/KeysViewLayout.spec.ts src/components/layout/__tests__/AppSidebarCustomGroups.spec.ts` and confirm failure.

### Task 2: Move the entry

**Files:**
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/components/layout/AppSidebar.vue`

**Step 1:** Add a secondary `RouterLink` button for `/custom-groups` beside the create-key button.

**Step 2:** Remove both sidebar item insertions for `/custom-groups`.

**Step 3:** Run the focused tests and confirm success.

### Task 3: Verify and commit

**Step 1:** Run `pnpm typecheck`.

**Step 2:** Run the frontend test suite.

**Step 3:** Build the frontend and restart the local backend so the embedded UI is current.

**Step 4:** Commit the implementation.
