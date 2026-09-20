# Batch Test Latency and Mobile Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show per-model end-to-end request duration in batch test results and make the account test modal usable on narrow mobile screens.

**Architecture:** Reuse the existing `BatchModelResult.durationMs` measurement and add only a formatter and localized label in the Vue component. Adjust the modal's local Tailwind layout classes so mobile controls stack and long content wraps without changing the BaseDialog contract or backend API.

**Tech Stack:** Vue 3, TypeScript, Tailwind utility classes, Vitest, Vue Test Utils, vue-i18n.

---

### Task 1: Add failing latency and mobile guardrail tests

**Files:**
- Modify: `frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts`

**Step 1: Write the failing tests**

Add assertions that:
- the mixed success/failure batch result includes the measured `durationMs` values;
- the rendered result shows the localized latency label for both successful and failed models;
- the modal footer uses a full-width mobile stack and the result rows use responsive wrapping classes;
- the copy button is visible on touch-sized layouts instead of relying only on hover.

Use the existing mocked SSE streams and spy on `Date.now()` with a deterministic sequence so the test verifies the real batch timing path.

**Step 2: Run the focused test to verify it fails**

Run: `pnpm exec vitest run src/components/admin/account/__tests__/AccountTestModal.spec.ts`

Expected: FAIL because duration is not rendered and the required mobile classes/labels are absent.

**Step 3: Commit the red tests**

```bash
git add frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts
git commit -m "test: cover batch latency and mobile layout"
```

### Task 2: Render per-model request duration

**Files:**
- Modify: `frontend/src/components/admin/account/AccountTestModal.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/en/admin/accounts.ts`

**Step 1: Add a duration formatter**

Format a completed `durationMs` as integer milliseconds below one second and two-decimal seconds at or above one second. Return an empty string for undefined durations.

**Step 2: Render duration beside each completed result**

Add the localized “request duration” label to each result row when `durationMs` exists. Keep pending, testing and cancelled-without-a-completed-request rows free of fabricated timing.

**Step 3: Run the focused test**

Run: `pnpm exec vitest run src/components/admin/account/__tests__/AccountTestModal.spec.ts`

Expected: PASS for latency assertions and all existing modal tests.

### Task 3: Make the modal responsive on mobile

**Files:**
- Modify: `frontend/src/components/admin/account/AccountTestModal.vue`

**Step 1: Stack footer actions on narrow screens**

Make the footer action wrapper full width with a column layout below the small breakpoint and restore the existing right-aligned row on larger screens. Give each action a full-width touch target on mobile and intrinsic width on desktop.

**Step 2: Prevent batch results and metadata from overflowing**

Use a responsive two-column summary on mobile, stack each result row below the small breakpoint, allow model names and errors to shrink or wrap, and keep status and duration readable.

**Step 3: Improve touch and long-text behavior**

Keep the copy action visible on small screens with a touch-sized target, make the account name and mode summary shrink or wrap, and allow terminal output to break long words while retaining vertical scrolling.

**Step 4: Run component tests and lint**

Run:

```bash
pnpm exec vitest run src/components/admin/account/__tests__/AccountTestModal.spec.ts
pnpm exec eslint src/components/admin/account/AccountTestModal.vue src/components/admin/account/__tests__/AccountTestModal.spec.ts
```

Expected: PASS with no lint errors.

### Task 4: Run full verification and commit

**Files:**
- No additional files.

**Step 1: Run verification**

```bash
pnpm exec vitest run src/i18n/__tests__/localeKeyCompleteness.spec.ts
pnpm run typecheck
pnpm exec vitest run src/components/admin/__tests__/AdminComponentSurfaces.spec.ts
pnpm exec vitest run src/components/account/__tests__/AccountTestModal.spec.ts
```

Expected: all commands exit 0.

**Step 2: Check the diff**

```bash
git diff --check
git status --short
```

Confirm only the intended latency/mobile files are staged; leave pre-existing unrelated changes untouched.

**Step 3: Commit the implementation**

```bash
git add frontend/src/components/admin/account/AccountTestModal.vue frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/i18n/locales/en/admin/accounts.ts
git commit -m "feat: show batch model latency on mobile"
```
