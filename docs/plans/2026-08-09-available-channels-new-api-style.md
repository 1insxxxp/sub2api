# 可用渠道模型广场风格 UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将可用渠道页面改成参考 new-api 模型广场的紧凑模型列表风格，同时保持现有数据、计价、筛选和移动端行为。

**Architecture:** 保留 `AvailableChannelsView` 的 API 与 catalog 构建层，新增面向模型的展示层，把同名模型按渠道/分组上下文安全分组。桌面端用紧凑可展开列表，移动端复用现有底部筛选弹窗和单列价格卡片；价格始终来自 `availableChannelCatalog`，不在 UI 重算。

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Tailwind utility classes, Vue Test Utils/Vitest, vue-i18n.

---

### Task 1: 建立模型列表视图模型

**Files:**
- Modify: `frontend/src/components/channels/availableChannelCatalog.ts`
- Create: `frontend/src/components/channels/__tests__/availableChannelModelList.spec.ts`

**Steps:**
1. 写失败测试：模型列表保留渠道/分组上下文、同名模型不错误合并、按模型名稳定排序、价格字段直接引用 catalog。
2. 运行聚焦 Vitest，确认 RED。
3. 实现最小纯函数和类型，不新增 API 请求或计价公式。
4. 运行测试确认 GREEN，并执行 ESLint。
5. 提交 `feat: add model plaza list view model`。

### Task 2: 实现紧凑模型列表与展开详情

**Files:**
- Create: `frontend/src/components/channels/AvailableChannelModelList.vue`
- Create: `frontend/src/components/channels/__tests__/AvailableChannelModelList.spec.ts`
- Modify: `frontend/src/components/channels/AvailableChannelModelPrice.vue`

**Steps:**
1. 写失败测试：桌面端模型、平台、官方价、本站价同屏；展开详情含渠道/分组；键盘和 ARIA 状态正确；移动端无横向滚动。
2. 运行测试确认 RED。
3. 实现列表行、展开区域、状态标签和 responsive 单列卡片；复用价格组件。
4. 运行聚焦测试确认 GREEN，检查暗色、长名称、未定价和刷新状态。
5. 提交 `feat: add compact available model list`。

### Task 3: 接入页面筛选与模型广场视觉层

**Files:**
- Modify: `frontend/src/views/user/AvailableChannelsView.vue`
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`
- Test: `frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`

**Steps:**
1. 写失败集成测试：搜索/平台/仅有价筛选传入模型列表；刷新保留旧内容；无数据和无结果状态区分。
2. 运行集成测试确认 RED。
3. 替换目录主体为模型列表，同时保留桌面渠道导航、移动渠道 picker 和倍率降级提示。
4. 添加中英文模型广场文案，确保 aria-label 和状态文字完整。
5. 运行页面、目录、价格及模型列表测试，确认 GREEN。
6. 提交 `feat: restyle available channels as model plaza`。

### Task 4: 响应式与无障碍回归

**Files:**
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelModelList.spec.ts`
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Steps:**
1. 补充窄屏 320px、桌面 xl、长文本、键盘展开、焦点恢复、减少动画测试。
2. 使用单线程运行全部相关 Vitest，避免本机并发磁盘压力。
3. 执行 `npm run lint:check`、`npm run typecheck`、`git diff --check`。
4. 运行生产构建 `npm run build`，确认没有旧表格生产引用。
5. 提交 `test: cover model plaza responsive regressions`。

### Task 5: 合并前审查

**Steps:**
1. 由独立审查代理检查 API/计费未改变、模型身份稳定、移动端无横滚、官方价/本站价语义正确。
2. 修复 Critical/Important 问题并补回归测试。
3. 在主 `dev` 工作区执行相关测试、lint、typecheck、build。
4. 合并功能分支到 `dev`；本轮不部署线上，待用户验收后再推送/部署。
