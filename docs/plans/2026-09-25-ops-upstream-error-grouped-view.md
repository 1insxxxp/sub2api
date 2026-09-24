# 上游错误分组定位视图 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在运维面板的上游错误明细中增加一个服务端聚合的“分组定位”弹窗，让管理员按分组 → 模型 → 上游账号 → 错误原因快速定位故障。

**Architecture:** 在现有 `OpsErrorLogFilter` 语义上新增一个数据库聚合查询和管理员 API，返回完整的分组树，不复用分页列表数据。前端在 `OpsDashboard` 级别挂载新的响应式弹窗；错误列表弹窗只负责触发入口，聚合节点通过错误 ID回到现有详情弹窗。所有原因文本沿用错误日志入库时的脱敏和截断结果。

**Tech Stack:** Go、Gin、PostgreSQL、Vue 3 `<script setup>`、TypeScript、Vitest、现有 `BaseDialog`/管理端材质样式。

---

### Task 1: 定义聚合数据契约和仓储接口

**Files:**
- Modify: `backend/internal/service/ops_models.go`
- Modify: `backend/internal/service/ops_port.go`
- Test: `backend/internal/service/ops_upstream_error_summary_test.go`

**Step 1: Write the failing test**

添加聚合契约测试，构造包含两个分组、同一模型多个账号、多个状态码和原因的摘要对象，验证 JSON 字段、错误计数、最近时间和代表错误 ID 都能表达；同时验证空结果能安全返回空数组。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/service -run TestOpsUpstreamErrorSummary -count=1`

Expected: FAIL because the summary types and repository method do not exist.

**Step 3: Write minimal implementation**

在 `ops_models.go` 增加以下服务层类型：

- `OpsUpstreamErrorSummary`：`TotalErrors`、`GroupCount`、`LatestAt`、`Groups`。
- `OpsUpstreamErrorSummaryGroup`：分组 ID/名称、错误数、模型数、账号数、最近时间、模型数组。
- `OpsUpstreamErrorSummaryModel`：模型、错误数、最近时间、状态码计数、账号数组。
- `OpsUpstreamErrorSummaryAccount`：账号 ID/名称、错误数、最近时间、最近状态码、原因数组。
- `OpsUpstreamErrorSummaryReason`：脱敏原因、错误类型、状态码、次数、最近时间、代表错误 ID。

所有时间字段使用 `time.Time` 或可空指针并设置稳定 JSON 标签；空层级使用空数组而不是 `null`。

在 `OpsRepository` 增加：

```go
GetUpstreamErrorSummary(ctx context.Context, filter *OpsErrorLogFilter) (*OpsUpstreamErrorSummary, error)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/service -run TestOpsUpstreamErrorSummary -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/ops_models.go backend/internal/service/ops_port.go backend/internal/service/ops_upstream_error_summary_test.go
git commit -m "feat(ops): define upstream error summary contract"
```

---

### Task 2: 实现服务端聚合查询

**Files:**
- Modify: `backend/internal/repository/ops_repo.go`
- Test: `backend/internal/repository/ops_upstream_error_summary_test.go`
- Modify: `backend/internal/service/ops_service.go`
- Test: `backend/internal/service/ops_upstream_error_summary_test.go`

**Step 1: Write the failing test**

为仓储查询增加测试数据，覆盖：

- 同一分组下多个模型和账号的累计计数；
- 同一账号多个状态码和原因的合并；
- 时间、平台、分组、状态码、关键词、`view` 和 `IncludeRecoveredUpstream` 筛选；
- 无分组或无账号时使用稳定的“未分组/未知账号”展示键；
- 结果按错误数降序、相同错误数按最近时间降序；
- 原因只取已存储的 `upstream_error_message`/`error_message`，不查询请求体或原始凭证。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/repository -run TestGetUpstreamErrorSummary -count=1`

Expected: FAIL because the repository method is not implemented.

**Step 3: Write minimal implementation**

在 `ops_repo.go` 增加 `GetUpstreamErrorSummary`：

