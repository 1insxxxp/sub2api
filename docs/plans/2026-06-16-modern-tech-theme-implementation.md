# Modern Tech Theme Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the teal brand theme with a blue-violet technology palette that works in both light and dark modes.

**Architecture:** Centralize the main change in `frontend/tailwind.config.js` so existing `primary-*` usage updates across the app. Patch only hardcoded teal values and homepage brand accents that do not inherit from `primary`, while preserving semantic success colors.

**Tech Stack:** Vue 3, Tailwind CSS, Vitest, Vite.

---

### Task 1: Theme Token Tests

**Files:**
- Modify: `frontend/src/views/__tests__/HomeView.spec.ts`

**Step 1: Write the failing test**

Add tests that read `tailwind.config.js` and `HomeView.vue` to assert:
- `primary.500` is `#3b82f6`
- `primary.600` is `#2563eb`
- `primary.700` is `#1d4ed8`
- Theme gradients include blue/violet/cyan instead of teal-heavy values
- Homepage light and dark hero washes include blue-toned values
- Homepage brand status accents use `primary`/`cyan` where appropriate

**Step 2: Run test to verify it fails**

Run: `pnpm --dir frontend run test:run -- HomeView`

Expected: FAIL because the current config still uses teal primary values.

### Task 2: Theme Token Implementation

**Files:**
- Modify: `frontend/tailwind.config.js`
- Modify: `frontend/src/styles/onboarding.css`
- Modify: `frontend/src/views/KeyUsageView.vue`
- Modify: `frontend/src/components/charts/ModelDistributionChart.vue`
- Modify: `frontend/src/components/charts/GroupDistributionChart.vue`
- Modify: `frontend/src/components/charts/EndpointDistributionChart.vue`

**Step 1: Update primary palette**

Change `primary` from teal to blue, update glow shadows, primary gradients, mesh gradients, and keyframes to use blue/violet/cyan.

**Step 2: Update hardcoded teal brand colors**

Replace hardcoded `#14b8a6`, `#0d9488`, and related teal glow values used for brand primary UI with the new blue values. Leave semantic success green/emerald values intact.

**Step 3: Run focused tests**

Run: `pnpm --dir frontend run test:run -- HomeView`

Expected: PASS.

### Task 3: Homepage Light/Dark Adaptation

**Files:**
- Modify: `frontend/src/views/HomeView.vue`

**Step 1: Refresh homepage brand accents**

Update the hero wash, eyebrow, routing panel accents, code values, integration accent text, and CTA panel gradient to use the new blue-violet-cyan system.

**Step 2: Preserve semantic status**

Keep actual health/success dots green where they communicate operational status, but use `primary`/`cyan` for brand and motion accents.

**Step 3: Verify in browser**

Use local dev server screenshots or DOM inspection for:
- `http://127.0.0.1:3000/` light mode
- `http://127.0.0.1:3000/` dark mode after toggling

Expected: no blurry CTA, no text overlap, no overly low-contrast blue on dark backgrounds.

### Task 4: Final Verification

**Files:**
- No new code files.

**Step 1: Run all required checks**

Run:
- `pnpm --dir frontend run test:run -- HomeView`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run build`

Expected: all commands pass. Existing Vite chunk/import warnings are acceptable only if they match prior known warnings.

**Step 2: Commit**

Commit with: `feat: refresh theme palette`
