# 空回申请独立详情入口 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为所有状态的空回申请增加始终可用的只读详情入口，同时保持现有审批和批量审批流程不变。

**Architecture:** 复用现有 `EmptyResponseClaimReviewDialog`，将动作类型扩展为 `view | approve | reject`。列表为每条记录增加查看按钮；只读模式复用全部结构化上下文但隐藏审核表单与提交动作。

**Tech Stack:** Vue 3、TypeScript、Vue I18n、Vitest、Tailwind CSS。

---

### Task 1: 锁定只读详情行为

**Files:**
- Modify: `frontend/src/components/admin/usage/__tests__/EmptyResponseClaimsPanel.spec.ts`

**Step 1: Write the failing test**

增加已补偿申请夹具，断言：

- 列表存在 `view-claim-{id}`；
- 点击后显示请求 ID、Token 和响应证据；
- 弹窗不存在 `textarea` 与 `submit-claim-review`；
- 已补偿记录不存在审批/驳回按钮。

**Step 2: Run test to verify it fails**

Run: `cd frontend && npm run test:run -- src/components/admin/usage/__tests__/EmptyResponseClaimsPanel.spec.ts`

Expected: FAIL because `view-claim-{id}` does not exist.

### Task 2: 实现列表详情入口和弹窗只读模式

**Files:**
- Modify: `frontend/src/components/admin/usage/EmptyResponseClaimsPanel.vue`
- Modify: `frontend/src/components/admin/usage/EmptyResponseClaimReviewDialog.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/en/admin/resources.ts`

**Step 1: Extend the action type**

将父组件和弹窗的动作类型扩展为 `view | approve | reject`，增加 `openDetail(claim)` 并保持审批、驳回与批量路径不变。

**Step 2: Add the always-visible button**

在移动卡片和桌面操作列增加次要样式的“查看详情”按钮；待审核记录旁继续显示审批与驳回。

**Step 3: Make the dialog read-only**

`view` 模式使用详情标题，隐藏备注表单和双按钮 footer，只显示一个全宽关闭按钮。继续复用现有结构化上下文和移动端底部弹窗布局。

**Step 4: Add bilingual labels**

增加中英文 `viewDetails`、`detailTitle` 与 `closeDetail` 文案，保持 locale parity。

**Step 5: Run focused tests to verify GREEN**

Run: `cd frontend && npm run test:run -- src/components/admin/usage/__tests__/EmptyResponseClaimsPanel.spec.ts src/i18n/__tests__/localeParity.spec.ts`

Expected: PASS.

### Task 3: 完成本地验证与提交

**Files:**
- No additional files.

**Step 1: Run type checking**

Run: `cd frontend && npm run typecheck`

Expected: PASS.

**Step 2: Run production build**

Run: `cd frontend && npm run build`

Expected: PASS.

**Step 3: Review the diff**

Run: `git diff --check && git status --short`

Expected: only the intended frontend files and this plan, plus the pre-existing untracked root package files.

**Step 4: Commit**

Stage only the intended files and commit with `feat: add empty response claim detail view`. Do not push or deploy.
