# Recharge Promotion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add one scheduled, site-wide balance recharge promotion with a replacement multiplier and an administrator-managed user blacklist.

**Architecture:** Store the single promotion as one validated JSON setting. Resolve the effective multiplier in the backend for both checkout preview and order creation, with order creation as the authoritative check. Extend the existing payment settings form and user checkout view without changing payment provider or fee calculations.

**Tech Stack:** Go/Gin/Ent service layer, settings KV repository, Vue 3 + TypeScript + Vue Test Utils, Vitest, existing admin user search selector.

---

### Task 1: Add backend promotion value object and validation tests

**Files:**
- Create: `backend/internal/service/payment_promotion.go`
- Test: `backend/internal/service/payment_promotion_test.go`

**Step 1: Write the failing tests**

Cover JSON parsing defaults, positive multiplier validation, RFC3339 start/end ordering, `[start,end)` boundaries, duplicate/invalid blacklist IDs, and replacement-vs-fallback resolution.

**Step 2: Run the focused test**

Run: `go test ./backend/internal/service -run 'Test.*RechargePromotion' -count=1`

Expected: FAIL because the promotion types and resolver do not exist.

**Step 3: Implement the minimal value object**

Add a `BalanceRechargePromotion` type, safe parser/validator, active-window check, blacklist lookup, and a resolver that returns either the promotion multiplier or the existing tier/global multiplier. Invalid or missing JSON must resolve to disabled.

**Step 4: Run the focused test**

Run: `go test ./backend/internal/service -run 'Test.*RechargePromotion' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/payment_promotion.go backend/internal/service/payment_promotion_test.go
git commit -m "feat: add recharge promotion eligibility rules"
```

### Task 2: Persist promotion in payment configuration

**Files:**
- Modify: `backend/internal/service/payment_config_service.go`
- Modify: `backend/internal/handler/admin/setting_handler.go`
- Modify: `backend/internal/handler/admin/setting_handler_update.go`
- Modify: `backend/internal/handler/payment_handler.go`
- Modify: `backend/internal/service/payment_config_service_test.go`
- Test: `backend/internal/handler/payment_handler_test.go` (or the existing payment handler contract test file)

**Step 1: Write failing configuration tests**

Verify the JSON setting round-trips through `GetPaymentConfig`/`UpdatePaymentConfig`, invalid values are rejected, and admin settings responses include the full promotion while user checkout responses expose only the safe effective promotion fields.

**Step 2: Run focused tests**

Run: `go test ./backend/internal/service ./backend/internal/handler -run 'Test.*Payment.*Promotion|Test.*RechargePromotion' -count=1`

Expected: FAIL because the setting key, request fields, and response mapping are absent.

**Step 3: Implement configuration plumbing**

Add one `BALANCE_RECHARGE_PROMOTION` key, include it in payment config parsing and update validation, and map it through the integrated admin settings endpoint. Keep blacklist IDs out of user-facing checkout responses. Use the authenticated subject in `GetCheckoutInfo` to compute the safe effective multiplier and activity metadata.

**Step 4: Run focused tests**

Run: `go test ./backend/internal/service ./backend/internal/handler -run 'Test.*Payment.*Promotion|Test.*RechargePromotion' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/payment_config_service.go backend/internal/handler/admin/setting_handler.go backend/internal/handler/admin/setting_handler_update.go backend/internal/handler/payment_handler.go backend/internal/service/payment_config_service_test.go backend/internal/handler
git commit -m "feat: expose recharge promotion settings"
```

### Task 3: Apply the promotion authoritatively when creating orders

**Files:**
- Modify: `backend/internal/service/payment_order.go`
- Test: `backend/internal/service/payment_order_promotion_test.go`

**Step 1: Write failing order tests**

Verify an eligible user receives the promotion multiplier in `PaymentOrder.Amount`, the paid amount and fee calculation remain based on the original recharge amount, and a blacklisted/out-of-window user receives the existing tier/global result.

**Step 2: Run the focused test**

Run: `go test ./backend/internal/service -run 'Test.*Order.*Promotion|Test.*RechargePromotionOrder' -count=1`

