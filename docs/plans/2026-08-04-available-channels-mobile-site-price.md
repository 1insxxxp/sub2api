# Available Channels Mobile Site Price Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the mobile available-channels view show each model's site price using its source group's effective multiplier.

**Architecture:** Reuse the desktop table's existing group-scoped model branches and multiplier helpers inside the mobile card surface. Preserve a platform-scoped fallback for older or ungrouped API responses.

**Tech Stack:** Vue 3, TypeScript, Vitest, Vue Test Utils, Tailwind CSS

---

### Task 1: Render correctly priced group models on mobile

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelsTable.vue`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelsTable.spec.ts`

**Step 1: Write the failing test**

Extend the `SupportedModelChip` stub to expose `cnyPriceMultiplier`. Add a row with two groups, group-scoped models, distinct default/user multipliers, and assert the mobile surface renders each group name and passes the correct multiplier to each model chip.

**Step 2: Run test to verify it fails**

Run: `pnpm vitest run src/components/channels/__tests__/AvailableChannelsTable.spec.ts`

Expected: FAIL because the mobile branch renders only `section.supported_models` and does not pass `cnyPriceMultiplier`.

**Step 3: Write minimal implementation**

Mirror the desktop supported-model branch in the mobile surface. For group-scoped models, iterate through groups and pass `groupCNYMultiplier(g)`; otherwise iterate through section models and pass `sectionCNYMultiplier(section)`.

**Step 4: Run focused tests**

Run: `pnpm vitest run src/components/channels/__tests__/AvailableChannelsTable.spec.ts src/components/channels/__tests__/PricingRow.spec.ts`

Expected: PASS.

**Step 5: Verify frontend**

Run: `pnpm typecheck && pnpm build`

Expected: both commands succeed.

**Step 6: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelsTable.vue frontend/src/components/channels/__tests__/AvailableChannelsTable.spec.ts
git commit -m "fix: show mobile available channel site prices"
```

