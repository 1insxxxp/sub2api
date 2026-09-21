# 运维监控时间范围记忆设计

## 目标

运维监控页面在用户重新进入或刷新后恢复上一次选择的顶部主时间范围；自定义范围同时恢复起止时间。现有 URL 参数仍可用于深链接并优先于本地记忆。

## 方案

在 `frontend/src/views/admin/ops/OpsDashboard.vue` 中增加版本化的 `localStorage` 快照。组件初始化时按 URL `tr`、本地快照、默认 `1h` 的顺序恢复主范围。已有路由查询同步逻辑不变，因此带 `tr` 的分享链接覆盖本地记忆；本地无效或损坏快照被忽略。

快照保存 `timeRange`、`customStartTime`、`customEndTime`。普通预设范围清理旧的自定义时间；自定义范围仅在起止时间均为合法 ISO 日期且顺序正确时恢复。读取和写入均捕获存储异常，保证隐私模式或存储配额错误不会阻断监控页面。

告警事件、OpenAI Token 统计等子卡片的独立时间范围不在本次范围内。

## 数据流

1. `applyRouteQueryToState` 解析路由；无有效 `tr` 时从本地快照恢复。
2. `onCustomTimeRangeChange` 更新自定义起止时间，随后 `onTimeRangeChange('custom')` 保存完整快照。
3. 顶部范围、平台、分组、查询模式的现有 watch 继续触发数据刷新和 URL 同步；时间范围 watch 额外持久化快照。
4. 组件卸载后本地快照保留，下一次进入页面恢复。

## 容错与兼容

- 只接受 `5m`、`30m`、`1h`、`6h`、`24h`、`custom`。
- 自定义时间必须能解析为日期，且开始时间早于结束时间。
- JSON 解析失败、字段不完整或 `localStorage` 抛错时静默回退默认值。
- 不改变接口参数、权限、后台设置或子卡片行为。

## 验证

新增 `OpsDashboard.timeRangePersistence.spec.ts`，覆盖默认值、本地恢复、URL 优先、自定义范围恢复和非法快照回退；同时运行现有 `OpsDashboardAutoRefresh.spec.ts` 及相关前端测试。
