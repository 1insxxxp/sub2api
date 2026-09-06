# Model Status Refresh Countdown Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在模型状态标题栏增加与现有 30 秒轮询同步的自动刷新倒计时。

**Architecture:** `ModelStatusView.vue` 维护倒计时秒数和每秒计时器；请求完成时重置倒计时，现有 30 秒轮询继续负责拉取数据。标题栏新增可访问的倒计时文本，CSS 在窄屏下压缩显示。

**Tech Stack:** Vue 3 Composition API, TypeScript, Vitest, Vue Test Utils.

---

### Task 1: Add countdown behavior tests

**Files:**
- Modify: `frontend/src/views/__tests__/ModelStatusView.spec.ts`

**Step 1:** Add tests asserting the countdown starts at 30, decreases after one second, and resets after a manual refresh resolves.

**Step 2:** Run `pnpm vitest run src/views/__tests__/ModelStatusView.spec.ts` and confirm the new assertions fail because the countdown is not rendered yet.

### Task 2: Implement countdown state and UI

**Files:**
- Modify: `frontend/src/views/ModelStatusView.vue`

**Step 1:** Add a 30-second countdown ref and a one-second interval managed with the existing mount/unmount lifecycle.

**Step 2:** Reset the countdown when each refresh request settles, and keep the existing 30-second data polling interval unchanged.

**Step 3:** Render the countdown beside the refresh button with desktop and mobile responsive text styles.

### Task 3: Verify and commit

**Files:**
- No additional files.

**Step 1:** Run the focused model status test.

**Step 2:** Run `pnpm typecheck` and `pnpm build` from `frontend`.

**Step 3:** Run `git diff --check`, inspect the diff, and commit the implementation.
