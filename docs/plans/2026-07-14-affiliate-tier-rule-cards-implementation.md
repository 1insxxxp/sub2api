# Affiliate Tier Rule Cards Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Render every affiliate tier as a complete framed card with unlocked, current, and locked states.

**Architecture:** Keep the existing affiliate API and tier presentation mapping. Derive each rule card's state from its position relative to the normalized automatic level, then render a responsive independent-card grid with tier-specific decorative frames contained inside each card.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, scoped CSS, Vue Test Utils, Vitest.

---

### Task 1: Lock The Tier Card State Contract

**Files:**
- Modify: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Write the failing tests**

Add tests asserting that:

- four `[data-testid="tier-rule"]` elements render;
- each rule has the complete-card class and a `data-tier-state` attribute;
- an Orbit user sees Origin and Pulse as `unlocked`, Orbit as `current`, and Core as `locked`;
- the cards expose localized status text;
- locked Core shows the remaining qualified invitee count.

**Step 2: Run the focused test and verify it fails**

Run:

```bash
cd frontend && pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
```

Expected: FAIL because the current shared-border rules do not expose card states or status labels.

**Step 3: Commit the test contract**

```bash
git add frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
git commit -m "test: define affiliate tier rule card states"
```

### Task 2: Implement Independent Tier Cards

**Files:**
- Modify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`

**Step 1: Add state derivation**

Create a `TierRuleState` type and derive `unlocked`, `current`, or `locked` by comparing each tier index with the current tier index. Add helpers for localized status text and the remaining threshold for locked tiers.

**Step 2: Replace the shared-border grid**

Render a one-column mobile / two-column desktop grid with spacing. Give every rule its own complete border, stable content layout, state label, and decorative frame element. Preserve badge images, rates, thresholds, and existing test IDs.

**Step 3: Add tier and state styling**

Use contained pseudo-elements for Origin circular traces, Pulse waveform marks, Orbit arcs, and Core hex/reactor framing. Keep all effects inside the card, emphasize current, retain normal contrast for unlocked, and reduce intensity for locked.

**Step 4: Add localized status labels**

Add English and Chinese keys for unlocked, current, locked, and remaining invitees.

**Step 5: Run the focused test**

Run:

```bash
cd frontend && pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
```

Expected: PASS.

### Task 3: Regression And Visual Verification

**Files:**
- Test: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`
- Test: `frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts`

**Step 1: Run affiliate regression tests**

```bash
cd frontend && pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts src/views/user/__tests__/AffiliateView.tiers.spec.ts
```

Expected: PASS.

**Step 2: Run static verification**

```bash
cd frontend && pnpm typecheck
cd frontend && pnpm lint:check
cd frontend && pnpm build
```

Expected: all commands exit 0.

**Step 3: Verify in the browser**

Open the affiliate page with the local Vite server at desktop and mobile widths. Confirm:

- all four cards have complete frames;
- no effect overlaps adjacent cards;
- states and remaining counts are readable;
- the mobile layout is one column without horizontal overflow;
- browser console contains no runtime errors.

**Step 4: Commit the implementation**

```bash
git add frontend/src/components/affiliate/AffiliateTierIdentity.vue \
  frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts \
  frontend/src/i18n/locales/en/dashboard.ts \
  frontend/src/i18n/locales/zh/dashboard.ts \
  docs/plans/2026-07-14-affiliate-tier-rule-cards-implementation.md
git commit -m "feat: redesign affiliate tier rule cards"
```
