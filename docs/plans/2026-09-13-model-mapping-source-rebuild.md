# Model Mapping Source Rebuild Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 根据右侧真实上游模型名删除统一前缀/后缀，并用清洗后的名称加统一前缀重建左侧请求模型名。

**Architecture:** 保留现有 `model_mapping` 数据结构。批量操作读取每条映射的 `to`，只对其派生新的 `from`；`to` 始终保持原值。生成前显示预览，遇到左侧来源冲突时保留原行并统计提示。

**Tech Stack:** Vue 3、TypeScript、Vitest、Vue Test Utils、vue-i18n。

---

### Task 1: Replace target cleanup helper with source rebuild helper

**Files:**
- Modify: `frontend/src/composables/useModelWhitelist.ts`
- Test: `frontend/src/composables/__tests__/useModelWhitelist.spec.ts`

**Step 1: Write the failing tests**

Cover both prefix and suffix removal, preservation of `to`, source collision handling, idempotence, and input immutability.

**Step 2: Run focused tests to verify failure**

Run: `cd frontend && pnpm vitest run src/composables/__tests__/useModelWhitelist.spec.ts`
Expected: the old helper behavior fails because it mutates `to` instead of rebuilding `from`.

**Step 3: Implement the minimal helper**

Add a helper accepting mappings, unified prefix, remove-prefix, and remove-suffix. Remove at most one matching leading/trailing segment from each non-empty `to`, derive `from = unifiedPrefix + cleanedTo`, preserve rows whose source would collide, and return immutable mappings plus changed/collision counts.

**Step 4: Run focused tests to verify success**

Run the same Vitest command; all tests pass.

**Step 5: Commit**

```bash
git add frontend/src/composables/useModelWhitelist.ts frontend/src/composables/__tests__/useModelWhitelist.spec.ts
git commit -m "fix: rebuild request names from cleaned upstream models"
```

### Task 2: Update bulk mapping UI and preview

**Files:**
- Modify: `frontend/src/components/account/ModelMappingBulkActions.vue`
- Modify: `frontend/src/i18n/locales/en/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Test: `frontend/src/components/account/__tests__/ModelMappingBulkActions.spec.ts`

**Step 1: Write the failing component test**

Set remove-prefix, remove-suffix, and unified prefix; assert preview and emitted mappings have rebuilt left names while right names remain unchanged.

**Step 2: Run the component test to verify failure**

Run: `cd frontend && pnpm vitest run src/components/account/__tests__/ModelMappingBulkActions.spec.ts`
Expected: the existing cleanup action emits changed right-side targets.

**Step 3: Implement the UI change**

Replace the direct target cleanup action with a source-rebuild action. Require a unified prefix and at least one removal rule, keep the existing “prepend prefix” action, and update labels/hints to explain the direction clearly.

**Step 4: Run component and locale tests**

Run the component test and `pnpm vitest run src/i18n/__tests__/localeKeyCompleteness.spec.ts`; all pass.

**Step 5: Commit**

```bash
git add frontend/src/components/account/ModelMappingBulkActions.vue frontend/src/components/account/__tests__/ModelMappingBulkActions.spec.ts frontend/src/i18n/locales/en/admin/accounts.ts frontend/src/i18n/locales/zh/admin/accounts.ts
git commit -m "fix: rebuild model mapping sources from cleaned names"
```

### Task 3: Full verification

**Files:** None.

**Step 1: Run checks**

```bash
cd frontend
pnpm vitest run src/composables/__tests__/useModelWhitelist.spec.ts src/components/account/__tests__/ModelMappingBulkActions.spec.ts
pnpm typecheck
pnpm lint:check
pnpm build
```

**Step 2: Confirm local server**

Run `curl -fsS -o /dev/null -w '%{http_code}\\n' http://localhost:3000/` and expect `200`.
