# User Allowed Groups Checkbox Uniform Size Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make every checkbox in the admin user allowed-groups modal use the same 20px custom checkbox style so the exclusive/public group list no longer mixes different sizes.

**Architecture:** Keep the existing selection logic unchanged. Extract or inline one consistent 20px checkbox visual block and reuse it for both exclusive and public group rows. Preserve current click behavior, disabled/read-only behavior for the public-group display state, and the existing save payload.

**Tech Stack:** Vue 3 SFC, TypeScript, Tailwind CSS, Vitest, Vue Test Utils

---

### Task 1: Add a regression test for checkbox size consistency

**Files:**
- Modify: `frontend/src/components/admin/user/__tests__/UserAllowedGroupsModal.spec.ts`

**Step 1: Write the failing test**

Create a test that mounts the modal with at least one exclusive and one public group and asserts every rendered checkbox visual wrapper uses the same 20px dimensions / same custom checkbox class pattern, regardless of group type.

**Step 2: Run test to verify it fails**

Run: `pnpm vitest run frontend/src/components/admin/user/__tests__/UserAllowedGroupsModal.spec.ts -t "checkboxes use the same 20px custom size"`
Expected: FAIL because the public-group checkbox still uses a different native size.

**Step 3: Write minimal implementation**

Update the modal to use one shared custom checkbox markup for both group sections.

**Step 4: Run test to verify it passes**

Run: `pnpm vitest run frontend/src/components/admin/user/__tests__/UserAllowedGroupsModal.spec.ts -t "checkboxes use the same 20px custom size"`
Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/admin/user/UserAllowedGroupsModal.vue frontend/src/components/admin/user/__tests__/UserAllowedGroupsModal.spec.ts docs/plans/2026-09-09-user-allowed-groups-checkbox-uniform-size-design.md
git commit -m "fix: unify allowed-groups checkbox sizing"
```
