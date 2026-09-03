# Lottery Balance History Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refresh the logged-in balance after a successful lottery draw and show balance-type lottery rewards in the user's redeem-page activity area.

**Architecture:** Keep `lottery_draws` as the source of truth for lottery rewards and keep `redeem_codes` as the source of truth for redeem-code history. The frontend loads both sources independently, renders lottery balance rewards in a dedicated activity block, and treats either history request as independently optional. `LotteryView` refreshes the auth user after a successful draw without changing the draw transaction or creating duplicate ledger rows.

**Tech Stack:** Vue 3 `<script setup>`, Pinia auth store, TypeScript API clients, Vitest + Vue Test Utils, Vite.

---

### Task 1: Add a failing test for immediate balance refresh after a draw

**Files:**
- Modify: `frontend/src/views/user/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/views/user/LotteryView.vue`

**Step 1: Write the failing test**

Mock `useAuthStore` with `refreshUser`, return a balance prize from `draw`, mount the view, trigger the draw button, and assert `refreshUser` is called after the draw succeeds.

**Step 2: Run the focused test to verify it fails**

Run: `cd /Users/alien/Documents/sub2/frontend && npm run test -- --run src/views/user/__tests__/LotteryView.spec.ts`

Expected: the new assertion fails because `LotteryView.handleDraw` currently does not refresh the auth store.

**Step 3: Implement the minimal code**

Import and instantiate `useAuthStore` in `LotteryView.vue`; call `await authStore.refreshUser()` immediately after `lotteryAPI.draw` resolves. Catch refresh errors and log them while preserving the successful draw result.

**Step 4: Run the focused test to verify it passes**

Run the same Vitest command; expected result is PASS for the new test and existing lottery tests.

**Step 5: Commit**

```bash
git add frontend/src/views/user/LotteryView.vue frontend/src/views/user/__tests__/LotteryView.spec.ts
git commit -m "fix: refresh balance after lottery draw"
```

### Task 2: Add a failing test for lottery balance rewards on the redeem page

**Files:**
- Modify: `frontend/src/views/user/__tests__/RedeemView.checkinReward.spec.ts`
- Modify: `frontend/src/views/user/RedeemView.vue`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`

**Step 1: Write the failing test**

Mock `lotteryAPI.history` with a balance draw and a product draw. Mount `RedeemView` and assert the dedicated lottery reward title and formatted positive dollar amount are rendered, while the product draw is not rendered in that balance-only block.

**Step 2: Run the focused test to verify it fails**

Run: `cd /Users/alien/Documents/sub2/frontend && npm run test -- --run src/views/user/__tests__/RedeemView.checkinReward.spec.ts`

Expected: the new assertion fails because `RedeemView` currently only requests redeem-code history.

**Step 3: Implement the minimal code**

Import `lotteryAPI` and `LotteryDraw`, load the first page of lottery history independently, filter `prize_type === 'balance'`, and render a dedicated section with localized title, amount, and timestamp. Keep the existing redeem-code list and pagination unchanged; a lottery history error must not prevent redeem history from rendering.

**Step 4: Run the focused test to verify it passes**

Run the same Vitest command; expected result is PASS for the new assertion and existing redeem tests.

**Step 5: Commit**

```bash
git add frontend/src/views/user/RedeemView.vue frontend/src/views/user/__tests__/RedeemView.checkinReward.spec.ts frontend/src/i18n/locales/en/dashboard.ts frontend/src/i18n/locales/zh/dashboard.ts
git commit -m "feat: show lottery balance rewards in redeem activity"
```

### Task 3: Run regression checks and review the diff

**Files:**
- Verify: `frontend/src/api/lottery.ts`
- Verify: `frontend/src/views/user/LotteryView.vue`
- Verify: `frontend/src/views/user/RedeemView.vue`

**Step 1: Run frontend tests**

Run: `cd /Users/alien/Documents/sub2/frontend && npm run test -- --run src/views/user/__tests__/LotteryView.spec.ts src/views/user/__tests__/RedeemView.checkinReward.spec.ts src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts src/views/user/__tests__/RedeemView.subscriptionGuide.spec.ts`

Expected: all selected tests pass.

**Step 2: Run frontend type/build checks**

Run: `cd /Users/alien/Documents/sub2/frontend && npm run build`

Expected: Vite production build completes successfully.

**Step 3: Review hygiene**

Run: `git diff --check && git status --short --branch`

Expected: no whitespace errors; only intended commits are present and the worktree is clean.
