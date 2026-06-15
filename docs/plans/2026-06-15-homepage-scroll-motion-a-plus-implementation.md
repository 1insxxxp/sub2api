# Homepage Scroll Motion A+ Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the homepage scroll-loading animation visibly stronger while preserving the existing homepage style and CSS-only approach.

**Architecture:** Add source-level test coverage for A+ motion constants, then update the scoped CSS in `HomeView.vue`. No layout, copy, backend, or dependency changes are required.

**Tech Stack:** Vue 3, scoped CSS, Vitest, Vue Test Utils.

---

### Task 1: Add A+ Motion Regression Test

**Files:**
- Modify: `frontend/src/views/__tests__/HomeView.spec.ts`

**Steps:**

1. Import `readFileSync` from `node:fs`.
2. Add a test that reads `../HomeView.vue` and asserts the A+ motion constants:
   - `--motion-distance: 30px`
   - `--motion-section-distance: 24px`
   - `--motion-scale: 0.985`
   - `--motion-blur: 6px`
   - `calc(90ms + (var(--motion-index) * 68ms))`
3. Run:

```bash
pnpm --dir frontend run test:run -- HomeView
```

Expected: fail because the current CSS still has the gentler A values.

### Task 2: Increase Homepage Motion Strength

**Files:**
- Modify: `frontend/src/views/HomeView.vue`

**Steps:**

1. Update `.home-motion-root` variables:
   - `--motion-distance: 30px`
   - `--motion-duration: 860ms`
   - `--motion-section-distance: 24px`
   - `--motion-scale: 0.985`
   - `--motion-section-scale: 0.99`
   - `--motion-blur: 6px`
   - `--motion-section-blur: 4px`
2. Update `.home-scroll-reveal`:
   - duration `860ms`
   - delay `calc(90ms + (var(--motion-index) * 68ms))`
   - range `entry 4% cover 36%`
3. Update `.home-section-reveal`:
   - duration `820ms`
   - range `entry 6% cover 34%`
   - delays `0ms`, `90ms`, `170ms`
4. Add scale and blur settling to `home-rise-in` and `home-section-rise`.
5. Make `.home-code-panel.home-scroll-reveal` and `.home-cta-panel.home-scroll-reveal` slightly slower than cards.
6. Keep mobile smaller:
   - `--motion-distance: 20px`
   - `--motion-section-distance: 16px`
   - `--motion-blur: 3px`
   - `--motion-section-blur: 2px`

### Task 3: Verify

**Commands:**

```bash
pnpm --dir frontend run test:run -- HomeView
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

Then verify the local page at:

```text
http://127.0.0.1:3000/
```

Expected: homepage renders with no horizontal overflow, and motion hooks remain present.

### Task 4: Commit

**Files:**
- Modify: `frontend/src/views/HomeView.vue`
- Modify: `frontend/src/views/__tests__/HomeView.spec.ts`

**Commands:**

```bash
git add frontend/src/views/HomeView.vue frontend/src/views/__tests__/HomeView.spec.ts
git commit -m "feat: amplify homepage scroll motion"
```