1. 调用现有 `buildOpsErrorLogsWhere`，保持上游错误列表的筛选语义和参数编号。
2. 用 CTE 选出基础记录，连接 `groups` 和 `accounts`，使用稳定的 `COALESCE` 显示名称。
3. 在数据库端按分组、模型、账号、状态码和脱敏原因聚合，分别统计数量和 `MAX(created_at)`；保存一个 `MIN(id)` 或最近记录 ID 作为代表详情入口。
4. 将扁平行按排序结果组装为嵌套摘要，限制每个分组的模型、每个模型的账号、每个账号的原因数量，并提供截断标记或总数以提示存在更多原因。
5. 对原因进行长度限制，避免历史脏数据放大响应；不要对摘要重新拼接原始上游 URL。

在 `OpsService` 增加 `GetUpstreamErrorSummary`，先调用 `RequireMonitoringEnabled`，仓储为空时返回空摘要，仓储错误原样返回给 handler。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/repository -run TestGetUpstreamErrorSummary -count=1`

Expected: PASS.

Run: `go test ./internal/service -run 'TestOpsUpstreamErrorSummary|TestOpsService.*Summary' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/repository/ops_repo.go backend/internal/repository/ops_upstream_error_summary_test.go backend/internal/service/ops_service.go backend/internal/service/ops_upstream_error_summary_test.go
git commit -m "feat(ops): aggregate upstream errors by group"
```

---

### Task 3: 暴露管理员聚合接口

**Files:**
- Modify: `backend/internal/handler/admin/ops_handler.go`
- Modify: `backend/internal/server/routes/admin.go`
- Test: `backend/internal/handler/admin/ops_upstream_error_summary_test.go`

**Step 1: Write the failing test**

增加 handler 测试，验证 `GET /admin/ops/upstream-errors/summary`：

- 解析时间范围和自定义时间；
- 传递平台、分组、关键词、状态码、阶段、错误归属、视图和 resolved 筛选；
- 固定设置 `ErrorPhasesAny=["upstream","account_auth"]`、`Owner=provider`、`IncludeRecoveredUpstream=true`；
- 拒绝非法 group/status/resolved 参数；
- 监控关闭或服务不可用时返回现有错误格式。

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/admin -run TestOpsUpstreamErrorSummary -count=1`

Expected: FAIL because the route and handler do not exist.

**Step 3: Write minimal implementation**

抽取或复用现有 `ListUpstreamErrors` 的筛选解析逻辑，新增 `SummaryUpstreamErrors` handler。路由必须放在 `/upstream-errors/:id` 之前：

```go
ops.GET("/upstream-errors/summary", h.Admin.Ops.SummaryUpstreamErrors)
```

返回 `response.Success(c, summary)`，不包分页结构。保持所有输入校验和监控开关行为与现有上游错误接口一致。

**Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/admin -run TestOpsUpstreamErrorSummary -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/admin/ops_handler.go backend/internal/handler/admin/ops_upstream_error_summary_test.go backend/internal/server/routes/admin.go
git commit -m "feat(ops): expose grouped upstream error summary"
```

---

### Task 4: 增加前端 API 类型和请求封装

**Files:**
- Modify: `frontend/src/api/admin/ops.ts`
- Test: `frontend/src/api/admin/__tests__/opsApi.spec.ts`（若该测试文件不存在则在现有 API 测试目录创建）

**Step 1: Write the failing test**

Mock `apiClient.get`，调用 `opsAPI.getUpstreamErrorSummary`，验证请求路径为 `/admin/ops/upstream-errors/summary`，并且完整透传当前错误筛选参数。

**Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/api/admin/__tests__/opsApi.spec.ts -t "upstream error summary"`

Expected: FAIL because the API method and types do not exist.

**Step 3: Write minimal implementation**

在 `ops.ts` 增加与后端对应的 TypeScript 类型和 `getUpstreamErrorSummary(params)` 方法，复用 `OpsErrorListQueryParams` 的时间、平台、分组、状态码、筛选和分页无关参数。原因和文本字段标注为可选，兼容旧服务端空数组响应。

**Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run src/api/admin/__tests__/opsApi.spec.ts -t "upstream error summary"`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/ops.ts frontend/src/api/admin/__tests__/opsApi.spec.ts
git commit -m "feat(ops): add grouped upstream error API"
```

