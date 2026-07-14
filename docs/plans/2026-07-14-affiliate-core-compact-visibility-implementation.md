# Affiliate Core Compact Visibility Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the compact Core tier recognizable in static screenshots through a persistent badge perimeter and a Core-only animated rule-cell border.

**Architecture:** Keep the existing badge DOM and add a Core theme attribute to each tier rule cell. CSS uses that attribute for a contained pseudo-element border, while compact Core overrides strengthen the existing reactor perimeter and node. Media queries reduce touch motion and fully stop motion for reduced-motion users.

**Tech Stack:** Vue 3 SFC, scoped CSS, Vitest, Vue Test Utils

---

### Task 1: Define compact Core visibility contracts

**Files:**
- Modify: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Write the failing test**

Add assertions that each rule exposes `data-effect-theme`, Core has a persistent compact perimeter, the Core rule cell uses `tier-core-cell-idle`, desktop hover uses `tier-core-cell-burst`, touch uses the idle animation, and reduced motion stops the cell pseudo-element.

**Step 2: Run test to verify it fails**

Run: `pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

Expected: FAIL because rule-level Core hooks and cell animations do not exist.

### Task 2: Implement the visible Core rule treatment

**Files:**
- Modify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`

**Step 1: Add the Core rule hook**

Bind `:data-effect-theme="effectTheme(tier.level)"` to each `.tier-identity__rule`.

**Step 2: Add the static compact treatment**

Use a persistent blue-white hexagonal perimeter, a larger ignition node, and a contained Core-only rule-cell pseudo-element. Keep the badge at 36px and prevent pointer interception.

**Step 3: Add motion states**

Add `tier-core-cell-idle` for idle/touch, `tier-core-cell-burst` for desktop hover, and static reduced-motion fallbacks. Keep cycles at or above 3 seconds and avoid flashing.

**Step 4: Run focused tests**

Run: `pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

Expected: PASS.

### Task 3: Verify and integrate

**Files:**
- Test: `frontend/src/config/__tests__/affiliateTierPresentation.spec.ts`
- Test: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`
- Test: `frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts`

**Step 1: Run regression checks**

Run:

```bash
pnpm test:run \
  src/config/__tests__/affiliateTierPresentation.spec.ts \
  src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts \
  src/views/user/__tests__/AffiliateView.tiers.spec.ts
pnpm typecheck
pnpm lint:check
pnpm build
```

Expected: all commands exit 0.

**Step 2: Verify in browser**

Check desktop and 390px views at `/affiliate`. Confirm the Core cell is distinguishable in a static screenshot, the compact badge remains 36px, and the page has no horizontal overflow.

**Step 3: Commit**

```bash
git add frontend/src/components/affiliate/AffiliateTierIdentity.vue \
  frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts \
  docs/plans/2026-07-14-affiliate-core-compact-visibility-implementation.md
git commit -m "design: clarify compact core tier"
```
