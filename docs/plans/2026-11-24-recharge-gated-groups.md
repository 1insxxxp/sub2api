# 福利分组充值门槛 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在分组创建和编辑中支持“充值用户专享”门槛，按用户累计已完成充值金额控制福利分组的可见、绑定和使用权限。

**Architecture:** 在 `groups` 表增加可选的累计充值门槛字段，并将资格判断集中到现有 API Key 分组授权链路。管理员接口和前端表单只负责配置，模型广场、可用分组、API Key 绑定及请求前校验复用同一条资格规则。门槛为 0 时保持现有行为，手动授权、订阅资格和充值门槛同时满足时才允许使用。

**Tech Stack:** Go, Ent, PostgreSQL migrations, Vue 3 + TypeScript, existing admin group APIs and service tests.

---

## 1. 数据模型和迁移

**Files**

- Modify `backend/ent/schema/group.go`
- Modify generated Ent group files under `backend/ent/group/` and `backend/ent/mutation.go` as generated
- Add `backend/migrations/268_add_group_min_total_recharge.sql`
- Modify repository group mapping and persistence code under `backend/internal/repository/`

**Work**

- Add `min_total_recharge_usd` as a non-negative decimal-compatible field with default `0`.
- Add a forward-only migration with a safe default so existing groups remain unrestricted.
- Map the field in database-to-domain and create/update persistence paths.
- Regenerate Ent output using the repository’s existing generation command. Do not stage unrelated generated duplicate files.

**Tests**

- Add or extend repository/schema coverage to verify the default is zero and configured values round-trip.
- Run the focused repository/service package tests.

## 2. Admin API and domain inputs

**Files**

- Modify `backend/internal/service/admin_group.go`
- Modify `backend/internal/handler/admin/group_handler.go`
- Modify `frontend/src/types/index.ts`
- Update relevant admin group API typing if required
- Update `frontend/src/i18n/locales/zh/adminGroups.ts` and `frontend/src/i18n/locales/en/adminGroups.ts`

**Work**

- Add `min_total_recharge_usd` to create/update input, response and validation.
- Reject negative values and normalize an omitted value to zero.
- Return the field in group details and list responses.
- Add localized labels for “充值用户专享” and the threshold amount, including the unrestricted state.

**Tests**

- Add handler/service tests for create, update, negative input rejection and omitted/default values.
- Run the focused admin group tests.

## 3. Centralized eligibility and request enforcement

**Files**

- Modify `backend/internal/service/user.go` or add a small group-eligibility helper beside it
- Modify `backend/internal/service/api_key_service.go`
- Modify the request authentication/dispatch path that resolves an API key’s group
- Modify relevant error definitions and response mapping

**Work**

- Implement one predicate that combines existing manual authorization/subscription logic with the recharge threshold.
- Use `user.total_recharged >= group.min_total_recharge_usd`; threshold zero is unrestricted.
- Preserve existing behavior for public groups, explicit allowed groups and active subscriptions.
- Apply the same predicate when listing available groups, creating/changing an API key, validating an existing key and authorizing a request.
- Return a stable internal error code `GROUP_RECHARGE_REQUIRED` with threshold/current values. Expose a concise Chinese message to the client and avoid leaking unrelated account details.
- Ensure a user who crosses the threshold can use the group immediately after the completed recharge transaction.

**Tests**

- Add table-driven tests for threshold zero, below threshold, exact threshold, above threshold, manual authorization, subscription authorization, balance consumption and refund-independent cumulative qualification.
- Add request-level coverage proving an existing key is blocked before dispatch and allowed after qualification.
- Run focused service/handler tests.

## 4. Model plaza and group visibility

**Files**

- Modify `backend/internal/handler/model_plaza_handler.go`
- Modify any user group visibility DTO or service method that feeds the model plaza
- Add/extend visibility tests

**Work**

- Filter recharge-gated groups with the same centralized predicate.
- Keep hidden groups out of public model/group listings while preserving direct request rejection for stale keys.
- Do not treat hidden groups as deleted and do not alter manual group membership.

**Tests**

- Verify below-threshold users cannot see or bind the gated group.
- Verify qualified users see it immediately.
- Verify existing non-gated groups are unchanged.

## 5. Admin UI

**Files**

- Modify `frontend/src/views/admin/GroupsView.vue`
- Modify `frontend/src/types/index.ts`
- Modify group locale files

**Work**

- Add a checkbox in create/edit group access settings labelled “充值用户专享”.
- Show an amount input only when enabled, in the same currency unit used by cumulative recharge.
- Explain that the condition uses cumulative completed recharge and is unaffected by balance consumption or refunds.
- Reset and populate the field correctly on create/edit, submit it through existing APIs, and show a compact badge/summary in the group list.
- Support light/dark mode and mobile layout without changing existing group controls.

**Tests**

- Run frontend typecheck/build or the repository’s focused frontend check.
- Manually verify create, edit, disable, re-enable and list display in the local admin page.

## 6. Verification and integration

**Work**

- Run focused Go tests first, then the relevant frontend check.
- Run one full Go test/build/lint set only if needed after focused checks, using the repository rule of `-p 2`.
- Review `git diff` and `git status`. Stage only intended files and leave known untracked duplicate artifacts untouched.
- Do not deploy in this task unless separately requested.
- Commit implementation in logical commits after verification, then report the exact behavior and tests.

