# Affiliate Tier High-Energy Effects Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make affiliate tier badge animation immediately visible through stronger layered light, shorter cycles, and tier-specific high-energy motion.

**Architecture:** Extend the existing CSS-only effect wrapper with one additional outer aura layer and stronger keyframes. Keep the current activation, compact clipping, touch suppression, and reduced-motion contracts unchanged.

**Tech Stack:** Vue 3, TypeScript, scoped CSS, Vue Test Utils, Vitest, Vite

---

### Task 1: Lock The High-Energy Contract

**Files:**
- Modify: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`
- Modify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`

**Step 1: Write the failing test**

Add assertions that each effect wrapper renders an `aria-hidden` outer aura layer. Add source-contract assertions for the four high-energy animation names, cycles no longer than 3.6 seconds, and peak opacity values of at least 0.86.

**Step 2: Run the focused test and verify RED**

```bash
cd frontend
pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
```

Expected: FAIL because the outer aura and high-energy timing contract do not exist.

**Step 3: Add the outer aura markup**

Add `.tier-badge-effect__aura` to large and compact wrappers. Keep it decorative, pointer-inert, and inside the existing fixed wrapper dimensions.

**Step 4: Implement stronger tier motion**

- Increase static glow presence and peak opacity.
- Add an expanding Origin energy ring.
- Add Pulse afterglow and faster wing expansion.
- Add a counter-rotating Orbit aura with a brighter trail.
- Add a Core hexagonal shockwave after convergence.
- Keep large overflow visible and compact overflow clipped.
- Keep touch suppression and reduced-motion overrides covering every layer.

**Step 5: Run the focused test and verify GREEN**

Expected: all component tests pass.

### Task 2: Verify Intensity And Regression Safety

**Files:**
- Verify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`
- Verify: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Run affiliate regressions**

```bash
cd frontend
pnpm test:run src/config/__tests__/affiliateTierPresentation.spec.ts src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts src/views/user/__tests__/AffiliateView.tiers.spec.ts
```

Expected: all tests pass, excluding existing todo cases.

**Step 2: Run static checks**

```bash
cd frontend
pnpm typecheck
pnpm lint:check
pnpm build
```

Expected: all commands exit successfully.

**Step 3: Verify in a real browser**

At desktop and 390px widths, confirm all five badge images load, no horizontal overflow occurs, compact effects remain within 36px, and only current compact motion runs on touch. Sample large-badge computed opacity at multiple points and confirm a substantial change. Inspect reduced-motion CSS to ensure all effect layers stop animating.

**Step 4: Commit the implementation**

```bash
git add frontend/src/components/affiliate/AffiliateTierIdentity.vue frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
git commit -m "design: amplify affiliate badge effects"
```
