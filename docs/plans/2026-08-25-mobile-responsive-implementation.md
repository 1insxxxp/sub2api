# Mobile Responsive Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the shared application shell and the highest-traffic views usable at mobile widths without changing the desktop information architecture.

**Architecture:** Fix responsive behavior in shared `AppLayout`, `AppHeader`, `AppSidebar`, `BaseDialog`, `DataTable`, and `TablePageLayout` first. Then adjust page-specific dense layouts only where the shared primitives cannot solve the problem. Keep mobile adaptations inside existing Vue/Tailwind patterns and preserve desktop markup wherever possible.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest, Vue Test Utils, Vite.

---

### Task 1: Add shared shell overflow and mobile viewport regression coverage

**Files:**
- Modify: `frontend/src/components/layout/AppLayout.vue`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/style.css`
- Test: `frontend/src/components/layout/__tests__/AppLayout.spec.ts`
- Test: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`
- Test: `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`

**Step 1: Write the failing tests**

Add structural assertions for viewport-safe content, mobile header actions, and drawer behavior. Assert that the shell content uses `min-w-0`/overflow protection, the mobile action region does not render desktop-only labels, and the sidebar overlay remains limited to mobile widths.

**Step 2: Run tests to verify the expected failures**

Run: `cd frontend && pnpm vitest run src/components/layout/__tests__/AppLayout.spec.ts src/components/layout/__tests__/AppHeader.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts`

Expected: FAIL on the new assertions before implementation.

**Step 3: Implement the shared shell changes**

Use viewport-safe width constraints on the shell and main content, normalize mobile header control sizes and gaps, keep high-value actions visible, and ensure the mobile sidebar drawer cannot cause document-width overflow.

**Step 4: Run the focused tests**

Run the same Vitest command and expect PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/layout/AppLayout.vue frontend/src/components/layout/AppHeader.vue frontend/src/components/layout/AppSidebar.vue frontend/src/style.css frontend/src/components/layout/__tests__
git commit -m "fix: stabilize mobile app shell layout"
```

### Task 2: Center and constrain shared dialogs, toasts, and overlays

**Files:**
- Modify: `frontend/src/components/common/BaseDialog.vue`
- Modify: `frontend/src/components/common/ConfirmDialog.vue`
- Modify: `frontend/src/components/common/Toast.vue`
- Modify: `frontend/src/style.css`
- Test: `frontend/src/components/common/__tests__/BaseDialog.spec.ts`
- Test: `frontend/src/components/common/__tests__/Toast.spec.ts`

**Step 1: Write the failing tests**

Add assertions that dialog content has a mobile-safe max width/height, body scrolling is internal, action footers remain visible, and toast width is bounded by the viewport.

**Step 2: Run focused tests and verify failure**

Run: `cd frontend && pnpm vitest run src/components/common/__tests__/BaseDialog.spec.ts src/components/common/__tests__/Toast.spec.ts`

Expected: FAIL on the new mobile structure assertions.

**Step 3: Implement the dialog and toast rules**

Keep dialogs centered on mobile, remove bottom-sheet transforms where they are shared behavior, add `min-width: 0` to content regions, and use `max-width: calc(100vw - 1rem)` plus bounded internal scrolling. Keep short action sheets unchanged where a component explicitly opts into that interaction.

**Step 4: Run focused tests**

Run the same command and expect PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/common/BaseDialog.vue frontend/src/components/common/ConfirmDialog.vue frontend/src/components/common/Toast.vue frontend/src/style.css frontend/src/components/common/__tests__
git commit -m "fix: make shared dialogs mobile safe"
```

### Task 3: Make shared tables and pagination readable on mobile

**Files:**
- Modify: `frontend/src/components/common/DataTable.vue`
- Modify: `frontend/src/components/layout/TablePageLayout.vue`
- Modify: `frontend/src/components/common/Pagination.vue`
- Test: `frontend/src/components/common/__tests__/DataTable.spec.ts`
- Test: `frontend/src/components/layout/__tests__/TablePageLayout.spec.ts`

**Step 1: Write the failing tests**

Cover selectable rows, action placement, long values, and pagination controls at mobile widths. Ensure the mobile renderer does not put the selection control after the main content and that long model/account names cannot widen the document.

**Step 2: Run the tests and verify failure**

