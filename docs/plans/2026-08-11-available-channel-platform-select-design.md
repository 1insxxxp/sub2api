# 可用渠道平台下拉统一样式设计

## 目标

将“可用渠道”工具栏中的平台筛选从浏览器原生 `select` 替换为项目通用 `Select.vue`，使触发器、选项浮层、选中态、暗色模式和键盘交互与站内其他下拉选择保持一致。

## 方案比较

1. **复用通用 `Select.vue`（采用）**：视觉和交互天然与全站一致，已有 Teleport、键盘、焦点、暗色模式支持，改动范围最小。
2. **继续装饰原生 `select`**：只能统一收起状态，展开后的系统菜单仍由浏览器控制，无法解决截图中的主要问题。
3. **为本页新写下拉弹层**：能完全定制，但会重复通用组件已经具备的交互和无障碍逻辑，维护成本更高。

## 组件与数据流

- 修改 `AvailableChannelsToolbar.vue`，引入通用 `Select` 与 `SelectOption`。
- 通过计算属性将空值“全部平台”和 `platforms` 数组转换为通用 Select 的选项结构。
- `modelValue` 继续绑定当前 `platform`；`update:modelValue` 继续向父级发出 `update:platform`，不改变筛选逻辑或 API。
- 选项数量很少，显式关闭搜索；保留当前平台筛选的 aria-label。
- 用局部 wrapper/deep 样式将通用触发器对齐到当前工具栏的 44px 高度、圆角和宽度，不修改全局 Select 样式。

## 响应式与无障碍

- 保持当前 grid 列布局和移动端占位关系，不引入横向滚动。
- 使用通用组件已有的 `button + listbox + option` 语义、方向键、Enter、Escape、点击外部关闭和焦点行为。
- 暗色、hover、focus-visible 由通用组件统一提供。

## 测试

- 先更新 Toolbar 组件测试，要求不存在原生 `select`，并真实挂载通用 Select。
- 验证全部平台与动态平台选项、当前选中值以及 `update:platform` 事件。
- 验证关闭搜索、保留 aria-label 和 44px 对齐样式。
- 运行可用渠道组件回归、Vue 类型检查和 ESLint。
