# 管理端 UI 统一审计与分阶段改造 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将管理员端公共组件和五个高频页面统一到现有中性工作区视觉基准，并补齐 320–640px 移动布局、键盘焦点、深色模式和减少动态效果支持。

**Architecture:** 先在公共组件层收敛 appearance 上下文、材质令牌和移动布局，再按仪表盘、账号、用户/分组、渠道/监控的顺序迁移页面。页面只接入明确的工作区类和组件外观，不改接口、权限、数据字段或业务流程；每个阶段保留独立测试和可回滚提交。

**Tech Stack:** Vue 3、TypeScript、Vue Test Utils、Vitest、Tailwind CSS、现有 `workspace-tokens.css` / `admin-workspace.css`。

---

### Task 1: 固化管理员公共材质和测试基线

**Files:**
- Modify: `frontend/src/styles/workspace-tokens.css`
- Modify: `frontend/src/styles/admin-workspace.css`
- Create: `frontend/src/components/admin/__tests__/AdminWorkspaceTokens.spec.ts`
- Reference: `frontend/src/components/admin/__tests__/AdminComponentSurfaces.spec.ts`

**Step 1: Write the failing test**

在新测试中挂载一个带 `admin-console-theme` 的最小容器，断言公共表面、控件、分隔线和焦点状态使用工作区变量，并检查深色主题和 `prefers-reduced-motion` 的规则入口存在。

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/components/admin/__tests__/AdminWorkspaceTokens.spec.ts`

Expected: FAIL，因为新测试引用的统一材质选择器和变量尚未完整覆盖。

**Step 3: Write minimal implementation**

补齐 `workspace-tokens.css` 中浅色/深色的 surface、control、rule、divider、ink、muted、hover、shadow 令牌；在 `admin-workspace.css` 中把 `.admin-surface`、`.admin-table-stage`、`.admin-list-surface`、输入控件和焦点态收敛到这些令牌。只添加缺失规则，保留现有显式外观覆盖优先级，并为过渡属性添加统一的 reduced-motion 入口。

**Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm vitest run src/components/admin/__tests__/AdminWorkspaceTokens.spec.ts src/components/admin/__tests__/AdminComponentSurfaces.spec.ts`

Expected: PASS。

**Step 5: Commit**

```bash
git add frontend/src/styles/workspace-tokens.css frontend/src/styles/admin-workspace.css frontend/src/components/admin/__tests__/AdminWorkspaceTokens.spec.ts
git commit -m "style: consolidate admin workspace tokens"
```

### Task 2: 统一 DataTable 的加载、空状态和移动记录表面

**Files:**
- Modify: `frontend/src/components/common/DataTable.vue`
- Modify: `frontend/src/components/common/__tests__/DataTable.spec.ts`
- Reference: `frontend/src/styles/admin-workspace.css`

**Step 1: Write the failing test**

在 `DataTable.spec.ts` 增加桌面 loading、移动 loading、空状态和普通移动行的断言：默认记录使用 `admin-record` 或统一 surface 类，骨架不再出现硬编码白底，空状态和分隔线带有可被主题覆盖的类名。

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/components/common/__tests__/DataTable.spec.ts`

Expected: FAIL，因为当前骨架和普通移动卡直接使用 `bg-white`、`border-gray-*` 和 `rounded-2xl`。

**Step 3: Write minimal implementation**

将 loading skeleton、空状态、普通移动记录和操作分隔线改为语义类名；由 `admin-workspace.css` 提供浅色/深色 surface、rule、highlight 和圆角。保留现有 `mobileLayout`、slot、选择、排序、虚拟滚动和事件逻辑不变。长字段继续使用 `overflow-wrap:anywhere`，避免为了视觉统一引入横向裁切。

**Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm vitest run src/components/common/__tests__/DataTable.spec.ts`

Expected: PASS。

**Step 5: Commit**

```bash
git add frontend/src/components/common/DataTable.vue frontend/src/components/common/__tests__/DataTable.spec.ts
git commit -m "style: unify data table loading and mobile surfaces"
```

### Task 3: 完成 Pagination 和 BaseDialog 的移动外观收敛

**Files:**
- Modify: `frontend/src/components/common/Pagination.vue`
- Modify: `frontend/src/components/common/BaseDialog.vue`
- Modify: `frontend/src/components/common/__tests__/Pagination.jump.spec.ts`
- Modify: `frontend/src/components/common/__tests__/BaseDialog.spec.ts`

