# Available Channel Rate Format Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Display available-channel group and effective rates with exactly two decimal places using downward truncation.

**Architecture:** Add one presentation-only formatter in the channels module and use it for both values in `AvailableChannelOfferingCard`. Keep catalog rate values and all price/billing calculations unchanged.

**Tech Stack:** TypeScript, Vue 3, Vitest, Vue Test Utils.

---

### Task 1: Define downward two-decimal formatting

**Files:**
- Create: `frontend/src/components/channels/availableChannelRateDisplay.ts`
- Create: `frontend/src/components/channels/__tests__/availableChannelRateDisplay.spec.ts`

**Step 1: Write the failing formatter test**

Cover `7.5 -> 7.50`, `1.239 -> 1.23`, `0.035 -> 0.03`, `1.999 -> 1.99`, exact decimal boundaries such as `1.15 -> 1.15`, and invalid values returning `-`.

**Step 2: Run the test to verify RED**

```bash
cd frontend && npx vitest run src/components/channels/__tests__/availableChannelRateDisplay.spec.ts
```

Expected: FAIL because the module does not exist.

**Step 3: Implement the formatter**

Export `formatAvailableChannelRate(value)` that validates finite non-negative input, compensates only for binary floating-point boundary noise, truncates downward at two decimal places, and finishes with `toFixed(2)`.

**Step 4: Run the test to verify GREEN**

```bash
cd frontend && npx vitest run src/components/channels/__tests__/availableChannelRateDisplay.spec.ts
```

Expected: PASS.

### Task 2: Apply the formatter to both visible rates

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelOfferingCard.vue`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts`

**Step 1: Change the component test first**

Assert that the card renders `7.50×` for group rate and `1.23×` for an effective rate of `1.239`, and does not render the previous unpadded values.

**Step 2: Run the component test to verify RED**

```bash
cd frontend && npx vitest run src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts
```

Expected: FAIL because the current local formatter removes trailing zeroes and preserves more than two decimals.

**Step 3: Use the shared formatter**

Import `formatAvailableChannelRate` and use it for both group and effective rates. Delete the duplicate local `rate()` implementation.

**Step 4: Run focused tests to verify GREEN**

```bash
cd frontend && npx vitest run src/components/channels/__tests__/availableChannelRateDisplay.spec.ts src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts
```

Expected: PASS.

**Step 5: Run channel regressions and static checks**

```bash
cd frontend && npx vitest run src/components/channels/__tests__/AvailableChannelModelListView.spec.ts src/components/channels/__tests__/AvailableChannelCatalog.spec.ts && npx vue-tsc --noEmit --pretty false && npx eslint src/components/channels/availableChannelRateDisplay.ts src/components/channels/AvailableChannelOfferingCard.vue src/components/channels/__tests__/availableChannelRateDisplay.spec.ts src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts
```

Expected: all commands exit 0.

**Step 6: Commit**

```bash
git add frontend/src/components/channels/availableChannelRateDisplay.ts frontend/src/components/channels/AvailableChannelOfferingCard.vue frontend/src/components/channels/__tests__/availableChannelRateDisplay.spec.ts frontend/src/components/channels/__tests__/AvailableChannelOfferingCard.spec.ts
git commit -m "fix: truncate available channel rates to two decimals"
```
