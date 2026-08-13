# Channel Navigation Metadata Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make every desktop channel navigation item display its platform badges and group/model counts on consistent separate rows.

**Architecture:** Preserve the existing catalog, selection state, 240px desktop rail, and mobile picker. Change only the navigation item's internal markup/classes and lock the hierarchy with a Vue component test.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vue Test Utils, Vitest

---

### Task 1: Lock the navigation metadata hierarchy

**Files:**
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`

**Step 1: Write the failing test**

Extend the desktop rail test to require separate `channel-nav-platforms` and `channel-nav-counts` rows. Assert both are block/flex rows, the count row has stable top spacing, and their parent is not the old shared wrapping container.

**Step 2: Run the focused test to verify it fails**

Run: `npx vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

Expected: FAIL because the two dedicated rows do not exist yet.

**Step 3: Implement the minimal layout change**

Replace the shared badge/count wrapper with two independent rows. Keep badge wrapping within its row and keep counts together on the row below.

**Step 4: Run focused and frontend verification**

Run:

```bash
npx vitest run src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
npx eslint src/components/channels/AvailableChannelCatalog.vue src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
npx vue-tsc --noEmit
npm run build
```

Expected: all commands pass.

**Step 5: Review the diff**

Confirm only the navigation item layout and its regression test changed; no catalog data, pricing, mobile picker, or selection behavior changed.
