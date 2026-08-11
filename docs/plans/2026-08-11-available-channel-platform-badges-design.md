# 可用渠道平台标签统一设计

## 目标

可用渠道页所有小平台标签统一复用 API 密钥分组使用的 `GroupBadge` 视觉体系，使 Anthropic、OpenAI、Gemini 等平台的图标、主题色、字号和暗色模式保持一致。

## 方案比较

1. **增加可用渠道平台标签薄封装（采用）**：`AvailableChannelPlatformBadge` 内部复用 `GroupBadge`，集中处理平台类型转换和关闭倍率显示；各消费位置只传平台字符串。
2. **各位置直接使用 `GroupBadge`**：视觉一致，但会重复 `name`、`platform`、`showRate` 参数和类型转换。
3. **复制 `GroupBadge` 的颜色类**：短期改动少，但两套主题以后会再次漂移，不采用。

## 组件设计

- 新增 `AvailableChannelPlatformBadge.vue`。
- 输入仅为 `platform: string`。
- 内部渲染 `GroupBadge`：名称使用原平台文本，平台值传入相同平台，`show-rate` 固定为 `false`。
- 不复制任何平台图标或主题色，保持 `GroupBadge` 为唯一视觉来源。

## 替换范围

- 桌面渠道导航中的平台标签。
- 可用渠道页头中的平台标签。
- 模型卡片标题下的平台标签。
- 展开方案卡片中的平台标签。
- 移动端渠道选择器触发器及选项中的平台标签。
- 旧的分组列表本身已经使用 `GroupBadge`，无需修改。
- 平台筛选下拉属于表单控件，不改成标签；模型卡左侧大品牌图标继续保留。

## 数据与交互

平台字符串、筛选逻辑、渠道选择、模型聚合和计价数据均不改变。标签只负责展示，不产生事件或状态。

## 响应式与无障碍

标签保持 `inline-flex`，多平台时继续自动换行；移动端不得引入横向滚动。平台图标由 `PlatformIcon` 提供，平台名称仍以可见文本呈现。

## 测试

- 新组件测试验证其向 `GroupBadge` 传递平台名称、平台值并关闭倍率。
- 更新 Catalog、Toolbar、ModelList、OfferingCard、Picker 测试，验证所有平台标签使用统一组件。
- 验证旧灰色标签和平台字符串 join 展示不再存在。
- 运行可用渠道完整组件测试、Vue 类型检查和 ESLint。
