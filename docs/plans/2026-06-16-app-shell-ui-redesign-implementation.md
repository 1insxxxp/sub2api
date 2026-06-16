# App Shell UI Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refresh the authenticated app shell and shared UI primitives so user and admin pages visually match the refreshed homepage while preserving behavior.

**Architecture:** Update shared layout components and global Tailwind component classes first, allowing existing pages to inherit the new style. Keep route logic, data loading, stores, APIs, feature flags, and backend untouched.

**Tech Stack:** Vue 3, Vite, TypeScript, Tailwind CSS, Vitest, local Vite hot reload on `http://127.0.0.1:18080`.

---

### Task 1: Baseline Layout Verification

**Files:**
- Inspect: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`
- Inspect: `frontend/src/components/layout/__tests__`
- Modify only if needed: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`

**Step 1: Run existing layout tests**

Run:

```bash
pnpm --dir frontend run test:run -- src/components/layout/__tests__
```

Expected: Existing tests pass or reveal current failures unrelated to this redesign.

**Step 2: Add minimal structure tests if missing**

If there are no tests covering the header check-in popover, sidebar collapse state, or table layout wrapper, add small tests that assert stable element presence through `data-test` attributes or text-independent selectors.

**Step 3: Run the targeted tests again**

Run:

```bash
pnpm --dir frontend run test:run -- src/components/layout/__tests__
```

Expected: PASS.

**Step 4: Commit**

```bash
git add frontend/src/components/layout/__tests__
git commit -m "test: cover app shell layout primitives"
```

Skip this commit if no test changes were required.

---

### Task 2: Refresh Global UI Primitives

**Files:**
- Modify: `frontend/src/style.css`
- Modify if needed: `frontend/tailwind.config.js`

**Step 1: Tune shared component classes**

Update these classes in `frontend/src/style.css`:

- `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost`
- `.input`
- `.card`, `.card-hover`, `.card-glass`, `.stat-card`
- `.table-container`, `.table`, `.badge`, `.dropdown`, `.tabs`
- `.sidebar`, `.sidebar-header`, `.sidebar-link`, `.sidebar-link-active`
- `.page-title`, `.page-description`

Design constraints:

- Keep cards practical and not overly rounded.
- Use blue as the main accent.
- Keep semantic green/yellow/red states.
- Preserve dark mode classes.
- Preserve existing class names so pages do not need broad rewrites.

**Step 2: Run a targeted frontend build check**

Run:

```bash
pnpm --dir frontend run typecheck
```

Expected: PASS.

**Step 3: Commit**

```bash
git add frontend/src/style.css frontend/tailwind.config.js
git commit -m "style: refresh shared app primitives"
```

---

### Task 3: Refresh App Layout, Sidebar, And Header

**Files:**
- Modify: `frontend/src/components/layout/AppLayout.vue`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/components/layout/AppHeader.vue`

**Step 1: Update shell structure and spacing**

In `AppLayout.vue`:

- Refine page background and main content spacing.
- Keep sidebar offset logic unchanged.
- Preserve onboarding wiring.

In `AppSidebar.vue`:

- Improve logo/header surface.
- Improve active nav state, section spacing, collapsed state, and mobile overlay.
- Preserve navigation arrays, feature flags, simple mode behavior, and group expansion behavior.

In `AppHeader.vue`:

- Improve sticky header surface.
- Keep all actions: mobile menu, announcement bell, docs, locale switcher, subscription mini, balance, check-in, user dropdown.
- Keep check-in behavior unchanged; only polish its button/popover styling if needed.

**Step 2: Run layout tests**

Run:

```bash
pnpm --dir frontend run test:run -- src/components/layout/__tests__
```

Expected: PASS.

**Step 3: Commit**

```bash
git add frontend/src/components/layout/AppLayout.vue frontend/src/components/layout/AppSidebar.vue frontend/src/components/layout/AppHeader.vue
git commit -m "style: refine app shell layout"
```

---

### Task 4: Refresh Table Page Layout

**Files:**
- Modify: `frontend/src/components/layout/TablePageLayout.vue`

**Step 1: Improve table page frame**

Refine fixed action/filter sections, scrollable table surface, sticky table headers, row hover, and mobile mode. Keep slot names and layout behavior unchanged.

**Step 2: Run affected admin page tests**

Run:

```bash
pnpm --dir frontend run test:run -- src/views/admin/__tests__/UsersView.spec.ts src/views/admin/__tests__/CheckinsView.spec.ts src/views/admin/__tests__/UsageView.spec.ts
```

Expected: PASS.

**Step 3: Commit**

```bash
git add frontend/src/components/layout/TablePageLayout.vue
git commit -m "style: polish table page layout"
```

---

### Task 5: Browser Verification

**Files:**
- No code changes unless visual issues are found.

**Step 1: Verify local app is reachable**

Check:

```bash
curl http://127.0.0.1:18080/api/v1/settings/public
curl http://127.0.0.1:18081/health
```

Expected: HTTP 200 from frontend proxy/settings and backend health.

**Step 2: Use the in-app browser**

Open and inspect:

- `http://127.0.0.1:18080/home`
- `http://127.0.0.1:18080/dashboard`
- `http://127.0.0.1:18080/keys`
- `http://127.0.0.1:18080/admin/dashboard`
- `http://127.0.0.1:18080/admin/users`
- `http://127.0.0.1:18080/admin/accounts`
- `http://127.0.0.1:18080/admin/checkins`

Check:

- Light mode and dark mode.
- Desktop around 1280px.
- Mobile around 360px.
- No horizontal overflow.
- Header actions do not overlap.
- Sidebar collapsed and expanded states are usable.
- Table pages remain scrollable.

**Step 3: Fix any visual regressions**

If issues are found, patch the smallest shared component or class that caused them, then rerun the relevant tests.

**Step 4: Commit**

```bash
git add frontend/src
git commit -m "fix: resolve app shell visual regressions"
```

Skip this commit if no fixes were needed.

---

### Task 6: Final Verification

**Files:**
- No code changes expected.

**Step 1: Run frontend verification**

Run:

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
```

Expected: PASS.

**Step 2: Review git status**

Run:

```bash
git status --short --branch
```

Expected: Only intentional tracked changes or known untracked files such as `price.md`.

**Step 3: Final commit if needed**

If any uncommitted intentional changes remain:

```bash
git add frontend/src frontend/tailwind.config.js docs/plans/2026-06-16-app-shell-ui-redesign-design.md docs/plans/2026-06-16-app-shell-ui-redesign-implementation.md
git commit -m "feat: refresh authenticated app shell"
```

