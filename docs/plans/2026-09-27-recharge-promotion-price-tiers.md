# Recharge Promotion Price Tiers Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在不改变充值额度、历史订单和现有倍率活动兼容性的前提下，为限时充值优惠增加按档位设置实际支付价格的能力。例如基础档位为“支付 10 元到账 50 额度”，活动期间可配置为“支付 8 元到账 50 额度”。

**Architecture:** 复用现有 `BalanceRechargePromotion` 配置，在活动 JSON 中增加可选的 `price_tiers` 数组。每个活动价格档位以基础档位的到账额度为键，记录新的实际支付价格。后端在创建订单时以用户、时间和黑名单为准决定是否命中活动价格，并继续将到账额度写入订单的 `Amount`、实际支付金额写入 `PayAmount`。前端同时保留基础档位和活动价格映射，由后端返回用户有资格看到的活动配置；旧的全局 `multiplier` 仍作为没有价格覆盖时的兼容回退。

**Tech Stack:** Go、Gin/现有支付服务与 PostgreSQL 持久化、Vue 3、TypeScript、Vitest。

---

## Task 1: Add backend tests for promotion price tiers

**Files:**
- Modify: `backend/internal/service/payment_promotion_test.go`
- Modify: `backend/internal/service/payment_amounts_test.go`
- Modify: `backend/internal/service/payment_order_promotion_test.go`
- Modify: `backend/internal/service/payment_config_service_test.go`
- Modify: `backend/internal/handler/payment_handler_promotion_test.go`

**Steps:**
1. 先为 `price_tiers` 的 JSON 解析、禁用活动、未开始/已结束活动、黑名单用户和价格档位校验补测试。
2. 添加活动价格命中测试：基础档位支付 10 元到账 50，活动价支付 8 元仍到账 50；不命中活动价格时保留基础档位；旧倍率活动继续生效。
3. 添加边界测试：重复到账额度、重复活动价格、非正数、未知到账额度、活动价格与其他基础支付档位冲突均被拒绝。
4. 添加订单测试，确认 `PaymentOrder.Amount` 是到账额度，`PayAmount` 是活动实际支付金额，退款/订单历史所需字段不变。
5. 添加 checkout 响应测试，确认只返回当前用户有资格使用的价格档位，不泄露黑名单用户列表。
6. 运行相关测试，确认新测试在实现前按预期失败：
   ```bash
   cd /Users/alien/sub2-local/backend
   go test ./internal/service ./internal/handler -run 'RechargePromotion|PaymentConfig|Checkout'
   ```

## Task 2: Extend promotion model, parsing, and configuration validation

**Files:**
- Modify: `backend/internal/service/payment_promotion.go`
- Modify: `backend/internal/service/payment_amounts.go`
- Modify: `backend/internal/service/payment_config_service.go`
- Modify: `backend/internal/service/payment_config_service_test.go`

**Steps:**
1. 增加 `BalanceRechargePromotionPriceTier`，字段为 `CreditedAmount` 和 `Price`，并将 `PriceTiers` 加入 `BalanceRechargePromotion`。
2. 更新活动校验：启用活动时允许“有效倍率”或“至少一个有效价格档位”二选一；价格和到账额度必须为有限正数，到账额度和活动价格分别不能重复。
3. 增加基础档位交叉校验：活动到账额度必须对应唯一基础档位的到账额度；活动价格不能与其他基础档位的支付价格产生歧义，允许覆盖自身基础价格。
4. 保持旧配置 JSON 可解析：没有 `price_tiers` 的活动行为完全不变；无效的新字段不能让结算绕过基础档位校验。
5. 在配置更新和读取路径中使用同一套校验，兼容只更新活动配置或只更新基础档位配置的 patch 请求，并避免保存无法结算的交叉引用。
6. 运行配置与解析测试，确认旧倍率活动和新价格档位活动都能通过。

## Task 3: Apply promotional prices in order creation and checkout

**Files:**
- Modify: `backend/internal/service/payment_order.go`
- Modify: `backend/internal/handler/payment_handler.go`
- Modify: `backend/internal/service/payment_amounts.go`
- Modify: `backend/internal/service/payment_order_promotion_test.go`
- Modify: `backend/internal/handler/payment_handler_promotion_test.go`

**Steps:**
1. 增加按用户和当前时间查找活动价格档位的解析函数，顺序为：活动价格命中、旧倍率活动回退、基础档位倍率、固定回退。
2. 将活动价格作为合法的订单支付金额参与 preset 校验；用户不符合活动时间或黑名单条件时，8 元等活动价格必须被拒绝，不能借此获得额度。
3. 创建余额订单时将活动价格映射为基础档位对应的到账额度；保持支付网关收到的支付金额为请求中的活动价格。
4. 保持订单金额、退款金额、充值到账和幂等逻辑的现有语义，不增加数据库字段和迁移。
5. checkout 安全响应增加 `price_tiers`，只返回当前用户可用的活动信息，并继续隐藏 `blacklist_user_ids`。
6. 运行服务层和 handler 测试，重点确认：活动关闭、过期、黑名单、未命中价格档位时均回到基础价格；价格档位命中时额度不变。

