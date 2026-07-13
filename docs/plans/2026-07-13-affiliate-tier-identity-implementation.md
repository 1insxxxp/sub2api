# Affiliate Tier Identity Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the game-style affiliate tier presentation with a blue/cyan brand-geometric identity and make the affiliate page's visual emphasis and stage objective respond to Origin, Pulse, Orbit, and Core tiers.

**Architecture:** Preserve `standard`, `bronze`, `silver`, and `gold` as backend/API values, and add one frontend presentation map that owns display keys, theme names, badge assets, objectives, and featured states. Extract the current tier summary into a focused Vue component; keep affiliate loading, transfer, invite-link, and rebate calculations in `AffiliateView.vue`.

**Tech Stack:** Vue 3 Composition API, TypeScript, vue-i18n, Tailwind CSS, scoped CSS, Vitest, Vue Test Utils, generated WebP assets, Vite.

---

### Task 1: Lock The New Tier Vocabulary In Tests

**Files:**
- Modify: `frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/AdminAffiliateRecordsTable.spec.ts`

**Step 1: Update the user-page expectations to the approved names**

Change the mocked translations and assertions to use:

```ts
'affiliate.tiers.levels.standard': 'Origin',
'affiliate.tiers.levels.bronze': 'Pulse',
'affiliate.tiers.levels.silver': 'Orbit',
'affiliate.tiers.levels.gold': 'Core',
```

Assert that Orbit progress points to Core and that the Core fixture shows the highest-level message.

**Step 2: Add an administrator display-name assertion**

Make the admin affiliate record test's translation stub resolve `admin.affiliates.tiers.silver` to `Orbit`, then assert that the rendered automatic tier uses `Orbit` rather than the internal value.

**Step 3: Run focused tests and verify they fail**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/AffiliateView.tiers.spec.ts src/views/admin/__tests__/AdminAffiliateRecordsTable.spec.ts
```

Expected: FAIL because production translations still return the old names and the new presentation contract does not exist.

**Step 4: Commit the test contract**

```bash
git add frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts frontend/src/views/admin/__tests__/AdminAffiliateRecordsTable.spec.ts
git commit -m "test: define affiliate tier identity names"
```

### Task 2: Add The Central Tier Presentation Map

**Files:**
- Create: `frontend/src/config/affiliateTierPresentation.ts`
- Create: `frontend/src/config/__tests__/affiliateTierPresentation.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/overview.ts`
- Modify: `frontend/src/i18n/locales/en/admin/overview.ts`

**Step 1: Write failing map and fallback tests**

Cover all stable API values and an invalid runtime value:

```ts
expect(getAffiliateTierPresentation('standard').theme).toBe('origin')
expect(getAffiliateTierPresentation('bronze').theme).toBe('pulse')
expect(getAffiliateTierPresentation('silver').theme).toBe('orbit')
expect(getAffiliateTierPresentation('gold').theme).toBe('core')
expect(getAffiliateTierPresentation('future-tier').theme).toBe('origin')
```

Also assert each entry has a `labelKey`, `objectiveKey`, and `featuredMetric`.

**Step 2: Run the new test and verify it fails**

Run:

```bash
cd frontend
pnpm test:run src/config/__tests__/affiliateTierPresentation.spec.ts
```

Expected: FAIL because the module does not exist.

**Step 3: Implement the presentation map**

Define a typed map with these stable concepts:

```ts
export type AffiliateTierTheme = 'origin' | 'pulse' | 'orbit' | 'core'
export type AffiliateFeaturedMetric = 'invited' | 'qualified' | 'history' | 'rate'

