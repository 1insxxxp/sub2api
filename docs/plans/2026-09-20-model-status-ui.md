# 模型状态页面 UI 优化 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 统一模型状态页的工作区材质并修复移动端状态、筛选、错误和分组上下文的可读性。

**Architecture:** 仅改 `ModelStatusView.vue` 的模板和 scoped CSS，以及现有模型状态测试；状态请求、自动刷新、滚动分页和桶详情逻辑保持不变。页面使用现有 workspace token，公共组件继续复用 `Select`、`BaseDialog` 和现有 `ModelIcon`。

**Tech Stack:** Vue 3、TypeScript、Vitest、Vue Test Utils、Tailwind CSS。

---

### Task 1: 模型状态页面材质和移动结构

**Files:**
- Modify: `frontend/src/views/ModelStatusView.vue`
- Modify: `frontend/src/views/__tests__/ModelStatusView.spec.ts`

**Steps:**

1. 在测试中增加失败契约：页面使用 workspace surface 语义类；失败状态有重试按钮；筛选无结果有清除筛选入口；移动模型身份区允许独立换行；sticky 工具栏有深色主题类契约。
2. 运行 `cd frontend && pnpm exec vitest run src/views/__tests__/ModelStatusView.spec.ts`，确认新增断言失败。
3. 调整模板和 scoped CSS：用 workspace token 收敛标题栏、筛选区、分组标题、模型卡和详情统计；补齐移动端 320–640px 的换行/单列布局；深色 sticky 背景同步；保留分组上下文名称；加入重试和清除筛选操作；补齐 focus-visible 和 reduced-motion。
4. 再运行同一测试，确认通过；运行 `pnpm exec eslint src/views/ModelStatusView.vue src/views/__tests__/ModelStatusView.spec.ts` 和 `git diff --check`。
5. 提交：`git commit -m "style: unify model status workspace UI"`。