Expected: FAIL because order creation currently selects only the normal multiplier.

**Step 3: Implement authoritative resolution**

Load the user before calculating the credited balance, call the promotion resolver with the user ID and current time, and use the resulting multiplier only for balance order `Amount`. Preserve `limitAmount`, `payAmount`, daily limits, and provider selection behavior.

**Step 4: Run focused tests**

Run: `go test ./backend/internal/service -run 'Test.*Order.*Promotion|Test.*RechargePromotionOrder' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/payment_order.go backend/internal/service/payment_order_promotion_test.go
git commit -m "feat: apply recharge promotion during order creation"
```

### Task 4: Add administrator payment settings UI

**Files:**
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`
- Reuse: `frontend/src/views/admin/settings/OpenAIFastPolicyUserSelector.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/en/admin/settings.ts`

**Step 1: Write failing UI tests**

Cover loading the promotion fields, editing schedule/multiplier, adding/removing blacklist users, and serializing the payload on save.

**Step 2: Run the focused test**

Run: `cd frontend && npx vitest run src/views/admin/__tests__/SettingsView.spec.ts -t 'promotion'`

Expected: FAIL because the form controls and payload fields are missing.

**Step 3: Implement the settings section**

Extend the settings types and form defaults, render a compact promotion card below the existing recharge tiers editor, use native datetime-local inputs converted to RFC3339, and reuse the existing searchable user selector for blacklist IDs. Add Chinese and English labels/help text.

**Step 4: Run the focused test**

Run: `cd frontend && npx vitest run src/views/admin/__tests__/SettingsView.spec.ts -t 'promotion'`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/settings.ts frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/__tests__/SettingsView.spec.ts frontend/src/views/admin/settings/OpenAIFastPolicyUserSelector.vue frontend/src/i18n/locales/zh/admin/settings.ts frontend/src/i18n/locales/en/admin/settings.ts
git commit -m "feat: add recharge promotion admin settings"
```

### Task 5: Show the effective promotion on the user recharge page

**Files:**
- Modify: `frontend/src/types/payment.ts`
- Modify: `frontend/src/views/user/PaymentView.vue`
- Modify: `frontend/src/views/user/__tests__/PaymentView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/misc.ts`
- Modify: `frontend/src/i18n/locales/en/misc.ts`

**Step 1: Write failing UI tests**

Cover rendering the activity label/end time and using the effective activity multiplier in the credited preview, plus the normal preview when the backend says the activity is inactive.

**Step 2: Run the focused test**

Run: `cd frontend && npx vitest run src/views/user/__tests__/PaymentView.spec.ts -t 'promotion'`

Expected: FAIL because checkout types and activity display are missing.

**Step 3: Implement safe checkout display**

Add optional safe promotion fields to `CheckoutInfoResponse`, use the backend-provided effective multiplier/tier data for previews, and render a small activity notice without exposing blacklist information.

**Step 4: Run the focused test**

Run: `cd frontend && npx vitest run src/views/user/__tests__/PaymentView.spec.ts -t 'promotion'`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/types/payment.ts frontend/src/views/user/PaymentView.vue frontend/src/views/user/__tests__/PaymentView.spec.ts frontend/src/i18n/locales/zh/misc.ts frontend/src/i18n/locales/en/misc.ts
git commit -m "feat: show effective recharge promotion"
```

### Task 6: Run regression checks and review the diff

**Files:**
- Modify only if tests reveal regressions.

**Step 1: Run backend tests**

Run: `go test ./backend/internal/service ./backend/internal/handler ./backend/internal/server`

**Step 2: Run frontend tests and type checks**

Run: `cd frontend && npx vitest run src/views/admin/__tests__/SettingsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts && npm run type-check`

**Step 3: Run the production build**

Run: `cd frontend && npm run build`

**Step 4: Inspect the final diff**

Run: `git diff --check && git status --short && git log -6 --oneline`

Expected: no whitespace errors, all focused/regression checks pass, and only the promotion feature files are changed.

**Step 5: Commit any test-only fixes**

```bash
git add <verified-fix-files>
git commit -m "test: stabilize recharge promotion coverage"
```
