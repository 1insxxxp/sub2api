# Theme Color Unification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the authenticated app UI consistently use primary blue, slate neutrals, and small cyan accents while preserving semantic status colors.

**Architecture:** This is a frontend-only visual pass. Update high-visibility Vue components first, then shared utility classes, and verify with typecheck, tests, build, and browser screenshots. Do not restart the existing local dev services unless the current process is broken.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Pinia, Vite, Vitest.

---

### Task 1: Normalize Shared Theme Utilities

**Files:**
- Modify: `frontend/src/style.css`

**Step 1: Inspect current shared classes**

Run:

```powershell
rg -n "btn-success|btn-warning|badge-purple|stat-success|alert-success|emerald|purple|violet|indigo|teal|cyan" frontend/src/style.css
```

Expected: Identify shared classes that are decorative versus semantic.

**Step 2: Update decorative shared styles**

Keep semantic classes such as success, warning, and danger intact. Replace decorative purple/indigo utility classes with primary blue or slate variants. If a class name is semantic, such as `badge-success`, keep green.

**Step 3: Verify CSS compiles through typecheck later**

Do not run the full suite yet. This task is completed after the file is updated and reviewed.

### Task 2: Unify User Dashboard Cards

**Files:**
- Modify: `frontend/src/components/user/dashboard/UserDashboardStats.vue`
- Modify: `frontend/src/components/user/dashboard/UserDashboardQuickActions.vue`
- Modify if needed: `frontend/src/components/user/dashboard/UserDashboardRecentUsage.vue`
- Modify if needed: `frontend/src/components/user/dashboard/UserDashboardCharts.vue`

**Step 1: Replace decorative card colors**

Use blue/slate/cyan styles for stat icon backgrounds and emphasized values. Keep green only for active/success metrics and amber only for quota thresholds or reward/warning meaning.

**Step 2: Keep layout dimensions stable**

Do not change grid structure or text wrapping behavior. Confirm the stat cards remain responsive.

**Step 3: Run focused typecheck later**

No behavioral tests should need changes unless snapshots or class assertions exist.

### Task 3: Unify Admin Dashboard Cards And Charts

**Files:**
- Modify: `frontend/src/views/admin/DashboardView.vue`

**Step 1: Replace decorative stat card colors**

Move user, system, cost, token, and performance cards to blue/slate/cyan variants. Preserve green for success deltas and amber/red for warning or failure thresholds.

**Step 2: Update chart palette**

Replace the multi-color palette with a controlled set:

```ts
[
  '#2563eb',
  '#0891b2',
  '#475569',
  '#60a5fa',
  '#0e7490',
  '#94a3b8',
  '#f59e0b',
  '#ef4444'
]
```

Use amber and red only for series where warning/error meaning is useful; otherwise prefer blue, cyan, and slate.

### Task 4: Normalize Key Usage Page

**Files:**
- Modify: `frontend/src/views/KeyUsageView.vue`

**Step 1: Keep status and quota semantics**

Preserve green/amber/red quota thresholds and active/error status dots.

**Step 2: Replace decorative icon accents**

Change non-semantic icon cards such as billing, quota summary, and token summary from emerald/indigo to blue/slate/cyan.

**Step 3: Preserve standalone page behavior**

Do not alter API calls, route params, loading states, or error states.

### Task 5: Sweep High-Visibility Admin Accents

**Files:**
- Modify as needed: `frontend/src/views/admin/UsersView.vue`
- Modify as needed: `frontend/src/views/admin/CheckinsView.vue`
- Modify as needed: `frontend/src/views/admin/AccountsView.vue`
- Modify as needed: `frontend/src/views/admin/GroupsView.vue`
- Modify as needed: `frontend/src/views/admin/orders/AdminOrdersView.vue`

**Step 1: Change decorative purple/indigo/violet accents**

Use blue or slate for role badges, exclusive group chips, refund actions, and tool menu icons when the old color was decorative.

**Step 2: Preserve semantic state colors**

Keep green active, amber warning, red danger, and check-in reward amber.

**Step 3: Re-scan changed paths**

Run:

```powershell
rg -n "purple|violet|indigo|teal|emerald" frontend/src/views/admin frontend/src/components/user/dashboard frontend/src/views/KeyUsageView.vue
```

Expected: Remaining hits are either semantic, low-priority operational charts, docs/examples, or intentionally deferred.

### Task 6: Verify Locally

**Files:**
- No source edits unless verification finds issues.

**Step 1: Typecheck**

Run:

```powershell
pnpm --dir frontend run typecheck
```

Expected: exit code 0.

**Step 2: Unit tests**

Run:

```powershell
pnpm --dir frontend run test:run
```

Expected: all tests pass.

**Step 3: Production build**

Run:

```powershell
pnpm --dir frontend run build
```

Expected: build exits 0.

### Task 7: Browser Visual Check

**Files:**
- No source edits unless visual check finds issues.

**Step 1: Open existing local frontend**

Use the running local service at:

```text
http://127.0.0.1:18080
```

**Step 2: Check core routes**

Inspect desktop and mobile widths for:

- `/dashboard`
- `/admin/dashboard`
- `/admin/users`
- `/admin/checkins`
- key usage route if a valid local URL is available.

Expected: colors are unified, dark mode remains legible, and no mobile horizontal overflow appears.

### Task 8: Commit Theme Sweep

**Files:**
- All modified UI files.
- Force-add docs because `docs/*` is ignored if needed.

**Step 1: Review diff**

Run:

```powershell
git diff --stat
git diff -- frontend/src docs/plans/2026-06-16-theme-color-unification-design.md docs/plans/2026-06-16-theme-color-unification-implementation.md
```

Expected: only theme/UI/doc changes.

**Step 2: Commit**

Run:

```powershell
git add -f docs/plans/2026-06-16-theme-color-unification-design.md docs/plans/2026-06-16-theme-color-unification-implementation.md
git add frontend/src
git commit -m "style: unify authenticated theme colors"
```

Expected: commit succeeds on the current branch.
