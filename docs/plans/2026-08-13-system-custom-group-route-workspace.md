# 系统自定义分组路由工作区实施计划

1. 在 `SystemCustomGroupDialog.spec.ts` 增加失败用例：每组全选三态、主工作区放大/高级设置折叠、跨协议提示。
2. 在 `SystemCustomGroupDialog.vue` 实现每组批量选择与独立折叠、紧凑设置区、扩大路由区和混合协议提示；补齐中英文文案。
3. 在后端 routes 测试中先加入失败的 Claude `/v1/messages` → Gemini 来源生产链测试，再用现有真实服务和可控上游完成测试夹具。
4. 运行前端目标/全量测试、类型检查、lint、build，以及后端 routes/service 相关测试、vet 和 server build。
5. 提交代码，使用新镜像只重建本地 API 容器，确认 API、PostgreSQL、Redis 健康且数据容器未变化。
