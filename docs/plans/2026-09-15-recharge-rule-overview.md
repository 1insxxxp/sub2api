# 充值规则总览 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在用户充值页快捷金额上方展示每个预设金额对应的到账余额与赠送额度，不展示倍率。

**Architecture:** 复用充值页已有 checkout 配置和 `resolveRechargeMultiplier` 计算逻辑，新增只读规则总览组件；金额选择仍由现有快捷金额控件处理，规则总览与支付明细共享同一计算口径。使用响应式网格适配移动端。

**Tech Stack:** Vue 3、TypeScript、Tailwind CSS、Vitest。

---

### Task 1: 充值规则总览

**Files:**
- Modify: `frontend/src/views/user/PaymentView.vue`
- Test: `frontend/src/views/user/__tests__/PaymentView.spec.ts`

**Steps:**
1. 为规则总览增加测试数据和断言，验证所有快捷金额均展示到账金额与赠送额度，且不出现倍率文案。
2. 运行对应测试确认新增断言失败。
3. 在快捷金额区上方渲染规则总览，使用现有 checkout tiers 或默认金额列表；金额到账按现有倍率计算，赠送额度为到账减充值金额。
4. 使用两列移动端网格、桌面端多列布局，长内容可换行。
5. 运行充值页测试、类型检查和构建。
6. 提交实现变更。

