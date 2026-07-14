# Affiliate Core Reactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give Core an exclusive three-hex reactor, node ignition, electric arc, convergence, and shockwave treatment that clearly exceeds the other affiliate tiers.

**Architecture:** Extend the shared badge-effect wrapper with two decorative layers that remain hidden for non-Core themes. Drive the full current-Core sequence and compact Core idle/hover states through scoped CSS while preserving touch, clipping, and reduced-motion contracts.

**Tech Stack:** Vue 3, TypeScript, scoped CSS, Vue Test Utils, Vitest, Vite

---

### Task 1: Add Core Reactor Structure

**Files:**
- Modify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`
- Test: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Write the failing test**

Require every effect wrapper to expose decorative `.tier-badge-effect__reactor` and `.tier-badge-effect__arc` layers. Assert they are `aria-hidden` and the wrapper still reports the correct tier theme and active state.

**Step 2: Run the focused test and verify RED**

```bash
cd frontend
pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
```

Expected: FAIL because the Core reactor layers do not exist.

**Step 3: Add minimal markup and hidden defaults**

Add the two decorative spans to large and compact wrappers. Hide both by default and reveal them only under `[data-effect-theme='core']` selectors. Preserve fixed wrapper dimensions and decorative semantics.

**Step 4: Run the focused test and verify GREEN**

Expected: all component tests pass.

### Task 2: Implement Full And Compact Core States

**Files:**
- Modify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`
- Test: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Write the failing CSS contract tests**

Assert source contracts for:

- `tier-core-reactor-spin`
- `tier-core-node-ignite`
- `tier-core-electric-arc`
- `tier-core-compact-idle`
- Current Core full-state selectors
- Non-current compact Core idle selectors
- Touch-only compact simplification
- Reduced-motion rules covering reactor and arc layers

**Step 2: Run the focused test and verify RED**

Expected: FAIL because Core-exclusive animations are absent.

**Step 3: Implement Core-exclusive CSS**

Use the reactor element and its pseudo-elements as three nested hexagonal structures with counter-rotation. Use layered radial gradients for six perimeter nodes and a conic mask for the ignition chase. Use the arc layer for a brief blue-white charge arc. Current Core runs the full sequence; compact non-current Core runs only a slow idle chase, and desktop hover upgrades it to a stronger burst.

Touch input keeps only the compact idle sequence. Reduced motion stops all reactor motion and displays static nested hexagons. Keep compact overflow clipped with `hidden` and `clip` fallback order.

**Step 4: Run focused and affiliate regression tests**

```bash
cd frontend
pnpm test:run src/config/__tests__/affiliateTierPresentation.spec.ts src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts src/views/user/__tests__/AffiliateView.tiers.spec.ts
```

Expected: all tests pass, excluding existing todo cases.

### Task 3: Verify And Integrate

**Files:**
- Verify: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`
- Verify: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Run static checks**

```bash
cd frontend
pnpm typecheck
pnpm lint:check
pnpm build
```

Expected: all commands exit successfully.

**Step 2: Verify in a real browser**

At desktop and 390px widths, inspect the compact Core preview and confirm its exclusive idle animation, 36px clipping, loaded asset, no overlap, and no horizontal overflow. Confirm touch media and reduced-motion CSS are present and no console errors occur.

**Step 3: Commit**

```bash
git add frontend/src/components/affiliate/AffiliateTierIdentity.vue frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
git commit -m "design: elevate affiliate core effects"
```
