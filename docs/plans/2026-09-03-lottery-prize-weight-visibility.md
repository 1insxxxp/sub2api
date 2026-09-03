# Lottery Prize Weight Visibility Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove weight text from user-facing lottery prize description cards without changing lottery probabilities or admin configuration.

**Architecture:** Keep the existing `LotteryPrize.weight` data flowing through the API and lottery slot machine. Change only the user prize-card template so the display contains the reward amount or inventory status but no weight label/value.

**Tech Stack:** Vue 3, TypeScript, Vue Test Utils, Vitest, pnpm.

---

### Task 1: Lock the user-facing behavior with a failing test

**Files:**
- Modify: `frontend/src/views/user/__tests__/LotteryView.spec.ts`

**Step 1: Write the failing test**

Add a populated state with a prize whose `weight` is `9`, mount the page, and assert the prize card includes its reward text but does not include `权重` or `Weight`/the numeric weight in the weight presentation.

**Step 2: Run test to verify it fails**

Run: `pnpm vitest run src/views/user/__tests__/LotteryView.spec.ts`

Expected: FAIL because the current prize card renders the weight.

### Task 2: Remove only the weight presentation

**Files:**
- Modify: `frontend/src/views/user/LotteryView.vue:104-106`

**Step 1: Write minimal implementation**

Delete the weight `<span>` from the prize-card detail row. Leave the reward amount/inventory branch, API types, admin controls, and slot-machine props unchanged.

**Step 2: Run focused test to verify it passes**

Run: `pnpm vitest run src/views/user/__tests__/LotteryView.spec.ts`

Expected: PASS.

### Task 3: Verify and commit

**Files:**
- Verify: `frontend/src/views/user/LotteryView.vue`
- Verify: `frontend/src/views/user/__tests__/LotteryView.spec.ts`

**Step 1: Run checks**

Run:

```bash
pnpm vitest run src/views/user/__tests__/LotteryView.spec.ts
pnpm typecheck
pnpm exec eslint src/views/user/LotteryView.vue src/views/user/__tests__/LotteryView.spec.ts
pnpm build
```

Expected: all commands pass; build may retain existing Vite/Browserslist warnings.

**Step 2: Commit**

```bash
git add frontend/src/views/user/LotteryView.vue frontend/src/views/user/__tests__/LotteryView.spec.ts
git commit -m "fix: hide lottery prize weights from users"
```
