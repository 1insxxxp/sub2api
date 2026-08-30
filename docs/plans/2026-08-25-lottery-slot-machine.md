# 抽奖老虎机动画 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将用户端抽奖入口升级为参考图风格的三列老虎机滚轴，并保证动画最终停在服务端返回的真实奖品。

**Architecture:** 新增轻量的 Vue `LotterySlotMachine` 展示组件，组件负责滚轴预览、滚动、减速和减少动态模式；`LotteryView` 继续负责抽奖 API、次数、结果弹窗和历史记录。服务端不改，前端通过现有 draw 响应中的 `prize_id` 将中奖奖品传给滚轴。

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Vue Test Utils, Vitest, Tailwind CSS, CSS transitions/keyframes, vue-i18n.

---

### Task 1: Extend the lottery draw contract

**Files:**
- Modify: `/Users/alien/Documents/sub2/frontend/src/api/lottery.ts:29-39`
- Test: `/Users/alien/Documents/sub2/frontend/src/api/__tests__/lottery.spec.ts`

**Step 1: Write the failing test**

Add a draw response fixture with `prize_id` and assert the typed response exposes the server-selected prize identifier used by the slot machine.

**Step 2: Run test to verify it fails**

Run: `cd /Users/alien/Documents/sub2/frontend && pnpm vitest run src/api/__tests__/lottery.spec.ts`

Expected: FAIL because `LotteryDraw` does not declare `prize_id`.

**Step 3: Write minimal implementation**

Add `prize_id?: number | null` to `LotteryDraw`, preserving compatibility with older history records.

**Step 4: Run test to verify it passes**

Run the same Vitest command. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/lottery.ts frontend/src/api/__tests__/lottery.spec.ts
git commit -m "feat: expose lottery prize id to the frontend"
```

### Task 2: Build the slot machine component with behavior tests

**Files:**
- Create: `/Users/alien/Documents/sub2/frontend/src/components/lottery/LotterySlotMachine.vue`
- Create: `/Users/alien/Documents/sub2/frontend/src/components/lottery/__tests__/LotterySlotMachine.spec.ts`

**Step 1: Write the failing tests**

Cover these behaviors:

- Renders three fixed slot columns and the configured prize names.
- Repeatedly fills a short prize list so all three columns have content.
- When `is-drawing` becomes true, exposes a spinning state and marks the control as busy.
- When `winner-id` arrives, the winning prize becomes the center item and the component emits `settled` after the animation timer.
- With reduced motion enabled, it skips the long delay but still emits `settled` and shows the winning prize.
- When the request fails and the parent clears `is-drawing` without a winner, it returns to the idle state without showing a fake result.

**Step 2: Run test to verify it fails**

Run: `cd /Users/alien/Documents/sub2/frontend && pnpm vitest run src/components/lottery/__tests__/LotterySlotMachine.spec.ts`

Expected: FAIL because the component does not exist.

**Step 3: Write minimal implementation**

Implement the component with:

- Props `prizes`, `isDrawing`, `winnerId`, and optional `reducedMotion` override for deterministic tests.
- Three computed/reusable reel sequences, each with stable card dimensions and a center highlight band.
- A small state machine: idle, spinning, settling, settled. Start rolling when `isDrawing` changes to true; wait for `winnerId`; settle to the matching prize and emit `settled`.
- CSS transitions for the stop sequence, an animated sheen while spinning, `aria-busy`, and `aria-live="polite"` only for the settled winner.
- `prefers-reduced-motion` detection with the explicit prop taking precedence.

**Step 4: Run test to verify it passes**

Run the same Vitest command. Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/lottery/LotterySlotMachine.vue frontend/src/components/lottery/__tests__/LotterySlotMachine.spec.ts
git commit -m "feat: add lottery slot machine animation"
```

### Task 3: Integrate the animation into the user lottery page

**Files:**
- Modify: `/Users/alien/Documents/sub2/frontend/src/views/user/LotteryView.vue`
- Modify: `/Users/alien/Documents/sub2/frontend/src/i18n/locales/zh/lottery.ts`
- Modify: `/Users/alien/Documents/sub2/frontend/src/i18n/locales/en/lottery.ts`

**Step 1: Write the failing integration assertions**

Extend or add page-level assertions for the presence of the slot machine, disabled draw action during animation, server `prize_id` being passed through, and existing result/history refresh behavior remaining intact.

**Step 2: Run test to verify it fails**

Run: `cd /Users/alien/Documents/sub2/frontend && pnpm vitest run src/components/lottery/__tests__/LotterySlotMachine.spec.ts src/api/__tests__/lottery.spec.ts`

Expected: FAIL on the page integration assertions because the old static prize grid is still the main draw UI.

**Step 3: Write minimal implementation**

- Replace the primary static prize grid/action row with `LotterySlotMachine` and a full-width draw button styled like the reference.
- Keep a compact prize explanation/list below the machine so users can still inspect descriptions and inventory.
- Track `winnerPrizeId` in the page; set it from `result.draw.prize_id` after a successful draw and clear it before each new draw.
- Keep `drawing` true until the component emits `settled`; on request errors clear it immediately.
- Keep the existing result modal, product copy action, history refresh and no-activity handling.
- Add Chinese/English keys for slot instructions, result announcement and reduced-motion labels.

**Step 4: Run test to verify it passes**

Run focused lottery tests, then run:

```bash
cd /Users/alien/Documents/sub2/frontend
pnpm typecheck
pnpm exec eslint src/api/lottery.ts src/api/__tests__/lottery.spec.ts src/components/lottery/LotterySlotMachine.vue src/components/lottery/__tests__/LotterySlotMachine.spec.ts src/views/user/LotteryView.vue src/i18n/locales/zh/lottery.ts src/i18n/locales/en/lottery.ts
```

Expected: all focused tests, typecheck and lint pass.

**Step 5: Commit**

```bash
git add frontend/src/api/lottery.ts frontend/src/api/__tests__/lottery.spec.ts frontend/src/components/lottery frontend/src/views/user/LotteryView.vue frontend/src/i18n/locales/zh/lottery.ts frontend/src/i18n/locales/en/lottery.ts
git commit -m "feat: integrate slot machine lottery experience"
```

### Task 4: Verify production behavior and preserve unrelated work

**Files:**
- No unrelated files should be staged.

**Step 1: Run the complete focused verification**

Run:

```bash
cd /Users/alien/Documents/sub2/frontend
pnpm vitest run src/components/lottery/__tests__/LotterySlotMachine.spec.ts src/api/__tests__/lottery.spec.ts src/i18n/__tests__/lotteryLocales.spec.ts
pnpm typecheck
pnpm build
git diff --check
```

Expected: all tests, typecheck and production build pass; only expected Vite dependency chunk warnings may appear.

**Step 2: Confirm local services**

Run:

```bash
curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3000/
curl -fsS http://127.0.0.1:18081/health
```

Expected: frontend `200` and backend `{"status":"ok"}`.

**Step 3: Review the final diff**

Run: `git status --short --branch && git log --oneline --decorate -6`

Confirm the pre-existing unrelated worktree changes remain unstaged and the lottery commits are on `dev`.
