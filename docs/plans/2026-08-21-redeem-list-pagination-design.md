# 兑换码列表分页设计

## 背景

用户端兑换页里，“最近活动”和“我生成的兑换码”在数据变多后会把列表撑得很长，也会让接口一次拉取过多数据。需要给两个列表增加分页，让页面保持可用、加载更轻。

当前本地 `dev` 分支已经有“最近活动”的前后端代码；“我生成的兑换码”在本地代码中尚未出现，但已有余额转兑换码设计文档定义了 `GET /api/v1/redeem/generated`。实现时先定位或补齐该列表接口，再按同一套分页模式处理。

## 目标

- “最近活动”支持服务端分页。
- “我生成的兑换码”支持服务端分页。
- 前端复用现有 `Pagination.vue`，两个列表互不影响页码。
- 刷新、兑换成功、生成成功后重新拉取对应列表。
- 不改变兑换码生成、兑换、扣费、返佣等业务规则。

## 非目标

- 不重做兑换页视觉结构。
- 不改变管理员兑换码管理分页。
- 不新增批量生成、过期退款或审批逻辑。

## 推荐方案

采用接口分页。

### 后端

- `GET /api/v1/redeem/history` 接收 `page` 和 `page_size`。
- 使用现有 `pagination.ParsePagination` / `response.PaginatedWithResult` 风格返回分页结构。
- `RedeemService` 增加用户历史分页方法，复用已有 `RedeemCodeRepository.ListByUserPaginated`。
- `GET /api/v1/redeem/generated` 返回当前用户生成的兑换码分页列表。
- generated 列表只返回 `created_by = 当前用户` 且来源为用户余额转赠的兑换码，避免看到管理员生成的码。

分页默认值：

- 默认 `page = 1`。
- 默认 `page_size = 10`。
- 最大值沿用后端分页包限制，前端只暴露常用选项。

### 前端

- `redeemAPI.getHistory(params)` 返回 `PaginatedResponse<RedeemHistoryItem>`。
- 新增或调整 `redeemAPI.getGeneratedCodes(params)` 返回 `PaginatedResponse<RedeemCode>`。
- `RedeemView.vue` 中维护两组分页状态：
  - `historyPage/historyPageSize/historyTotal`
  - `generatedPage/generatedPageSize/generatedTotal`
- 两个列表下方各放一个 `Pagination.vue`。
- 当用户兑换成功后，刷新最近活动第一页；当用户生成兑换码成功后，刷新已生成兑换码第一页并刷新余额。

### 错误处理

- 列表加载失败时保留当前已有错误提示风格，不影响另一块列表。
- 页码超出时以后端返回为空或校正结果为准；前端在删除/变更导致当前页为空时回退到上一页。

## 测试计划

- 后端测试 `redeem/history?page=1&page_size=10` 返回分页结构和正确总数。
- 后端测试 generated 列表只返回当前用户生成的余额转赠码。
- 前端测试历史列表请求会携带页码和 page size。
- 前端测试分页切换会更新对应列表，不影响另一个列表。
- 手动运行用户兑换页，检查移动端和桌面端分页不挤压列表内容。
