# Admin List Actions Without Refresh Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Update upstream billing probe results and edited groups in place without reloading their paginated admin lists.

**Architecture:** Reuse the complete mutation responses already returned by the admin APIs and merge them into Vue list state. Remove the post-probe sorted-list synchronization so manual probes never escalate to the full account loader.

**Tech Stack:** Vue 3 Composition API, TypeScript, Vitest, Vue Test Utils, pnpm.

---

### Task 1: Prove Group Edits Can Update In Place

**Files:**
- Modify: `frontend/src/views/admin/__tests__/GroupsView.duplicate.spec.ts`
- Modify: `frontend/src/views/admin/GroupsView.vue`

**Step 1: Write the failing test**

Add a test that opens the edit dialog, submits it, and has `updateGroup` return a changed group without list-only count fields. Assert that:

```ts
expect(listGroups).toHaveBeenCalledTimes(1)
expect(wrapper.text()).toContain('Updated Primary')
```

Also assert that the existing `account_count`, `active_account_count`, and `rate_limited_account_count` values remain on the merged row.

**Step 2: Run the test to verify it fails**

Run:

```bash
cd frontend
pnpm vitest run src/views/admin/__tests__/GroupsView.duplicate.spec.ts
```

Expected: FAIL because `handleUpdateGroup` calls `loadGroups()` after saving.

**Step 3: Write the minimal implementation**

Capture the API response and replace the matching row with a shallow merge:

```ts
const updatedGroup = await adminAPI.groups.update(editingGroup.value.id, payload)
groups.value = groups.value.map(group =>
  group.id === updatedGroup.id ? { ...group, ...updatedGroup } : group
)
```

Close the dialog and show the success message without calling `loadGroups()`.

**Step 4: Run the test to verify it passes**

Run the focused group suite again. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/__tests__/GroupsView.duplicate.spec.ts
git commit -m "fix(admin): update edited groups without reloading"
```

### Task 2: Keep Upstream Billing Probes Local

**Files:**
- Modify: `frontend/src/views/admin/__tests__/AccountsView.upstreamBillingRefresh.spec.ts`
- Modify: `frontend/src/views/admin/AccountsView.vue`

**Step 1: Write the failing test**

Replace the existing source assertions with regression assertions showing that the single and batch handlers call `patchUpstreamBillingSnapshot`, but neither calls a post-probe list refresh helper.

**Step 2: Run the test to verify it fails**

Run:

```bash
cd frontend
pnpm vitest run src/views/admin/__tests__/AccountsView.upstreamBillingRefresh.spec.ts
```

Expected: FAIL because both handlers currently call `refreshAccountsAfterUpstreamBillingProbe()`.

**Step 3: Write the minimal implementation**

Remove `refreshAccountsAfterUpstreamBillingProbe` and its two call sites. Preserve the existing row patching and notifications.

**Step 4: Run the test to verify it passes**

Run the focused account suite again. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/AccountsView.vue frontend/src/views/admin/__tests__/AccountsView.upstreamBillingRefresh.spec.ts
git commit -m "fix(admin): keep upstream rate probes in place"
```

### Task 3: Verify Frontend Regression Safety

**Files:**
- Verify: `frontend/src/views/admin/GroupsView.vue`
- Verify: `frontend/src/views/admin/AccountsView.vue`

**Step 1: Run focused tests**

```bash
cd frontend
pnpm vitest run src/views/admin/__tests__/GroupsView.duplicate.spec.ts src/views/admin/__tests__/AccountsView.upstreamBillingRefresh.spec.ts
```

Expected: all tests PASS.

**Step 2: Run type checking**

```bash
pnpm type-check
```

Expected: exit code 0.

**Step 3: Run the production build**

```bash
pnpm build
```

Expected: exit code 0.

**Step 4: Inspect the final diff**

```bash
git diff --check
git status --short
```

Expected: no whitespace errors and only intended files changed.