## Task 4: Extend frontend types and price-resolution helpers

**Files:**
- Modify: `frontend/src/types/payment.ts`
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/components/payment/rechargePromotion.ts`
- Modify: `frontend/src/components/payment/rechargeTiers.ts`
- Modify: `frontend/src/components/payment/__tests__/RechargePromotionEditor.spec.ts`
- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`

**Steps:**
1. 为管理员配置类型、用户 checkout 类型和公共活动类型增加 `price_tiers`。
2. 更新前端活动校验，使“只有活动价格档位、没有倍率”成为合法配置，同时继续校验活动时间、正数、唯一性和基础档位引用。
3. 扩展 `resolveRechargeMultiplier` 或新增固定到账额度解析 helper：价格档位命中时返回 `credited_amount / price`，以兼容现有金额计算；旧倍率行为保持不变。
4. 编写 helper 测试覆盖活动价格优先级、旧倍率回退、基础价格和无效活动配置。
5. 运行前端相关单元测试，确认类型检查和现有支付金额校验不回退。

## Task 5: Add per-tier activity price controls in admin settings

**Files:**
- Modify: `frontend/src/components/payment/RechargePromotionEditor.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/components/payment/__tests__/RechargePromotionEditor.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

**Steps:**
1. 给 `RechargePromotionEditor` 传入当前基础充值档位，按每个基础档位显示到账额度和可选的活动支付价格输入框。
2. 保留现有活动名称、开始/结束时间、旧倍率和黑名单配置；活动价格输入为空表示该档位不参加价格优惠。
3. 在 `SettingsView` 的表单加载、编辑和保存路径中完成 `price_tiers` 的规范化、序列化和回显，避免把空行保存为无效档位。
4. 明确界面文案：基础档位的到账额度不变，只填写活动期间实际支付价格；旧倍率字段仅用于兼容其他未单独设置价格的档位。
5. 遵循已有后台材质、紧凑表单和浅深色样式，不修改支付接口和权限逻辑。
6. 添加表单测试：输入 50 额度活动价 8 后保存并重新加载仍为 8；空值不会生成价格档位；校验错误会阻止保存。

## Task 6: Show activity prices in the user recharge page

**Files:**
- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/components/payment/rechargeTiers.ts`
- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`

**Steps:**
1. 根据 checkout 返回的 `price_tiers` 将基础档位展示为活动实际支付价格，同时保留到账额度展示，避免用户误以为额度被修改。
2. 快捷金额、选中金额、订单预览、支付限额校验和提交前刷新均使用活动实际支付金额。
3. 未命中活动、活动过期、黑名单用户或活动价格档位缺失时显示基础价格。
4. 保持现有活动倍率展示的兼容行为，并确保不出现“先按活动价下单、提交时被后端拒绝”的前后端口径分歧。
5. 添加测试覆盖活动期间的快捷档位、手动选择活动价、额度计算和活动失效回退。

## Task 7: Verify end to end and document the rollout

**Files:**
- Review: all files changed in Tasks 1-6
- Optional modify: `docs/plans/2026-09-27-recharge-promotion-price-tiers-design.md` if implementation decisions need clarification

**Steps:**
1. 运行后端格式化、单元测试和构建：
   ```bash
   cd /Users/alien/sub2-local/backend
   gofmt -w internal/service/payment_promotion.go internal/service/payment_amounts.go internal/service/payment_config_service.go internal/service/payment_order.go internal/handler/payment_handler.go
   go test ./...
   go build ./cmd/server
   ```
2. 运行前端格式化检查、类型检查和相关测试：
   ```bash
   cd /Users/alien/sub2-local/frontend
   npm run type-check
   npm run test -- --run src/components/payment/__tests__/RechargePromotionEditor.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/views/admin/__tests__/SettingsView.spec.ts
   npm run build
   ```
3. 本地打开充值设置，配置基础档位“到账 50、基础支付 10、活动支付 8”，验证管理员回显和用户 checkout 展示。
4. 使用非黑名单用户在活动期间创建订单，验证支付金额为 8、到账额度为 50；再用黑名单用户、活动外时间和活动关闭状态验证回退到 10 元或被拒绝。
5. 检查历史订单和退款流程，确认已有订单数据不受新配置影响。
6. 每个逻辑阶段单独提交，提交前检查只包含本任务文件，避免混入工作区已有的无关改动；推荐提交顺序：模型/校验、订单/接口、前端类型/helper、后台编辑器、用户页面、测试与文档。

