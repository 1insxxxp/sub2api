# Available Channels Model Card Grid Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the current compact available-model table with a responsive, colorful model-card grid that makes vendor identity, official price, site price, and savings immediately visible.

**Architecture:** Keep `availableChannelCatalog.ts` as the normalized catalog and billing source of truth. Add presentation-only helpers for vendor branding and comparable price summaries, then rebuild `AvailableChannelModelList.vue` as a responsive card grid while continuing to reuse `AvailableChannelModelPrice.vue` for expanded offering details.

**Tech Stack:** Vue 3 Composition API, TypeScript, Tailwind CSS, vue-i18n, Vitest, Vue Test Utils.

---

### Task 1: Vendor Brand Presentation

**Files:**
- Create: `frontend/src/components/channels/availableChannelBrand.ts`
- Create: `frontend/src/components/channels/AvailableChannelBrandIcon.vue`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelBrandIcon.spec.ts`

**Step 1: Write failing tests**

Cover OpenAI, Anthropic, Gemini, known aliases/case variants, and unknown platforms. Assert stable brand keys, distinct color classes, accessible labels, and a deterministic fallback glyph.

**Step 2: Run tests and verify RED**

Run: `cd frontend && npx vitest run src/components/channels/__tests__/AvailableChannelBrandIcon.spec.ts`

Expected: FAIL because the helper and component do not exist.

**Step 3: Implement the brand helper and icon**

Use local code-native SVG/glyphs and existing design tokens. Do not download logos or introduce an icon dependency. Return a stable brand descriptor containing label, foreground, background, border, and glyph identity. Always pair color with visible platform text or an accessible label.

**Step 4: Run tests and verify GREEN**

Run the focused test above. Expected: all tests pass.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/availableChannelBrand.ts frontend/src/components/channels/AvailableChannelBrandIcon.vue frontend/src/components/channels/__tests__/AvailableChannelBrandIcon.spec.ts
git commit -m "feat: add branded model platform icons"
```

### Task 2: Comparable Card Price Summary

**Files:**
- Modify: `frontend/src/components/channels/availableChannelPriceDisplay.ts`
- Modify: `frontend/src/components/channels/availableChannelCatalog.ts`
- Test: `frontend/src/components/channels/__tests__/availableChannelModelList.spec.ts`
- Test: `frontend/src/components/channels/__tests__/availableChannelPriceDisplay.spec.ts`

**Step 1: Write failing tests**

Add cases for input/output/cache/request/image prices, token `/1M` scaling, tiered starting prices, valid zero, unpriced models, and savings percentage. Ensure savings is emitted only when official and site values are finite, official is positive, and both values refer to the same dimension and unit.

**Step 2: Run tests and verify RED**

Run: `cd frontend && npx vitest run src/components/channels/__tests__/availableChannelPriceDisplay.spec.ts src/components/channels/__tests__/availableChannelModelList.spec.ts`

Expected: new comparison assertions fail.

**Step 3: Implement the minimal summary projection**

Expose presentation-ready price dimensions without changing catalog price calculation. Each dimension includes label key, official value, site value, scale, unit key, tier marker, and nullable savings percentage. Preserve existing model/offering references and stable keys.

**Step 4: Run focused tests and verify GREEN**

Expected: all focused tests pass.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/availableChannelPriceDisplay.ts frontend/src/components/channels/availableChannelCatalog.ts frontend/src/components/channels/__tests__/availableChannelModelList.spec.ts frontend/src/components/channels/__tests__/availableChannelPriceDisplay.spec.ts
git commit -m "feat: expose model card price comparisons"
```

### Task 3: Responsive Color Model Cards

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelModelList.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelModelListView.spec.ts`
- Modify: `frontend/src/locales/zh-CN/dashboard.ts`
- Modify: `frontend/src/locales/en-US/dashboard.ts`

**Step 1: Write failing component tests**

Assert card-grid classes, branded icons, model/platform labels, prominent site price, struck-through official price only for real discounts, savings badge, visible input/output/cache/request/image rows, unpriced state, tier marker, offering/channel/group counts, and expanded full offering details.

Add 320px-oriented class assertions for single-column layout, wrapping long names, no fixed widths, and 44px controls. Assert no hardcoded Chinese and no raw translation keys.

**Step 2: Run tests and verify RED**

Run: `cd frontend && npx vitest run src/components/channels/__tests__/AvailableChannelModelListView.spec.ts`

Expected: failures against the existing table-row presentation.

**Step 3: Implement the card grid**

Use a one-column mobile grid, two-column medium grid, three-column xl grid, and four-column only when the available content width supports it. Build each card with a branded header, primary site/official comparison, compact dimension rows, secondary availability counts, and a full-width expandable offering area. Continue using `AvailableChannelModelPrice` for detailed prices.

**Step 4: Add translations**

Add matched Chinese and English keys for savings, starting price, price dimensions, offering counts, expand/collapse, and unpriced states.

**Step 5: Run tests and verify GREEN**

Run the focused component test. Expected: all tests pass.

**Step 6: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelModelList.vue frontend/src/components/channels/__tests__/AvailableChannelModelListView.spec.ts frontend/src/locales/zh-CN/dashboard.ts frontend/src/locales/en-US/dashboard.ts
git commit -m "feat: render available models as colorful price cards"
```

### Task 4: Page Integration and State Regression

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`
- Modify: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Step 1: Write failing integration tests**

Cover card rendering from real API fixtures, search, platform filter, priced-only filter, desktop channel selection, mobile picker selection, selected-channel removal fallback, refresh retaining old cards, no-data/no-results, rate fallback, and selected-channel metadata projection.

**Step 2: Run tests and verify RED**

Run: `cd frontend && npx vitest run src/views/user/__tests__/AvailableChannelsView.spec.ts src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

Expected: only assertions for the new card-grid integration fail.

**Step 3: Make minimal integration changes**

Remove obsolete table-specific wrappers and spacing. Preserve existing API calls, catalog construction, filters, selection ownership, refresh behavior, and mobile picker behavior.

**Step 4: Run integration tests and verify GREEN**

Expected: all integration tests pass.

**Step 5: Commit**

```bash
git add frontend/src/components/channels/AvailableChannelCatalog.vue frontend/src/views/user/AvailableChannelsView.vue frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
git commit -m "feat: integrate available model card grid"
```

### Task 5: Accessibility, Responsive, and Production Verification

**Files:**
- Modify only files proven necessary by failures.

**Step 1: Run relevant channel suites**

Run all tests under `frontend/src/components/channels/__tests__` plus `AvailableChannelsView.spec.ts`. Expected: pass.

**Step 2: Run static verification**

```bash
cd frontend
npm run lint:check
npx vue-tsc --noEmit
npm run build
```

Expected: all commands exit 0; existing non-blocking build warnings may remain documented.

**Step 3: Perform browser verification**

At desktop, tablet, and 320px mobile widths verify: icon color and label, official/site contrast, savings correctness, long-name wrapping, no horizontal page overflow, expandable details, keyboard focus, dark mode, loading, refresh, empty states, and mobile picker behavior.

**Step 4: Fix only reproduced issues with regression tests**

For each issue, add a failing test where practical, implement the smallest correction, and rerun focused plus full relevant verification.

**Step 5: Final commit**

```bash
git add frontend/src/components/channels frontend/src/views/user/AvailableChannelsView.vue frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts frontend/src/locales/zh-CN/dashboard.ts frontend/src/locales/en-US/dashboard.ts
git commit -m "fix: polish available model card grid"
```