**Step 1: Write the failing test**

增加管理员上下文下 Pagination 自动使用 compact 的回归断言；增加 320px 视口下分页控件可换行、按钮触控尺寸达标的结构断言；增加 neutral BaseDialog footer 在窄屏纵向排列、body 可滚动且不会超出 viewport 的 class/样式契约断言。

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/components/common/__tests__/Pagination.jump.spec.ts src/components/common/__tests__/BaseDialog.spec.ts`

Expected: FAIL，因为部分默认分支仍使用旧分页 shell，neutral dialog 还没有窄屏 footer 和宽度约束契约。

**Step 3: Write minimal implementation**

保留 `Pagination` 的显式 `variant` 优先级，补齐 admin appearance 缺失的 shell 和 dark/mobile 规则；在 `BaseDialog` 的 neutral 样式中加入 320–639px 的 bottom-sheet-like 高度约束、稳定的 header/body/footer 分区和 footer 按钮换行规则，桌面继续居中。所有按钮维持至少 44px 触控尺寸，加入 reduced-motion 规则。

**Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm vitest run src/components/common/__tests__/Pagination.jump.spec.ts src/components/common/__tests__/BaseDialog.spec.ts`

Expected: PASS。

**Step 5: Commit**

```bash
git add frontend/src/components/common/Pagination.vue frontend/src/components/common/BaseDialog.vue frontend/src/components/common/__tests__/Pagination.jump.spec.ts frontend/src/components/common/__tests__/BaseDialog.spec.ts
git commit -m "style: stabilize admin pagination and dialogs on mobile"
```

### Task 4: 统一管理员仪表盘的卡片、筛选和响应式布局

**Files:**
- Modify: `frontend/src/views/admin/DashboardView.vue`
- Modify: `frontend/src/views/admin/__tests__/DashboardView.spec.ts`
- Modify: `frontend/src/styles/admin-workspace.css`

**Step 1: Write the failing test**

为统计卡增加结构断言：图标使用统一中性容器并保留厂商色作为小点缀；筛选区在窄屏包含独立操作行；统计卡在 320px 下不会固定为挤压的双列。保留现有数据加载、图表和刷新断言。

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/DashboardView.spec.ts`

Expected: FAIL，因为当前统计图标仍有彩色块，筛选控件和统计卡没有稳定的窄屏契约。

**Step 3: Write minimal implementation**

把统计卡图标背景改为 workspace control/surface，颜色只用于图标本身；将日期、粒度、刷新操作拆成可换行的 toolbar 行，320–639px 下操作区独占一行；按信息优先级调整统计卡网格，长数字使用 tabular numerals 和可换行文本。不要修改 API 参数、图表数据或刷新时序。

**Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/DashboardView.spec.ts`

Expected: PASS。

**Step 5: Commit**

```bash
git add frontend/src/views/admin/DashboardView.vue frontend/src/views/admin/__tests__/DashboardView.spec.ts frontend/src/styles/admin-workspace.css
git commit -m "style: align admin dashboard workspace layout"
```

### Task 5: 迁移管理员账号、用户和分组页面

**Files:**
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Modify: `frontend/src/views/admin/UsersView.vue`
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/views/admin/__tests__/AccountsView.lite.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/UsersView.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/GroupsView.spec.ts`
- Reference: `frontend/src/components/common/GroupOptionItem.vue`

**Step 1: Write the failing test**

为三页增加页面级材质契约：工具栏使用 `AdminListToolbar`/workspace 类；表格分页使用公共 appearance；选择卡沿用 `GroupOptionItem` 的中性表面；账号错误详情在移动端仍可读并支持换行；所有主要操作按钮在窄屏拥有 44px 触控尺寸。

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/AccountsView.lite.spec.ts src/views/admin/__tests__/UsersView.spec.ts src/views/admin/__tests__/GroupsView.spec.ts`

Expected: FAIL，因为页面仍有旧卡片、直接灰白背景、固定 tooltip 或移动端操作挤压。

**Step 3: Write minimal implementation**

