# Model Status Custom Group Filter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow authenticated users to use an active personal custom group as a preset for the models displayed on the Model Status page.

**Architecture:** Keep `/model-status` unchanged. Load active user custom groups through the existing authenticated API, encode selector values with a `custom:` prefix, and filter the existing public status report by `source_group_id` and `source_model`. Preserve the current public-group rendering and refresh lifecycle, with graceful fallback when custom-group data is unavailable or stale.

**Tech Stack:** Vue 3, TypeScript, Vitest, Vue Test Utils, existing `customGroupsAPI`, existing i18n and `Select` components.

---

### Task 1: Add failing coverage for custom-group loading and filtering

**Files:**
- Modify: `frontend/src/views/__tests__/ModelStatusView.spec.ts`

**Step 1: Extend test mocks and fixtures**

Mock `@/api/customGroups` and add a helper custom-group fixture whose models reference specific public group IDs and source model names. Keep the existing unauthenticated setup as the default.

**Step 2: Write the failing tests**

Add tests for:

- authenticated users loading active custom groups and seeing them in the selector;
- selecting `custom:<id>` showing only the referenced source models while preserving separate public groups for duplicate model names;
- disabled custom groups being excluded;
- a custom-group request failure leaving the public group selector and report usable;
- a custom group with no matching public status models rendering the existing no-match state.

**Step 3: Run the focused test file**

Run:

```bash
pnpm --dir frontend exec vitest run src/views/__tests__/ModelStatusView.spec.ts
```

Expected: the new tests fail because the view does not load or filter custom groups yet, while existing tests continue to pass.

### Task 2: Implement custom-group-backed selector state

**Files:**
- Modify: `frontend/src/views/ModelStatusView.vue`

**Step 1: Add custom-group state and selector values**

Import `customGroupsAPI` and `UserCustomGroup`. Add reactive state for active custom groups and a custom-group load error flag. Use a `custom:` value prefix and keep the existing public group filter values unchanged.

**Step 2: Load custom groups only for authenticated users**

During mount, request custom groups in parallel with the existing public settings/report flow when `authStore.isAuthenticated` is true. Treat failure as non-fatal. Do not make the request for guest users.

**Step 3: Build selector options**

Keep the existing “all public groups” and public group options, then append active custom-group options with a clear label. Avoid duplicate IDs and do not expose disabled custom groups.

**Step 4: Filter the report by source identity**

For a `custom:<id>` selection, build a set of `{ source_group_id, source_model }` entries from that custom group. Keep only matching models inside their original report groups, drop empty groups, and retain all original model metrics/buckets.

**Step 5: Handle invalid persisted selections**

When report or custom-group data finishes loading, clear a persisted public/custom selection if its referenced option is no longer available. Fall back to all public groups without breaking report rendering.

**Step 6: Keep refresh behavior consistent**

On report refresh, reapply the active custom-group filter. Reload custom groups on page visibility refresh or an equivalent lightweight refresh path so group edits become visible without a full navigation.

### Task 3: Update localization and selector presentation

**Files:**
- Modify: `frontend/src/i18n/locales/zh/modelStatus.ts`
- Modify: `frontend/src/i18n/locales/en/modelStatus.ts`
- Modify: `frontend/src/views/ModelStatusView.vue`

**Step 1: Add translation keys**

Add labels for “my custom groups”, custom-group loading failure, and the no matching monitored models state in both locales.

**Step 2: Make the selected label resilient**

Render the selected option label from the combined options list, including mobile-sheet selection, and avoid showing a raw `custom:<id>` value during loading.

**Step 3: Preserve mobile layout**

Keep the existing `Select` with `mobile-sheet`, allow the filter row to wrap, and verify the custom-group label truncates without pushing the model count off-screen.

### Task 4: Verify behavior and UI

**Files:**
- Modify if needed: `frontend/src/api/__tests__/customGroups.spec.ts`
- Modify if needed: `frontend/src/views/__tests__/ModelStatusView.spec.ts`

**Step 1: Run focused frontend tests**

```bash
pnpm --dir frontend exec vitest run \
  src/views/__tests__/ModelStatusView.spec.ts \
  src/api/__tests__/customGroups.spec.ts \
  src/api/__tests__/modelStatus.spec.ts
```

Expected: all focused tests pass.

**Step 2: Run frontend validation**

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run check:i18n
pnpm --dir frontend exec eslint \
  src/views/ModelStatusView.vue \
  src/views/__tests__/ModelStatusView.spec.ts \
  src/i18n/locales/zh/modelStatus.ts \
  src/i18n/locales/en/modelStatus.ts
git diff --check
```

Expected: all commands exit successfully.

**Step 3: Run a local browser smoke test**

With the existing Vite dev server running, verify `/model-status` at desktop and a 430px mobile viewport. Confirm the authenticated selector shows custom groups, selecting one reduces the model count/cards, the mobile sheet remains usable, and guest mode still shows only public groups.

**Step 4: Commit the implementation**

```bash
git add frontend/src/views/ModelStatusView.vue \
  frontend/src/views/__tests__/ModelStatusView.spec.ts \
  frontend/src/i18n/locales/zh/modelStatus.ts \
  frontend/src/i18n/locales/en/modelStatus.ts
git commit -m "feat: link model status filters to custom groups"
```
