# 分类默认分组

目标：为 Anthropic、OpenAI、Gemini、Grok、国产模型和其他六类提供一个由超级管理员配置的默认入口，降低新用户选择成本。生图和全部不新增独立默认入口。

## 已确认规则

- 使用真实 group_id；来源仅限同分类普通余额组，包括普通 composite，不允许嵌套默认入口或系统订阅容器。
- 来源公开调用名经 trim/casefold 后必须跨组唯一；同一组多个账号同名正常合并。不同公开名允许相同上游模型。
- 管理员预览并确认来源及模型发布清单，模型新增或公开映射修改不自动发布。删除、停用、改分类、撤销权限立即停止受影响调用；其余模型继续可用。
- 运行时只使用已发布且当前仍有效、唯一、对调用用户开放的模型。未发布新增名也参与重名判断。目录读取失败不得回退默认平台目录。
- 保留真实来源价格、用户倍率、高峰倍率、输入优惠、RPM、模型白名单与账号排除；解析来源必须先于冻结计费上下文。保留原密钥身份和额度，禁止跨来源回退。
- 新建密钥进入分类自动选默认项，异步刷新不覆盖手选；已有密钥、编辑和快速切换不自动改变绑定。
- 常规文本 HTTP、三协议流式/非流式、Gemini 原生生成、计数、模型列表与 Codex HTTP 为首版范围；特殊会话、实时、异步媒体使用具体分组。

## 管理接口

- GET /api/v1/admin/category-default-groups
- GET /api/v1/admin/category-default-groups/:category/candidates
- POST /api/v1/admin/category-default-groups/:category/sync-preview：source_group_ids
- POST /api/v1/admin/category-default-groups/:category/sync-apply：source_group_ids、expected_revision、preview_hash
- PUT /api/v1/admin/category-default-groups/:category/status：enabled

预览返回 revision、preview_hash、added、modified、removed、unchanged、conflicts、models_count。确认发布在服务端重新计算声明与摘要，事务整体替换；并发版本或摘要变化返回409。首次成功发布启用，后续发布保持状态。

## 实现与验证

分别实现核心服务/事务仓储、网关路由/权限/审计、前端配置/密钥体验。认证缓存加入入口类型并升级版本；使用日志 group_id 保持实际来源，独立保留 category_default_group_id，只有携带新审计的 outbox 使用新版本。

关键验证覆盖六类唯一、来源资格与重名、人工同步、指纹变化、并发发布、权限撤销、source RPM/白名单、各协议、普通 composite、原价与输入优惠、幂等计费及旧记录/旧密钥兼容。只使用隔离测试数据库、用户及假上游。

用户界面沿用已确认 API 密钥页中性紧凑风格；显示可用模型数量与“按来源分组计费”，不显示容器1x。全部运行节点与共享缓存兼容升级后才启用配置；回退先关闭入口、保留绑定并明确拒绝调用。
