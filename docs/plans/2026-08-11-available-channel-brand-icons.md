# Unified Available Channel Brand Icons Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make every available-channel model-card icon use the same platform logo and normal-state theme colors as API-key group badges.

**Architecture:** Extract the platform badge theme resolver from `GroupBadge` into a small shared utility, then make both `GroupBadge` and `AvailableChannelBrandIcon` consume it. Replace the brand icon's private SVG set with the common `PlatformIcon`, while retaining the existing 44px card icon container and platform alias normalization.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest, Vue Test Utils.

---

### Task 1: Extract the shared platform theme

**Files:**
- Create: `frontend/src/components/common/platformVisual.ts`
- Modify: `frontend/src/components/common/GroupBadge.vue`
- Modify: `frontend/src/components/common/__tests__/GroupBadge.spec.ts`

**Step 1: Write the failing test**

Add a table-driven test which mounts a normal `GroupBadge` for `anthropic`, `openai`, `gemini`, `antigravity`, `grok`, `composite`, and an unknown platform. Assert that every rendered badge contains the class string returned by `platformBadgeThemeClass(platform, false)`.

**Step 2: Run the test to verify it fails**

Run:

```bash
cd frontend
npx vitest run src/components/common/__tests__/GroupBadge.spec.ts
```

Expected: FAIL because `platformVisual.ts` and `platformBadgeThemeClass` do not exist.

**Step 3: Implement the shared theme resolver**

Create:

```ts
export function platformBadgeThemeClass(
  platform?: GroupPlatform,
  isSubscription = false,
): string {
  // Return the exact existing GroupBadge bg/text classes for each platform.
}
```

Replace the inline `badgeClass` platform branch in `GroupBadge.vue` with a computed call to the shared resolver. Preserve every existing subscription and fallback class exactly.

**Step 4: Run the test to verify it passes**

Run the focused GroupBadge test and confirm all cases pass.

### Task 2: Rebuild the model-card icon from shared primitives

**Files:**
- Modify: `frontend/src/components/channels/availableChannelBrand.ts`
- Modify: `frontend/src/components/channels/AvailableChannelBrandIcon.vue`
- Modify: `frontend/src/components/common/PlatformIcon.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelBrandIcon.spec.ts`

**Step 1: Write the failing tests**

Extend the branding tests to assert:

- Anthropic, OpenAI, Gemini, Antigravity, Grok, and Composite aliases resolve to the corresponding common platform key.
- Unknown platforms resolve to the generic fallback.
- `AvailableChannelBrandIcon` renders a `PlatformIcon` with `size="xl"` rather than private inline SVG artwork.
- The large icon contains the same normal-state theme classes returned by `platformBadgeThemeClass`.
- The outer container remains `size-11` with an accessible label.

**Step 2: Run the test to verify it fails**

Run:

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelBrandIcon.spec.ts
```

Expected: FAIL because the component still owns separate SVGs and theme tokens.

**Step 3: Implement the minimal component changes**

- Extend `PlatformIcon` with an `xl` size mapped to `w-6 h-6`.
- Normalize all supported aliases in `availableChannelBrand.ts` and expose the common platform key used by `PlatformIcon`.
- Render one `PlatformIcon` inside `AvailableChannelBrandIcon`.
- Apply `platformBadgeThemeClass(normalizedPlatform, false)` to the existing 44px rounded container, plus only neutral ring/shadow structure classes.
- Pass `undefined` for unknown platforms so `PlatformIcon` renders its existing generic fallback.

**Step 4: Run the focused tests to verify they pass**

Run the brand icon and GroupBadge suites together and confirm all cases pass.

### Task 3: Verify integration and commit

**Files:**
- Verify all changed frontend files.

**Step 1: Run channel and common component regression tests**

```bash
cd frontend
npx vitest run src/components/channels/__tests__ src/components/common/__tests__/GroupBadge.spec.ts
```

Expected: all tests pass.

**Step 2: Run static and build verification**

```bash
npm run typecheck
npx eslint src/components/common/platformVisual.ts src/components/common/GroupBadge.vue src/components/common/PlatformIcon.vue src/components/common/__tests__/GroupBadge.spec.ts src/components/channels/availableChannelBrand.ts src/components/channels/AvailableChannelBrandIcon.vue src/components/channels/__tests__/AvailableChannelBrandIcon.spec.ts
npm run build
git diff --check
```

Expected: all commands exit 0.

**Step 3: Commit**

```bash
git add frontend/src/components/common/platformVisual.ts frontend/src/components/common/GroupBadge.vue frontend/src/components/common/PlatformIcon.vue frontend/src/components/common/__tests__/GroupBadge.spec.ts frontend/src/components/channels/availableChannelBrand.ts frontend/src/components/channels/AvailableChannelBrandIcon.vue frontend/src/components/channels/__tests__/AvailableChannelBrandIcon.spec.ts
git commit -m "fix: unify available channel brand icons"
```
