# 分组模型别名与抗截断实现计划

> **执行约束：** 按 TDD 实施。每个任务先增加能复现需求的失败测试，确认测试失败后再实现；完成一个任务后运行该任务的定向测试和 `git diff --check`。

## 任务 1：增加分组别名领域模型和持久化字段

**目标：** 让分组可以保存别名、上游模型和抗截断策略，并通过 Ent、迁移、Service、DTO、认证快照完整往返。

**文件：**

- `backend/internal/domain/group_model_alias.go`（新增）
- `backend/internal/service/group.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/service/admin_group.go`
- `backend/internal/service/admin_group_duplicate.go`
- `backend/internal/service/api_key_auth_cache.go`
- `backend/internal/service/api_key_auth_cache_impl.go`
- `backend/ent/schema/group.go`
- `backend/ent/group.go` 及 Ent 生成文件
- `backend/internal/handler/admin/group_handler.go`
- `backend/internal/handler/dto/types.go`
- `backend/internal/handler/dto/mappers.go`
- `frontend/src/types/index.ts`
- `backend/migrations/246_group_model_aliases.sql`（按仓库当前迁移序号调整）

**步骤：**

1. 先写别名规范化、重复别名、范围和 JSON 往返测试；运行定向 Go 测试确认失败。
2. 添加 `ModelAlias`、`AntiTruncationPolicy` 和分组 `ModelAliases` 字段；在 schema 中使用 JSONB 默认空数组，生成 Ent 代码并补迁移。
3. 在 Create/Update/duplicate、认证快照和 DTO 中透传，确保旧分组空值保持兼容。
4. 为 `Group.ResolveModelAlias` 和 `Group.PublicModelIDs` 增加单元测试。

## 任务 2：接入模型解析、模型列表和使用记录

**目标：** 别名命中后使用上游模型调度，但客户端和记录保留别名；模型列表保留原模型并增加有可用目标的别名。

**文件：**

- `backend/internal/service/model_alias.go`（新增）
- `backend/internal/service/channel_service.go`
- `backend/internal/service/openai_models_list.go`
- `backend/internal/handler/openai_models_handler.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/openai_gateway_handler_responses.go`（如该路径实际存在）
- `backend/internal/service/gateway_usage_billing.go`
- `backend/internal/service/channel_service_test.go`
- `backend/internal/handler/openai_models_handler_test.go`

**步骤：**

1. 先写命中别名、未命中别名、渠道映射链和模型列表的失败测试。
2. 在认证后的公共模型解析点返回 `requested_model`、`resolved_model`、`alias_policy`，避免各协议自行猜测。
3. 将解析结果接到 Chat/Responses/Messages/Gemini 入口；先保证无别名路径字节级行为不变。
4. 扩展模型列表投影，按分组配置加入别名条目并应用公开模型白名单，不额外暴露内部目标。
5. 补使用记录字段/映射链测试，确认一次请求只有一条最终计费记录。

## 任务 3：实现 OpenAI Chat 和 Messages 兼容桥的抗截断续写

**目标：** 只对命中并开启策略的别名续写，默认关闭，保持同一账号和会话。

**文件：**

- `backend/internal/service/continuation_policy.go`（新增）
- `backend/internal/service/openai_gateway_chat_completions_raw.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/service/openai_gateway_messages_chat_fallback.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_gateway_handler.go`

**步骤：**

1. 先写流式 `finish_reason=length`、语义输出后的 EOF、正常 stop、客户端取消和工具调用的失败测试；确认未开启策略时保持现有错误语义。
2. 在流解析器中记录可安全重放的助手文本、终止原因和每轮 usage；开启策略且仍有预算时抑制中间 `length` 终止帧并返回“需要续写”状态。
3. 在同一账号槽位内构造追加助手消息和固定继续提示，重新调用同一协议路径；用总轮数、总秒数和累计输出预算保护循环。
4. 最后一轮透传正常终止事件；客户端取消、内容安全、4xx、工具调用、空响应和上下文无法重建时不续写。
5. 对 Messages→Chat 兼容桥和 Chat→Anthropic 转换路径接入续写；原生 Responses/WebSocket/原生 Anthropic、Gemini/Antigravity 先只解析别名，不改变既有流状态。
6. 增加多轮 usage 聚合测试，确保只提交一次使用记录，记录续写轮数和触发标记。

## 任务 4：增加管理端模型别名配置界面

**目标：** 管理员能在分组编辑页维护别名和抗截断参数，旧配置界面不变。

**文件：**

- `frontend/src/types/index.ts`
- `frontend/src/api/admin/groups.ts`
- `frontend/src/views/admin/GroupsView.vue`
- `frontend/src/views/admin/__tests__/GroupsView.modelAliases.spec.ts`（新增）

**步骤：**

1. 先写表单序列化和校验失败测试。
2. 在分组编辑的模型配置区域新增别名列表，支持新增/删除、上游模型、抗截断开关和预算输入。
3. 保存时沿用现有 Create/Update API，前端范围校验与后端一致；编辑回填和复制分组不丢配置。
4. 用 Vitest 做序列化、默认值、重复别名和关闭策略的回归测试。

## 任务 5：验证与交付

1. 运行后端定向测试：`go test ./backend/internal/service ./backend/internal/handler ./backend/internal/handler/dto`。
2. 运行前端定向测试和类型检查：`pnpm --dir frontend exec vitest run ...`、`pnpm --dir frontend run typecheck`。
3. 运行 `git diff --check`，必要时运行 `pnpm --dir frontend run build`。
4. 检查迁移、认证缓存版本和旧别名为空时的兼容路径。
5. 提交实现，保持版本号 `0.2.16` 不变；用户未要求时不推送、不部署。