逐页将外层 surface、筛选工具栏、分页、状态徽标和表单分区接入现有 admin workspace 类；删除大圆角和层层套卡的默认样式，保留业务特有的状态色和字段密度。错误详情改用可换行文本或可聚焦详情入口；不删除错误内容、不改变批量选择、更新、删除、调度和提交逻辑。

**Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/AccountsView.lite.spec.ts src/views/admin/__tests__/UsersView.spec.ts src/views/admin/__tests__/GroupsView.spec.ts`

Expected: PASS。

**Step 5: Commit**

```bash
git add frontend/src/views/admin/AccountsView.vue frontend/src/views/admin/UsersView.vue frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/__tests__/AccountsView.lite.spec.ts frontend/src/views/admin/__tests__/UsersView.spec.ts frontend/src/views/admin/__tests__/GroupsView.spec.ts
git commit -m "style: align admin account user and group pages"
```

### Task 6: 迁移管理员渠道和渠道监控页面

**Files:**
- Modify: `frontend/src/views/admin/ChannelsView.vue`
- Modify: `frontend/src/views/admin/ChannelMonitorView.vue`
- Modify: `frontend/src/views/admin/__tests__/ChannelsView.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/ChannelMonitorView.checkModeBadge.spec.ts`
- Reference: `frontend/src/components/channels/AvailableChannelsTable.vue`

**Step 1: Write the failing test**

增加渠道表格和监控结果的窄屏结构断言：状态、延迟、模型名和操作允许换行；长模型名不被 `overflow-x-hidden` 静默裁切；监控 header 使用统一 surface；分页和弹窗继承 admin appearance。

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/ChannelsView.spec.ts src/views/admin/__tests__/ChannelMonitorView.checkModeBadge.spec.ts`

Expected: FAIL，因为渠道监控 header 仍是大圆角白卡，长字段和 tooltip 在移动端不可稳定访问。

**Step 3: Write minimal implementation**

将渠道和监控外层改为无额外套卡的 workspace surface，统一状态点、延迟展示、空/加载态和表格分隔线；长模型名使用换行/详情展开，关键说明使用点击或键盘可访问的入口；不改测试请求、重试、批量操作、延迟计算和模型字段。

**Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/ChannelsView.spec.ts src/views/admin/__tests__/ChannelMonitorView.checkModeBadge.spec.ts`

Expected: PASS。

**Step 5: Commit**

```bash
git add frontend/src/views/admin/ChannelsView.vue frontend/src/views/admin/ChannelMonitorView.vue frontend/src/views/admin/__tests__/ChannelsView.spec.ts frontend/src/views/admin/__tests__/ChannelMonitorView.checkModeBadge.spec.ts
git commit -m "style: align admin channel workspace surfaces"
```

### Task 7: 完成第一阶段的移动端和全量验证

**Files:**
- Modify: affected component/page tests only when verification exposes a regression
- Reference: `frontend/src/styles/admin-workspace.css`, `frontend/src/styles/workspace-tokens.css`

**Step 1: Write the failing test**

补充一个集中的响应式回归测试，覆盖 320px、375px、390px、640px 下分页、dialog footer、toolbar、统计卡和长文本容器不产生横向溢出，并覆盖深色模式和 reduced-motion class。

**Step 2: Run test to verify it fails**

Run: `cd frontend && pnpm vitest run src/components/admin src/components/common src/views/admin`

Expected: FAIL，直到所有第一阶段页面都满足新的响应式契约。

**Step 3: Write minimal implementation**

只修复验证发现的布局问题：允许网格降为一列、让 footer 纵向排列、为详情文本增加换行或展开、补齐焦点和 reduced-motion 样式。不要借此扩大范围到用户端或低频管理员页面。

**Step 4: Run test to verify it passes**

Run: `cd frontend && pnpm vitest run src/components/admin src/components/common src/views/admin`

Expected: PASS。

**Step 5: Run broader checks**

Run: `cd frontend && pnpm run typecheck && pnpm run lint:check && pnpm run build`

Expected: typecheck、lint 和生产构建全部 PASS。

**Step 6: Commit**

```bash
git add frontend/src/components frontend/src/views/admin frontend/src/styles
git commit -m "style: complete admin UI consistency phase one"
```

执行完成后，再基于第一阶段验证结果为用户端仪表盘、使用记录、渠道状态和支付页面建立第二阶段计划，不在本阶段批量替换用户端旧样式。
