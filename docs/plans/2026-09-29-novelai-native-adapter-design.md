# NovelAI 原生协议适配层设计

日期：2026-09-29

## 目标

让使用智绘姬等 NovelAI 客户端的用户，使用 Passion API 生成 NAI Diffusion 图片。客户端发送原生 NovelAI 请求，网关负责 API Key 鉴权、分组权限、模型路由、账号调度、失败切换、计费和响应格式转换。现有 OpenAI 兼容生图接口保持不变。

## 现状与约束

- 当前网关已支持 `POST /v1/images/generations` 和 `/images/generations`。
- 当前网关没有 `POST /ai/generate-image` 原生入口。
- 现有图片调度和计费逻辑集中在 OpenAI 图片处理链中，不能绕过，否则会造成权限、账号切换和计费不一致。
- 现有 NAI 上游账号默认按 OpenAI 兼容协议处理；需要为图片账号增加协议选择，旧账号默认保持 OpenAI。
- NovelAI 原生请求需要保留完整 `parameters` 对象以及未来未知字段，不能只映射 prompt、尺寸和数量。

## 方案

新增原生入口 `POST /ai/generate-image` 和 `/v1/ai/generate-image`。入口使用现有 API Key 鉴权和分组权限，解析原生请求后交给共享的图片调度流程。共享流程根据账号的 `image_protocol` 选择上游协议：

- `openai`（默认）：继续使用现有 `/v1/images/generations` 转发。
- `novelai`：使用账号 Base URL 的 `/ai/generate-image`，发送原生 NovelAI body，并使用该账号上游密钥认证。

为避免复制完整的账号切换和使用记录逻辑，抽取图片调度中的“选择账号、获取槽位、转发、失败切换、记录使用量、释放资源”边界；OpenAI 和 NovelAI 只提供不同的请求/响应适配器。

## 请求与响应

原生入口支持：

- `input`、`model`、`action`；
- `parameters` 中的尺寸、步数、采样器、CFG/scale、种子、负面提示词、质量、参考图、图生图参数、多张生成等字段；
- 原生 JSON 和必要的图片字段；未知字段保留并原样转发；
- 仅允许 `action=generate`，不支持文本生成相关 action。

上游成功响应按 NovelAI 原生格式返回。对上游返回的 ZIP、PNG 或错误 JSON 不做无损以外的重编码；响应头、内容类型和文件名按客户端可识别格式规范化。上游错误转换为统一的网关错误，同时保留可审计的上游状态和账号信息。

## 配置

在 API Key 账号的图片协议设置中增加 NovelAI 选项。现有账号默认 `openai`，只有明确选择 `novelai` 的账号才访问原生路径。账号模型映射继续生效；外部模型名使用小写的 `nai-diffusion-*`。

分组仍需开启图片生成权限，并在模型白名单或账号映射中包含对应 NAI 模型。客户只使用 Passion API 自己生成的 API Key，不接触上游密钥。

## 错误处理与安全

- 沿用现有请求体大小限制、超时、并发限制、安全审计、失败账号排除和重试规则。
- 不记录原生请求中的 API Key、参考图内容或完整 prompt；日志只记录脱敏后的模型、路由和状态。
- 原生协议请求不能降级到文本聊天接口；模型不支持时返回明确的模型/分组错误。
- 图片生成中客户端断开时，继续使用现有图片上游脱钩策略，避免上游已计费任务被取消。

## 验证

测试覆盖：

1. 原生请求解析、默认值、非法 action、尺寸/数量/参数校验和未知字段保留。
2. `/ai/generate-image` 与 `/v1/ai/generate-image` 路由、鉴权和图片权限。
3. NovelAI 账号请求 URL、认证头和原始 body 转发。
4. ZIP/PNG/JSON 响应透传及错误映射。
5. 同名模型的账号切换、失败账号排除、使用记录和计费数据。
6. 旧 OpenAI 图片接口回归测试。

