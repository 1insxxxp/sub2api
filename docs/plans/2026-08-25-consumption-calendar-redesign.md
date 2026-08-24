# 消耗日历 UI 重构 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在消耗日历单元格内显示每日流水与收益，并以适配桌面和移动端的响应式弹窗展示日期明细。

**Architecture:** 保留现有日历、日期分组和请求日志 API。日历组件负责金额展示与日期选择，面板负责选中日期状态，现有 day drawer 重构为带遮罩的响应式 dialog；桌面居中显示、移动端底部显示，日志分页和分组展开逻辑保持不变。

**Tech Stack:** Vue 3、TypeScript、Tailwind CSS、Vue Test Utils、Vitest、vue-i18n。

---

### Task 1: 为日期单元格金额和弹窗行为写失败测试

**Files:**
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`
- Test target: existing commission calendar tests

**Step 1: 更新移动端日历断言**

- 断言有数据日期格显示日期、消耗金额和收益金额。
- 断言金额使用紧凑布局 class，完整金额仍存在于 `aria-label`。
- 删除“日历下方选中日期摘要卡”的旧断言，改为点击日期后出现 dialog。

**Step 2: 添加弹窗行为断言**

- 点击日期后断言 `[data-test="commission-day-dialog"]` 存在并包含日期、当日消耗和收益。
- 断言关闭按钮触发后 dialog 消失。
- 断言分组展开仍加载对应请求日志。

**Step 3: 运行测试确认失败**

Run: `cd frontend && npm run test:unit -- src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: FAIL，因为日期格尚未渲染金额且明细组件还不是 dialog。

---

### Task 2: 在日历单元格中展示每日消耗与收益

**Files:**
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue`

**Step 1: 增加紧凑金额格式化函数**

- 小于 1000 保留两位小数。
- 1000 及以上使用 `K/M` 紧凑表示，避免移动端单元格溢出。
- `title` 和 `aria-label` 继续使用完整金额。

**Step 2: 修改日期 cell 模板**

- 保留日期数字。
- 有数据时添加消耗和收益两行文字。
- 使用 `data-test="commission-calendar-day-<date>-amounts"` 作为稳定测试入口。
- 根据有数据/可选中/选中状态调整颜色和边框。

**Step 3: 移除日历下方选中摘要卡**

- 保留选中日期状态用于 aria-pressed 和选中样式。
- 删除摘要卡模板和相关展示逻辑，不删除日期选择事件。

**Step 4: 运行 Task 1 测试**

Run: `cd frontend && npm run test:unit -- src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: 日历金额相关断言通过，dialog 相关断言仍失败。

---

### Task 3: 将日期明细抽屉重构为响应式 dialog

**Files:**
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionDayDrawer.vue`
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionPanel.vue`

**Step 1: 修改组件语义和布局**

- 根节点改为 `role="dialog"`、`aria-modal="true"`、`data-test="commission-day-dialog"`。
- 增加全屏遮罩和 dialog card。
- 桌面端使用居中卡片和最大宽度/最大高度。
- 移动端使用底部对齐、顶部圆角和 `max-h-[90dvh]`，内容区独立滚动。

**Step 2: 增加关闭交互**

- 保留关闭按钮并补充可访问名称。
- 点击遮罩关闭。
- 监听 Escape 关闭，并在卸载时移除监听。
- 通过 `watch(date)` 或组件生命周期控制 body overflow，避免弹窗打开时背景滚动。

**Step 3: 增加日期汇总头部**

- 从日历数据通过 props 传入选中日期的 `actual_cost` 和 `commission_amount`，或在面板层传入可计算的选中日对象。
- 弹窗顶部展示日期、消耗和收益。
- 明细分组和请求日志沿用现有 API 与分页行为。

**Step 4: 更新面板传参**

- `SubAdminCommissionPanel` 保留 selectedDate 状态。
- 监听日历选择事件并将日期摘要传给 dialog。
- 关闭时将 selectedDate 置空。

**Step 5: 运行相关测试**

Run: `cd frontend && npm run test:unit -- src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: 所有 commission calendar/dialog 测试通过。

---

### Task 4: 补充移动端可访问性和样式回归测试

**Files:**
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue`
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionDayDrawer.vue`

**Step 1: 增加移动端断言**

- 日期格包含金额但不依赖固定桌面宽度。
- dialog 使用底部布局 class、最大高度和滚动内容 class。
- 日志卡片仍能断行，不产生横向溢出。

**Step 2: 增加可访问性断言**

- dialog 的 `role`、`aria-modal` 和标题关联存在。
- 日期格 aria-label 包含日期、消耗、收益。
- 关闭按钮有 aria-label。

**Step 3: 运行单测**

Run: `cd frontend && npm run test:unit -- src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: PASS。

---

### Task 5: 完成前端验证

**Files:**
- No source changes expected.

**Step 1: 运行相关 API 与组件测试**

Run: `cd frontend && npm run test:unit -- src/api/__tests__/admin.subAdminCommission.spec.ts src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

**Step 2: 运行前端类型检查/构建**

Run: `cd frontend && npm run build`

Expected: 测试和构建均成功，无 TypeScript 或模板编译错误。

**Step 3: 检查工作区**

Run: `git status --short --branch`

确认只包含本需求变更和原有未跟踪 `local-db-backups/`，不提交、不删除后者。

