# 多平台模型生命周期过滤设计

## 背景

现有过滤仅覆盖 Anthropic。Gemini 等平台也存在上游继续返回 shutdown 模型、以及项目内置默认列表滞后的情况。OpenAI 的 Deprecated 模型和 xAI 的退役重定向模型仍可能正常调用，不能仅凭“Deprecated/Retired”标签一刀切。

## 目标

建立统一的账号模型生命周期过滤入口，按平台维护“已明确不可调用”的精确模型 ID。同步结果、账号映射聚合和无映射默认列表采用同一口径。

## 规则

- Anthropic：过滤官方 Retired 精确 ID，Bedrock 继续使用独立生命周期。
- Gemini：过滤 shutdown 日期已过且不再提供原模型服务的精确 ID。
- OpenAI：本轮不把 Deprecated 等同于 shutdown；没有可靠停用证据的模型保留。
- Grok：自动重定向后仍可调用的历史 slug 保留。
- Antigravity：作为独立上游，不套 Gemini 官方 API 生命周期。
- 第三方自定义模型名称不做前缀或模糊过滤。

## 数据流

统一函数接收账号和模型列表：

1. 上游模型同步返回前过滤。
2. `GatewayService.GetAvailableModels` 按账号聚合时过滤。
3. Gemini 内置默认列表移除已 shutdown 模型，并更新默认测试模型。

数据库已有映射不删除，计费与实际请求路由不修改。

## 测试

- Gemini 同步结果过滤 shutdown ID、保留有效 ID。
- Gemini 映射聚合过滤 shutdown ID。
- Antigravity 同名 ID 不受 Gemini 规则影响。
- Gemini 默认模型和默认测试模型不再包含 shutdown ID。
- Anthropic 与 Bedrock 现有边界继续通过。