---

### Task 5: 创建分组定位弹窗并接入运维错误列表

**Files:**
- Create: `frontend/src/views/admin/ops/components/OpsUpstreamErrorSummaryModal.vue`
- Modify: `frontend/src/views/admin/ops/components/OpsErrorDetailsModal.vue`
- Modify: `frontend/src/views/admin/ops/OpsDashboard.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/ops.ts`
- Modify: `frontend/src/i18n/locales/en/admin/ops.ts`
- Test: `frontend/src/views/admin/ops/components/__tests__/OpsUpstreamErrorSummaryModal.spec.ts`
- Test: `frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts`

**Step 1: Write the failing test**

组件测试覆盖：

- 上游错误列表弹窗显示“分组定位”按钮，普通请求错误弹窗不显示；
- 点击按钮向父级发出打开摘要事件；
- 摘要弹窗加载并展示分组 → 模型 → 账号 → 原因层级；
- 默认展开分组，模型/账号按后端顺序显示，点击折叠不会丢失数据；
- 点击代表错误 ID 发出 `openErrorDetail` 并关闭摘要弹窗；
- 加载、空结果、接口错误状态均有可读提示；
- 使用长账号名、模型名和错误原因时不产生固定宽度溢出。

**Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/views/admin/ops/components/__tests__/OpsUpstreamErrorSummaryModal.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts`

Expected: FAIL because the component and event/API wiring do not exist.

**Step 3: Write minimal implementation**

实现 `OpsUpstreamErrorSummaryModal.vue`：

- Props 接收 `show`、当前时间范围/自定义时间、平台、分组和原列表筛选值；
- 打开时调用 `opsAPI.getUpstreamErrorSummary`，筛选变化时重新加载；
- 使用 `BaseDialog` 和现有管理端材质类；
- 分组、模型、账号使用原生按钮控制 `Set`/响应式展开状态；
- 错误原因显示计数、状态码、最近时间和代表错误按钮；
- 提供刷新、关闭和重试；
- 桌面端使用多列信息行，移动端使用单列、`break-words`/`min-w-0` 和可折叠卡片。

在 `OpsErrorDetailsModal.vue` 的工具栏加入按钮，仅 `errorType === 'upstream'` 时渲染，发出 `openGroupedSummary`。在 `OpsDashboard.vue`：

- 保存摘要弹窗状态；
- 与原错误列表/详情弹窗互斥；
- 传递同一套筛选条件；
- 摘要节点点击后关闭摘要并打开 `OpsErrorDetailModal`。

补齐中英文 locale：按钮、标题、计数、层级名称、空状态、加载失败、刷新和“查看详情”等文案。

**Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run src/views/admin/ops/components/__tests__/OpsUpstreamErrorSummaryModal.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/ops/components/OpsUpstreamErrorSummaryModal.vue frontend/src/views/admin/ops/components/OpsErrorDetailsModal.vue frontend/src/views/admin/ops/OpsDashboard.vue frontend/src/i18n/locales/zh/admin/ops.ts frontend/src/i18n/locales/en/admin/ops.ts frontend/src/views/admin/ops/components/__tests__
git commit -m "feat(ops): add grouped upstream error dialog"
```

---

### Task 6: 完成回归验证

**Files:**
- Modify: none

**Step 1: Run focused backend tests**

Run: `go test ./internal/service/... ./internal/repository/... ./internal/handler/admin/...`

Expected: PASS.

**Step 2: Run focused frontend tests**

Run: `pnpm exec vitest run src/views/admin/ops/components/__tests__/OpsUpstreamErrorSummaryModal.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts`

Expected: PASS.

**Step 3: Run frontend production build**

Run: `pnpm run build`

Expected: PASS; only existing Browserslist/dynamic-import warnings may remain.

**Step 4: Check repository state**

Run: `git diff --check && git status --short --branch`

Expected: no whitespace errors and a clean working tree.

**Step 5: Commit verification notes if needed**

Do not create a version bump or database migration unless the implementation reveals a schema requirement; this feature reads existing sanitized error-log fields.
