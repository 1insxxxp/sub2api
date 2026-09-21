# 一键模型测试全量并发实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将管理员账号的一键模型测试从串行改为全量并发，同时保持逐模型结果、耗时和取消能力。

**Architecture:** 复用现有账号测试 SSE 接口和请求体构造逻辑。批量调度创建模型结果快照后，为每个结果启动一个独立异步任务，通过共享 AbortController 支持统一取消，使用 Promise.all 等待所有任务结束。

**Tech Stack:** Vue 3、TypeScript、Vitest、Vue Test Utils、Fetch SSE。

---

### Task 1: Add failing concurrency regression test

**Files:**
- Modify: `frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts`

**Step 1:** Add a test with deferred fetch responses. Start `startBatchTest()`, assert every model request has been issued before resolving any response, then resolve all responses and assert completion.

**Step 2:** Run `pnpm --dir frontend exec vitest run src/components/admin/account/__tests__/AccountTestModal.spec.ts` and confirm the new test fails because only the first request is issued.

### Task 2: Run all model tests concurrently

**Files:**
- Modify: `frontend/src/components/admin/account/AccountTestModal.vue`

**Step 1:** Extract one result execution into a helper that sets `testing`, measures duration, stores success/failure, and ignores stale completion after cancellation.

**Step 2:** Map all pending results to helpers and await `Promise.all`, keeping the shared abort controller and existing cancellation token.

**Step 3:** Run the focused test file and confirm all tests pass.

### Task 3: Verify behavior and quality

**Files:**
- No additional production files.

**Step 1:** Run focused Vitest tests.

**Step 2:** Run ESLint on the component and test.

**Step 3:** Run Vue TypeScript checking if the focused checks pass.

**Step 4:** Review the diff and commit only the plan, component, and test files; leave unrelated `AGENTS.md` and `design/` changes untouched.