export interface AffiliateTierPresentation {
  theme: AffiliateTierTheme
  labelKey: string
  objectiveKey: string
  featuredMetric: AffiliateFeaturedMetric
}
```

Return Origin for unknown, null, or undefined runtime values. Do not rename `AffiliateTier` or change API types.

**Step 4: Update user and administrator translations**

Use these labels:

```ts
standard: '原点级' // Origin
bronze: '脉冲级'   // Pulse
silver: '星环级'   // Orbit
gold: '极核级'     // Core
```

Add localized identity and objective keys for first qualification, progress to Orbit, progress to Core, and highest-tier achievement. Keep all interpolation placeholders aligned between Chinese and English.

**Step 5: Run focused tests**

Run:

```bash
cd frontend
pnpm test:run src/config/__tests__/affiliateTierPresentation.spec.ts src/views/user/__tests__/AffiliateView.tiers.spec.ts src/views/admin/__tests__/AdminAffiliateRecordsTable.spec.ts
```

Expected: presentation-map tests PASS; page tests may remain red until the component integration task.

**Step 6: Commit**

```bash
git add frontend/src/config/affiliateTierPresentation.ts frontend/src/config/__tests__/affiliateTierPresentation.spec.ts frontend/src/i18n/locales/zh/dashboard.ts frontend/src/i18n/locales/en/dashboard.ts frontend/src/i18n/locales/zh/admin/overview.ts frontend/src/i18n/locales/en/admin/overview.ts
git commit -m "feat: add affiliate tier identity mapping"
```

### Task 3: Build The Tier Identity Component With TDD

**Files:**
- Create: `frontend/src/components/affiliate/AffiliateTierIdentity.vue`
- Create: `frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts`

**Step 1: Write component tests for all four themes**

Mount the component with a `UserAffiliateDetail` fixture for each tier. Assert:

- `data-tier-theme` is `origin`, `pulse`, `orbit`, or `core`.
- The localized tier label and effective rate are visible.
- Origin shows the first-qualified-invite objective.
- Pulse and Orbit show the correct next-tier objective and remaining count.
- Core shows the highest-tier achievement instead of a remaining count.
- A zero-invite Origin fixture renders a safe `0%` qualified ratio rather than `NaN` or `Infinity`.
- The custom-rate marker remains visible when `has_custom_rebate_rate` is true.

**Step 2: Run the test and verify it fails**

Run:

```bash
cd frontend
pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
```

Expected: FAIL because the component does not exist.

**Step 3: Implement the component structure**

Accept these props:

```ts
interface Props {
  detail: UserAffiliateDetail
  nextTier: AffiliateTierDefinition | null
  progress: number
  formattedRate: string
}
```

Render one un-nested identity section containing:

- badge stage and localized label;
- effective/custom rate;
- qualified count and progress or Core completion state;
- tier-aware current-stage objective;
- compact four-tier rules.

Use the central presentation map for theme and objective selection. Keep decorative grid, rings, nodes, and scan lines as CSS layers with `aria-hidden="true"`.

**Step 4: Add restrained tier-aware styles**

Use scoped CSS variables based on `data-tier-theme`. Keep all themes primarily within the project's blue/cyan palette. Increase structural density from Origin to Core without changing the component's outer dimensions. Disable all animation in:

```css
@media (prefers-reduced-motion: reduce) {
  /* animation: none for badge and decorative identity layers */
}
```

At narrow widths, hide secondary decorative nodes and reduce pattern opacity.

**Step 5: Run the component tests**

Run:

```bash
cd frontend
pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/components/affiliate/AffiliateTierIdentity.vue frontend/src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
git commit -m "feat: add tier-aware affiliate identity panel"
```

### Task 4: Integrate The Identity Panel And Tier-Aware Emphasis

**Files:**
- Modify: `frontend/src/views/user/AffiliateView.vue`
- Modify: `frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts`

**Step 1: Add integration assertions before replacing markup**

Assert that the affiliate view:

- renders exactly one tier identity component;
- applies the expected `data-tier-theme` for each fixture;
- marks the configured statistic with `data-featured="true"`;
- preserves all four rules, invite code/link, transfer action, desktop table, and mobile invitee cards.

**Step 2: Run the view test and verify the new assertions fail**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/AffiliateView.tiers.spec.ts
```

Expected: FAIL because the existing inline summary has no new identity contract.

**Step 3: Replace the inline tier summary**

Import `AffiliateTierIdentity` and the presentation helper. Move only identity/rule markup and its badge animation CSS into the component. Keep `nextTier`, `tierProgress`, loading, clipboard, transfer, and formatting behavior in the page.

Add stable `data-stat` hooks to the four statistic cards and apply the featured state from the current presentation. Do not reorder cards or alter transfer/invite behavior.

**Step 4: Run user-page and component tests**

Run:

```bash
cd frontend
pnpm test:run src/views/user/__tests__/AffiliateView.tiers.spec.ts src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/user/AffiliateView.vue frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts
git commit -m "feat: adapt affiliate page to promotion tier"
```

### Task 5: Generate And Replace The Four Badge Assets

**Files:**
- Modify: `frontend/src/assets/affiliate-tiers/standard.webp`
- Modify: `frontend/src/assets/affiliate-tiers/bronze.webp`
- Modify: `frontend/src/assets/affiliate-tiers/silver.webp`
- Modify: `frontend/src/assets/affiliate-tiers/gold.webp`

**Step 1: Generate a coherent four-badge contact sheet or four isolated assets**

Use the configured image-generation gateway and the imagegen skill. Prompt for transparent-looking, front-facing, centered geometric product emblems with a uniform chroma-key background. Require:

- Passion API-style segmented geometry;
- blue/cyan dominant palette;
- white structural lines and circular nodes;
- four clearly different silhouettes: open seed arc, horizontal pulse wings, tilted orbital paths, and six-direction core matrix;
- no words, letters, numbers, crowns, shields, armor, realistic metal, or warm tier palettes;
- clear silhouette at 40px and 88px; reject a set if the four assets read as variations of the same concentric target.

