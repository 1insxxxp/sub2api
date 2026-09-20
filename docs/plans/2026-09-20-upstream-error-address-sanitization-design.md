# 上游错误地址脱敏设计

## 目标

阻止供应商在错误响应中携带的上游 URL、域名或地址暴露给 API 客户端、Ops 详情和错误日志，同时保留 HTTP 状态码、错误类型、`code`、`param` 以及必要的错误语义。

## 现状与问题

当前仓库已经有 `sanitizeUpstreamErrorMessage`，但它主要处理若干摘要字段；完整错误体的递归脱敏只接入了少数路径。Gemini 原生错误、通用网关的部分 400 响应、错误透传规则、搜索/直连端点和 cyber policy 分支仍可能直接写出上游 body。Ops 的 body 清理主要针对凭据字段和大小，也不会统一替换地址。

## 方案

采用统一错误边界脱敏：

1. 扩展共享脱敏工具，提供完整错误 body 的公共入口。JSON 递归处理所有字符串字段；纯文本按同一规则处理。
2. 在错误响应写出前，对原始 body 生成脱敏副本。错误透传规则仍使用原始 body 匹配规则，但返回消息和 body 使用脱敏结果。
3. 在 Ops 入队、持久化以及可选上游错误日志之前使用相同脱敏副本，避免客户端和后台看到不同敏感内容。
4. 仅处理错误路径；成功响应、请求转发和供应商正常业务字段不改变。
5. 保留结构化诊断字段，地址只替换主机/地址部分，错误状态码和字段名不变。

### 地址规则

- `http://` 或 `https://` URL 的主机部分替换为 `[upstream-url]`，保留路径以便定位接口类别。
- 方括号包裹的域名或 IPv4 地址替换为 `[upstream]`。
- 已有的敏感查询参数替换规则继续保留。
- 非 JSON 错误文本也必须经过同一套替换。

## 主要改动位置

- `backend/internal/service/upstream_error_sanitizer.go`：共享完整 body 脱敏入口及规则。
- `backend/internal/service/error_passthrough_runtime.go`：透传规则返回消息统一脱敏。
- `backend/internal/service/gateway_upstream_response.go`、`openai_gateway_upstream_errors.go`、Gemini/图片/搜索等原始错误响应路径：写出前替换为脱敏 body。
- `backend/internal/service/ops_service.go` 及错误日志入口：存储和日志使用脱敏 body。

## 验证标准

- JSON 任意嵌套字段中的 URL、方括号地址和敏感查询参数都不再出现原值。
- 纯文本错误中的 URL 也不再出现原值。
- 原始透传路径返回脱敏后的 body，状态码和错误结构保持不变。
- 错误透传规则仍能按原始 body 命中，但最终返回消息已脱敏。
- Ops 详情、队列数据和可选日志不包含原始上游地址。
- 现有服务测试、相关 Go 测试和 `git diff --check` 通过。

## 不在本次范围

- 不修改成功响应。
- 不删除或改写供应商错误代码、参数路径、请求 ID。
- 不改变 failover、账号冷却、计费和错误分类逻辑。
