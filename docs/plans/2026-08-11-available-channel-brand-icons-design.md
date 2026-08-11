# 可用渠道模型图标统一设计

## 背景

可用渠道页面的平台标签已经复用 API 密钥分组的 `GroupBadge` 风格，但模型卡片左上角的 44px 大图标仍由 `AvailableChannelBrandIcon` 单独绘制。Anthropic 因而同时出现字母 A 和星芒两套标识，其他平台也存在未来样式漂移的风险。

## 目标

- 模型卡片大图标与 API 密钥分组标签使用同一套平台 Logo 和主题色。
- 覆盖 Anthropic、OpenAI、Gemini、Antigravity、Grok、Composite 和未知平台。
- 保留现有 44px 圆角方框、卡片间距和整体布局。
- 不修改模型聚合、渠道筛选、价格计算或 API 数据结构。

## 设计

### 共享视觉配置

从 `GroupBadge` 中抽取普通平台标签的主题色解析函数，作为平台颜色的唯一来源。`GroupBadge` 继续保留订阅态差异，但普通态颜色通过共享函数获取。模型大图标使用同一个普通态主题，并补充适合 44px 图标容器的边框与阴影。

### 共享平台图标

`AvailableChannelBrandIcon` 不再维护独立 SVG，改为渲染公共 `PlatformIcon`。平台名称先沿用现有品牌解析逻辑规范化，再转换为公共平台类型，因此大小写平台名和未知平台仍有稳定结果。

### 展示规则

- 容器继续保持 44px、圆角、不可收缩。
- 已知平台使用 `PlatformIcon` 的对应 Logo。
- 未知平台使用 `PlatformIcon` 的通用图标。
- 大图标的背景色和文字色与相邻普通平台标签一致。
- 不改变旁边已经统一完成的平台标签。

## 测试

- 先扩展 `AvailableChannelBrandIcon` 测试，要求所有已知平台渲染公共 `PlatformIcon`，并验证未知平台回退。
- 验证大图标和 `GroupBadge` 对每个平台取到相同的普通态主题类。
- 保留大小写规范化、44px 容器和无布局回归测试。
- 运行渠道组件全量测试、Vue 类型检查、ESLint 和生产构建。
