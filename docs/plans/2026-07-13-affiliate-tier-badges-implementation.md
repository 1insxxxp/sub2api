# Affiliate Tier Badges Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a coherent set of futuristic metal tier badges to the affiliate page with responsive sizing, restrained motion, and accessible localized labels.

**Architecture:** Generate four transparent raster assets and map the existing `AffiliateTier` enum to those build-time asset imports in `AffiliateView.vue`. Keep all tier names and state in the existing API and i18n paths; presentation adds a current-tier badge and compact rule badges without changing backend contracts.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest, Vite asset pipeline, generated WebP/PNG raster assets, in-app browser verification.

---

### Task 1: Generate the badge asset family

**Files:**
- Create: `frontend/src/assets/affiliate-tiers/standard.webp`
- Create: `frontend/src/assets/affiliate-tiers/bronze.webp`
- Create: `frontend/src/assets/affiliate-tiers/silver.webp`
- Create: `frontend/src/assets/affiliate-tiers/gold.webp`

**Step 1: Generate a coherent contact sheet**

Use the image generation skill to create four centered, transparent-background, square metal badges with one shared silhouette and lighting direction. The image must contain no text.

**Step 2: Split or export individual assets**

Export each badge as a square WebP with transparency and enough source resolution to remain crisp at 88px. Keep visual padding consistent across all four files.

**Step 3: Inspect assets**

Use local image inspection to confirm transparency, nonblank pixels, consistent framing, and clear differentiation at thumbnail size.

**Step 4: Commit**

```bash
git add frontend/src/assets/affiliate-tiers
git commit -m "feat: add affiliate tier badge assets"
```

### Task 2: Add failing badge presentation tests

**Files:**
- Modify: `frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts`

**Step 1: Add asset and state assertions**

Mount the view with a Silver detail response and assert:

- one current badge exposes the localized Silver alt label;
- every rule has its tier-specific badge;
- the Silver rule has the current-tier marker;
- badge containers have stable responsive size contracts;
- the current badge contains the reduced-motion-safe animation hook.

**Step 2: Add Gold assertions**

Mount Gold and assert the Gold badge uses the highest-tier highlight without adding a rotating animation contract.

**Step 3: Run tests and verify failure**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/AffiliateView.tiers.spec.ts
```

Expected: FAIL because badge elements and tier asset mapping do not exist.

**Step 4: Commit tests**

```bash
git add frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts
git commit -m "test: define affiliate tier badge presentation"
```

### Task 3: Implement the responsive badge system

**Files:**
- Modify: `frontend/src/views/user/AffiliateView.vue`
- Modify: `frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts`

**Step 1: Import and map assets**

Add a typed `Record<AffiliateTier, string>` mapping from enum values to the four generated assets. Add a small helper that returns the mapped source without changing the API model.

**Step 2: Add the current-tier badge**

Place an 88px desktop / 68px mobile badge beside the current-level text. Use localized alt text and `data-testid="current-tier-badge"`. Add a restrained glow layer marked `aria-hidden="true"`.

**Step 3: Add rule badges**

Add 36-44px badges to each tier rule. Mark the active automatic tier with `data-current="true"`, a stronger border, and a subtle background treatment. Keep the existing two-column rule grid and all rate/requirement text.

**Step 4: Add reduced-motion-safe styles**

Use scoped CSS keyframes for a slow opacity/box-shadow pulse and disable animation inside `@media (prefers-reduced-motion: reduce)`. Do not use rotating rings or particle effects.

**Step 5: Run focused verification**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/AffiliateView.tiers.spec.ts
pnpm typecheck
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/views/user/AffiliateView.vue frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts
git commit -m "feat: show affiliate tier badges"
```

### Task 4: Verify build and real responsive layout

**Files:**
- Modify only badge files if verification reveals defects.

**Step 1: Run production build**

Run:

```bash
cd frontend
pnpm build
```

Expected: PASS with all four assets emitted.

**Step 2: Verify real browser dimensions**

With HMR running at `http://127.0.0.1:3000`, inspect `/affiliate` at 320x800, 390x844, and 1440x900. Assert `document.documentElement.scrollWidth <= clientWidth` and the same for `main`.

**Step 3: Capture screenshots**

Capture mobile and desktop screenshots. Confirm badges are nonblank, current-tier emphasis is visible, no text overlaps, and the two-column tier grid remains readable.

**Step 4: Verify reduced motion**

Confirm the CSS media query disables badge animation when reduced motion is requested.

**Step 5: Final tests**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/AffiliateView.tiers.spec.ts src/i18n/__tests__/localeParity.spec.ts
pnpm typecheck
git diff --check
```

Expected: PASS.

**Step 6: Commit verification fixes if needed**

```bash
git add frontend/src/assets/affiliate-tiers frontend/src/views/user/AffiliateView.vue frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts
git commit -m "fix: complete affiliate badge verification"
```
