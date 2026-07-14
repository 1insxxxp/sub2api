# Affiliate Tier Light Effects Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add tier-specific CSS light animation to the current affiliate badge and compact tier rule badges without changing layout or business behavior.

**Architecture:** Extend `AffiliateTierIdentity.vue` with a reusable decorative badge-effect wrapper that receives the normalized tier theme and an active state. Keep all animation in scoped CSS, preserve the existing WebP assets, and disable repeated motion for reduced-motion users.

**Tech Stack:** Vue 3, TypeScript, scoped CSS, Vue Test Utils, Vitest, Vite

---

### Task 1: Define Badge Effect Structure

**Files:**
- Modify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`
- Test: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Write the failing component test**

Add assertions that the large badge has one `data-testid="tier-badge-effect"` wrapper with the normalized `data-effect-theme`, that every compact rule has its own effect wrapper, and that only the current compact rule carries `data-effect-active="true"`.

**Step 2: Run the focused test to verify it fails**

Run:

```bash
cd frontend
pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
```

Expected: FAIL because the effect wrappers and attributes do not exist.

**Step 3: Add the minimal effect markup**

Wrap the large and compact badge images with fixed-size `.tier-badge-effect` containers. Add decorative spans for glow, beam, and orbit layers with `aria-hidden="true"`. Expose normalized theme and active state as data attributes. Keep the image dimensions unchanged.

**Step 4: Run the focused test to verify it passes**

Run the same focused Vitest command. Expected: PASS.

### Task 2: Add Tier-Specific Motion

**Files:**
- Modify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`
- Test: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Add static CSS contract assertions**

Read the component source in the test and assert that it contains a `prefers-reduced-motion: reduce` override and distinct theme selectors for origin, pulse, orbit, and core effect layers.

**Step 2: Run the focused test to verify it fails**

Expected: FAIL because the new effect CSS is absent.

**Step 3: Implement the minimal CSS effects**

- Origin: breathing glow and short arc glint.
- Pulse: horizontal beam expansion and center pulse.
- Orbit: tilted rotating light track.
- Core: six-point glow sequence and center convergence.
- Large badge: continuously active.
- Compact current badge: continuously active.
- Compact non-current badge: active on `hover` or `focus-within` only.
- Reduced motion: remove animations and transforms while retaining a static glow.

Animate only opacity, transform, and filter. Keep effect layers clipped to fixed badge footprints and pointer-events disabled.

**Step 4: Run focused and affiliate regression tests**

```bash
cd frontend
pnpm test:run src/config/__tests__/affiliateTierPresentation.spec.ts src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts src/views/user/__tests__/AffiliateView.tiers.spec.ts
```

Expected: all tests pass.

### Task 3: Verify Responsive And Reduced-Motion Behavior

**Files:**
- Verify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`

**Step 1: Run static verification**

```bash
cd frontend
pnpm typecheck
pnpm lint:check
pnpm build
```

Expected: all commands exit successfully.

**Step 2: Verify in the browser**

Open `/affiliate` at 1440px and 390px. Confirm all five rendered badge images load, the four compact silhouettes remain distinct, and `scrollWidth <= clientWidth`. Inspect animation names for each tier and emulate reduced motion to confirm repeated animation is disabled.

**Step 3: Commit only the feature files**

```bash
git add frontend/src/components/affiliate/AffiliateTierIdentity.vue frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
git commit -m "feat: animate affiliate tier badges"
```