Run: `cd frontend && pnpm vitest run src/components/common/__tests__/DataTable.spec.ts src/components/layout/__tests__/TablePageLayout.spec.ts`

Expected: FAIL on the new responsive behavior assertions.

**Step 3: Implement responsive table behavior**

Keep the existing card renderer for mobile, move primary selection/actions into stable positions, add controlled wrapping/truncation, contain desktop table scrolling inside the table stage, and make pagination wrap without clipping.

**Step 4: Run focused tests**

Run the same command and expect PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/common/DataTable.vue frontend/src/components/layout/TablePageLayout.vue frontend/src/components/common/Pagination.vue frontend/src/components/common/__tests__ frontend/src/components/layout/__tests__/TablePageLayout.spec.ts
git commit -m "fix: improve mobile tables and pagination"
```

### Task 4: Adapt high-frequency dense pages

**Files:**
- Modify: `frontend/src/views/admin/AdminWorkbenchView.vue`
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Modify: `frontend/src/views/user/KeysView.vue`
- Modify: `frontend/src/views/user/SubscriptionsView.vue`
- Modify: `frontend/src/views/user/RedeemView.vue`
- Modify: `frontend/src/views/user/UsageView.vue`
- Modify: `frontend/src/views/user/ChannelStatusV2View.vue`
- Test: relevant existing view specs and focused mobile structure specs under `frontend/src/views/**/__tests__`

**Step 1: Write focused failing structure tests**

Assert single-column mobile grids, stable action placement, internal scrolling for dense panels, and no fixed desktop widths in the high-frequency views.

**Step 2: Run focused tests and verify failure**

Run the affected Vitest files with `pnpm vitest run` and confirm the new assertions fail.

**Step 3: Implement page-level adaptations**

Collapse multi-column forms and cards to one column, keep important actions above long content, contain calendars and usage records, and use the shared table/dialog behavior rather than adding page-specific mobile hacks.

**Step 4: Run the affected tests**

Run the changed view specs and expect PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/AdminWorkbenchView.vue frontend/src/views/admin/AccountsView.vue frontend/src/views/user/KeysView.vue frontend/src/views/user/SubscriptionsView.vue frontend/src/views/user/RedeemView.vue frontend/src/views/user/UsageView.vue frontend/src/views/user/ChannelStatusV2View.vue frontend/src/views/**/__tests__
git commit -m "fix: adapt high traffic views for mobile"
```

### Task 5: Adapt remaining complex pages and audit style regressions

**Files:**
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/views/admin/ChannelsView.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/views/ModelPlazaView.vue`
- Modify: relevant modal components under `frontend/src/components/admin`, `frontend/src/components/account`, and `frontend/src/components/payment`
- Test: add or update focused mobile specs alongside each affected component

**Step 1: Add failing tests for the remaining known layouts**

Cover wide group/channel forms, settings tab rows, model plaza cards, and nested dialogs.

**Step 2: Run the focused tests and verify failure**

Run the relevant Vitest files with `pnpm vitest run`.

**Step 3: Implement the remaining responsive rules**

Use stacked form sections, horizontally contained advanced tables, responsive card grids, and mobile-safe nested dialogs. Do not change business behavior or the current uncommitted commission calendar work.

**Step 4: Run focused tests**

Run the changed specs and expect PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/ChannelsView.vue frontend/src/views/admin/SettingsView.vue frontend/src/views/ModelPlazaView.vue frontend/src/components/admin frontend/src/components/account frontend/src/components/payment
git commit -m "fix: finish responsive layouts for complex pages"
```

### Task 6: Run full verification at mobile and desktop widths

**Files:**
- Verify: all changed frontend files

**Step 1: Run frontend tests and type checking**

Run:

```bash
cd frontend
pnpm test:run
pnpm typecheck
pnpm build
```

Expected: all tests pass, typecheck exits 0, and the production build completes.

**Step 2: Start the local app**

Run the existing local development command and use the local browser at 375px, 390px, 768px, and desktop width.

**Step 3: Check the responsive acceptance list**

Verify no document horizontal overflow, centered usable dialogs, visible navigation actions, readable cards/tables, stable pagination, and preserved desktop layout.

**Step 4: Review the diff**

Run `git status --short` and `git diff origin/dev...HEAD --stat`; confirm only the responsive work and the already committed design documents are included.

**Step 5: Report local verification**

Do not push or deploy until the user explicitly requests it after reviewing the local result.

