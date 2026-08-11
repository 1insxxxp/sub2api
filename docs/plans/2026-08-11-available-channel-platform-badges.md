# Available Channel Platform Badges Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make every small platform label on the available-channels page use the same icon and platform color styling as API key group badges.

**Architecture:** Add a presentation-only `AvailableChannelPlatformBadge` wrapper that delegates all visuals to the shared `GroupBadge` with rate display disabled. Replace page-local gray platform chips and joined platform text with this wrapper while leaving filters, model brand icons, selection, and pricing data unchanged.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vue Test Utils, Vitest.

---

### Task 1: Define the unified platform badge

**Files:**
- Create: `frontend/src/components/channels/AvailableChannelPlatformBadge.vue`
- Create: `frontend/src/components/channels/__tests__/AvailableChannelPlatformBadge.spec.ts`

**Step 1: Write the failing component test**

Mount the wished-for component with `platform="anthropic"`, stub `GroupBadge`, and assert that it receives:

```ts
{
  name: 'anthropic',
  platform: 'anthropic',
  showRate: false,
}
```

Also assert that the wrapper exposes `data-testid="available-channel-platform-badge"`.

**Step 2: Run the test to verify RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelPlatformBadge.spec.ts
```

Expected: FAIL because the component does not exist.

**Step 3: Implement the thin wrapper**

Create a component that accepts `platform: string` and renders:

```vue
<GroupBadge
  data-testid="available-channel-platform-badge"
  :name="platform"
  :platform="platform as GroupPlatform"
  :show-rate="false"
/>
```

Do not copy any platform classes or icon SVGs.

**Step 4: Run the test to verify GREEN**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelPlatformBadge.spec.ts
```

Expected: PASS.

### Task 2: Replace desktop and model-card platform chips

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Modify: `frontend/src/components/channels/AvailableChannelsToolbar.vue`
- Modify: `frontend/src/components/channels/AvailableChannelModelList.vue`
- Modify: `frontend/src/components/channels/AvailableChannelOfferingCard.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelModelListView.spec.ts`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts`

**Step 1: Add failing expectations**

Update the four suites to stub `AvailableChannelPlatformBadge` with a visible `data-platform-badge` marker. Assert:

- Catalog navigation and fallback header render the badge for each platform.
- Toolbar selected-channel context renders the badge.
- Every model-card platform entry renders the badge.
- Expanded offering metadata renders the badge.
- Raw source no longer contains the old gray platform-chip class blocks.

**Step 2: Run the four tests to verify RED**

```bash
cd frontend
npx vitest run \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelListView.spec.ts \
  src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts
```

Expected: FAIL because those locations still render local spans and `PlatformIcon` directly.

**Step 3: Replace the platform displays**

Import `AvailableChannelPlatformBadge` in all four components and replace each small platform label with:

```vue
<AvailableChannelPlatformBadge :platform="platformName" />
```

Remove unused `PlatformIcon`, `GroupPlatform`, and conversion helpers. Keep `AvailableChannelBrandIcon` on model cards unchanged.

**Step 4: Run the four tests to verify GREEN**

Run the command from Step 2. Expected: PASS.

### Task 3: Replace mobile picker platform text

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelPicker.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelPicker.spec.ts`

**Step 1: Add failing mobile expectations**

Stub `AvailableChannelPlatformBadge`, then assert that both the selected-channel trigger and each picker option render one badge per platform. Assert raw source no longer uses `platforms.join(' · ')`.

**Step 2: Run the picker test to verify RED**

```bash
cd frontend
npx vitest run src/components/channels/__tests__/AvailableChannelPicker.spec.ts
```

Expected: FAIL because mobile platform metadata is plain joined text.

**Step 3: Render wrapping platform badges**

Replace both joined-text locations with wrapping badge rows while retaining the group/model counts and existing trigger/option semantics.

**Step 4: Run the picker test to verify GREEN**

Run the command from Step 2. Expected: PASS.

### Task 4: Verify and commit

**Files:**
- All files listed above.

**Step 1: Run the complete channels suite**

```bash
cd frontend
npx vitest run src/components/channels/__tests__
```

Expected: all tests pass.

**Step 2: Run static checks**

```bash
cd frontend
npx vue-tsc --noEmit --pretty false
npx eslint \
  src/components/channels/AvailableChannelPlatformBadge.vue \
  src/components/channels/AvailableChannelCatalog.vue \
  src/components/channels/AvailableChannelsToolbar.vue \
  src/components/channels/AvailableChannelModelList.vue \
  src/components/channels/AvailableChannelOfferingCard.vue \
  src/components/channels/AvailableChannelPicker.vue \
  src/components/channels/__tests__/AvailableChannelPlatformBadge.spec.ts \
  src/components/channels/__tests__/AvailableChannelCatalog.spec.ts \
  src/components/channels/__tests__/AvailableChannelsToolbar.spec.ts \
  src/components/channels/__tests__/AvailableChannelModelListView.spec.ts \
  src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts \
  src/components/channels/__tests__/AvailableChannelPicker.spec.ts
```

Expected: exit 0.

**Step 3: Commit**

```bash
git add frontend/src/components/channels
git commit -m "fix: unify available channel platform badges"
```
