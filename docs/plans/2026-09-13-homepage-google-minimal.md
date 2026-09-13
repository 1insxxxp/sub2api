# Google-like Minimal Homepage Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rework the default homepage into a spacious, centered, Google-like landing experience with restrained blue/cyan motion while preserving all existing account, custom-content, locale, theme, and navigation behavior.

**Architecture:** Keep the existing `HomeView.vue` branching order intact: custom HTML/iframe content first, compact mode second, and the redesigned default page last. Replace only the default page markup and scoped styles with a centered hero, a single animated ambient orb/grid visual, minimal trust copy, and responsive controls. Keep script state and computed values unless the new markup no longer needs them, then remove dead data and update focused tests.

**Tech Stack:** Vue 3, TypeScript, Tailwind utility classes, scoped CSS keyframes, Vitest, pnpm.

---

### Task 1: Define the new default homepage contract in tests

**Files:**
- Modify: `frontend/src/views/__tests__/HomeView.spec.ts`

**Step 1: Update the default-page assertions**

Assert the redesigned page exposes a centered hero marker, a primary CTA, docs CTA when configured, and the animated visual marker while no longer requiring the removed multi-section cards.

**Step 2: Run the focused tests**

Run: `cd frontend && pnpm vitest run src/views/__tests__/HomeView.spec.ts src/views/__tests__/HomeView.compact.spec.ts`
Expected: FAIL only for assertions that reference the not-yet-implemented new selectors.

---

### Task 2: Replace the default homepage composition

**Files:**
- Modify: `frontend/src/views/HomeView.vue` (default branch template and scoped styles)

**Step 1: Preserve existing branches and behavior**

Leave `hasHomeContent`, `isHomeContentUrl`, compact mode, auth routing, public settings, `sanitizeUrl`, language switch, theme switch, docs conditional, model plaza conditional, and `WechatServiceButton` behavior unchanged.

**Step 2: Implement the centered hero**

Use a fixed translucent header with logo and existing controls, followed by one full-height centered section containing the eyebrow, large two-line title, subtitle, primary route CTA, optional docs link, and a compact compatibility line. Add an absolutely positioned soft blue/cyan orb plus a subtle grid/radial background as the only decorative visual. Remove the code preview, static metrics, capability strip, quickstart card, and repeated CTA.

**Step 3: Add restrained motion and accessibility**

Use staggered fade/translate entrance classes, slow orb drift, and a small hover lift on the primary CTA. Include `@media (prefers-reduced-motion: reduce)` to disable transitions and animations. Keep all interactive controls at least 44px high and prevent horizontal overflow on narrow screens.

**Step 4: Remove dead script data**

Delete `navItems` and `valueItems` only if no remaining template branch references them; retain `apiBaseUrl` only if the final markup uses it.

---

### Task 3: Verify and commit

**Files:**
- Modify: `frontend/src/views/__tests__/HomeView.spec.ts`
- Modify: `frontend/src/views/HomeView.vue`

**Step 1: Run focused tests**

Run: `cd frontend && pnpm vitest run src/views/__tests__/HomeView.spec.ts src/views/__tests__/HomeView.compact.spec.ts src/i18n/__tests__/homeLocales.spec.ts`
Expected: PASS.

**Step 2: Run static checks and production build**

Run: `cd frontend && pnpm typecheck && pnpm lint:check && pnpm build`
Expected: all commands exit 0.

**Step 3: Inspect the rendered local page**

Open `http://localhost:3000/home` and check desktop and narrow viewport behavior: centered hero, no horizontal scroll, visible CTA/docs controls, and no animation when reduced motion is enabled.

**Step 4: Commit only the feature files**

```bash
git add -f docs/plans/2026-09-13-homepage-google-minimal.md frontend/src/views/HomeView.vue frontend/src/views/__tests__/HomeView.spec.ts
git commit -m "feat: redesign homepage with minimal animated hero"
```

Do not stage or modify the pre-existing user change in `AGENTS.md`.