**Step 2: Remove chroma key and normalize assets**

Use the imagegen chroma-key removal helper, crop each badge to matching square bounds, and convert to WebP. Preserve alpha and keep dimensions consistent.

**Step 3: Inspect before replacing tracked files**

Open each image and a four-badge contact sheet. Reject assets with magenta fringe, unreadable small details, inconsistent lighting, or a game-rank silhouette. Compare them against `frontend/public/logo-passion-api-mark.png` and a live affiliate-page screenshot.

**Step 4: Replace assets and verify technical properties**

Confirm all files decode, have alpha, use identical dimensions, and remain visually distinct at compact size.

**Step 5: Run focused tests and build**

Run:

```bash
cd frontend
pnpm test:run src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts src/views/user/__tests__/AffiliateView.tiers.spec.ts
pnpm build
```

Expected: PASS and Vite includes all four WebP assets.

**Step 6: Commit**

```bash
git add frontend/src/assets/affiliate-tiers/standard.webp frontend/src/assets/affiliate-tiers/bronze.webp frontend/src/assets/affiliate-tiers/silver.webp frontend/src/assets/affiliate-tiers/gold.webp
git commit -m "design: replace affiliate tier badges"
```

### Task 6: Verify Administrator Naming And Full Frontend Quality

**Files:**
- Modify if needed: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`
- Modify if needed: `frontend/src/views/admin/__tests__/AdminAffiliateRecordsTable.spec.ts`

**Step 1: Run affiliate and administrator tests**

Run:

```bash
cd frontend
pnpm test:run src/config/__tests__/affiliateTierPresentation.spec.ts src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts src/views/user/__tests__/AffiliateView.tiers.spec.ts src/views/admin/__tests__/AdminAffiliateRecordsTable.spec.ts src/views/admin/__tests__/SettingsView.spec.ts
```

Expected: PASS, with settings field IDs still using stable internal enum names.

**Step 2: Run static checks without rewriting unrelated files**

Run:

```bash
cd frontend
pnpm typecheck
pnpm lint:check
pnpm build
```

Expected: all commands exit 0. Do not use the fixing lint command because the worktree contains unrelated user changes.

**Step 3: Review the final diff boundary**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; unrelated pre-existing frontend modifications remain unstaged and unchanged.

### Task 7: Browser Verification At Mobile And Desktop Sizes

**Files:**
- No source files unless verification reveals a defect.

**Step 1: Confirm the existing Vite HMR and backend processes serve current code**

Open the affiliate route through the active local frontend at `http://127.0.0.1:3000`. If the backend process is stale, restart only the local backend container/process and confirm its health endpoint before browser testing.

**Step 2: Verify all four themes**

Use fixture data or controlled API responses to render Origin, Pulse, Orbit, and Core. For each tier, confirm badge, name, theme, objective, highlighted statistic, rule list, invite operations, transfer section, and invitee records.

**Step 3: Check responsive layout**

Capture light-mode screenshots at 320px, 390px, and 1440px, plus dark-mode screenshots at 390px and 1440px. At every viewport assert:

```js
document.documentElement.scrollWidth <= document.documentElement.clientWidth
```

Confirm long emails, invite URLs, tier labels, percentages, and objective text do not overlap or escape their containers.

**Step 4: Check motion behavior**

Verify normal mode has only restrained identity animation. Emulate `prefers-reduced-motion: reduce` and confirm badge, pulse, orbit, and light-flow animation stop while the identity remains legible.

**Step 5: Fix any discovered defect with a failing regression test first**

Run the focused test after each fix, then rerun typecheck and build.

**Step 6: Commit verification fixes only if required**

```bash
git add <only-files-changed-for-the-fix>
git commit -m "fix: polish responsive affiliate tier identity"
```

### Task 8: Final Verification And Review

**Files:**
- No planned source changes.

**Step 1: Run the final focused suite**

```bash
cd frontend
pnpm test:run src/config/__tests__/affiliateTierPresentation.spec.ts src/components/affiliate/__tests__/AffiliateTierIdentity.spec.ts src/views/user/__tests__/AffiliateView.tiers.spec.ts src/views/admin/__tests__/AdminAffiliateRecordsTable.spec.ts src/views/admin/__tests__/SettingsView.spec.ts
pnpm typecheck
pnpm lint:check
pnpm build
```

Expected: every command exits 0.

**Step 2: Request code review**

Review specifically for enum compatibility, translation consistency, zero-value calculations, custom-rate behavior, reduced-motion support, mobile overflow, and accidental inclusion of unrelated dirty files.

**Step 3: Inspect branch state**

```bash
git status --short
git log --oneline -8
```

Expected: implementation commits are present; only the user's unrelated pre-existing modifications remain unstaged.
