# Affiliate Promotion Tiers Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the flat affiliate rebate with configurable cumulative promotion tiers at 8%, 10%, 12%, and 15%, using invitees with at least $50 in valid real-money payments as the qualification metric.

**Architecture:** Add durable qualifying-payment totals and qualification timestamps to `user_affiliates`, maintained by idempotent repository reconciliation against authoritative payment orders. Resolve each inviter's effective rate as custom override, then automatic tier, then the 8% base setting; expose tier progress through existing user/admin APIs and Vue views. Backfill historical relationships before enabling tier resolution so established promoters retain earned progress.

**Tech Stack:** Go, Gin, raw SQL repository, PostgreSQL migrations, Ent payment entities, Vue 3, TypeScript, vue-i18n, Vitest, Vite, Docker Compose.

---

### Task 1: Add durable qualification fields and historical backfill

**Files:**
- Create: `backend/migrations/175_add_affiliate_promotion_tiers.sql`
- Create: `backend/migrations/affiliate_tiers_migration_test.go`

**Step 1: Write the failing migration contract test**

Assert migration 175:

- adds `qualifying_payment_amount DECIMAL(20,8) NOT NULL DEFAULT 0` and nullable `qualified_at` to `user_affiliates`;
- adds a partial/indexed inviter lookup for qualified relationships;
- backfills only inviter-bound users from `payment_orders`;
- includes completed balance and subscription orders;
- subtracts successful partial/full refunds without allowing a negative total;
- excludes redeem codes and all non-payment balance mutations;
- changes the existing base setting from 10 to 8 without overwriting unrelated values;
- is written with idempotent `IF NOT EXISTS` operations.

**Step 2: Run the migration test and confirm failure**

Run: `cd backend && go test ./migrations -run TestMigration175AddsAffiliatePromotionTiers -count=1`

Expected: FAIL because migration 175 does not exist.

**Step 3: Implement the migration**

Use one aggregate over `payment_orders` grouped by invitee user ID. Treat `amount` as the USD-denominated purchased value and count only authoritative completed/partially-refunded orders of type `balance` or `subscription`. Calculate net qualifying value as `GREATEST(amount - refund_amount, 0)` for partially refunded orders and zero for fully refunded orders.

Update `qualified_at` to the earliest qualifying payment time when the historical cumulative amount reaches the rollout threshold of 50. Preserve a pre-existing timestamp on repeat migration runs. Add comments explaining that runtime qualification uses the configured threshold against the durable amount.

Update `affiliate_rebate_rate` from numeric value 10 to 8 only. Insert 8 when the setting is absent; do not overwrite a non-10 administrator value.

**Step 4: Run migration tests**

Run: `cd backend && go test ./migrations -run 'TestMigration175|TestMigration174' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/migrations/175_add_affiliate_promotion_tiers.sql backend/migrations/affiliate_tiers_migration_test.go
git commit -m "feat: add affiliate tier qualification storage"
```

### Task 2: Add validated affiliate-tier settings

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/settings_view.go`
- Modify: `backend/internal/service/setting_features.go`
- Modify: `backend/internal/service/setting_parse.go`
- Modify: `backend/internal/service/setting_update.go`
- Modify: `backend/internal/handler/dto/settings.go`
- Modify: `backend/internal/handler/admin/setting_handler.go`
- Modify: `backend/internal/handler/admin/setting_handler_update.go`
- Modify: `backend/internal/handler/admin/setting_handler_audit.go`
- Modify: `backend/internal/server/api_contract_test.go`
- Test: `backend/internal/service/setting_features_test.go` or a new focused affiliate-tier settings test

**Step 1: Write failing settings tests**

Cover defaults and parsing for:

```text
affiliate_rebate_rate = 8
affiliate_qualification_amount = 50
affiliate_bronze_invitees = 3
affiliate_bronze_rate = 10
affiliate_silver_invitees = 10
affiliate_silver_rate = 12
affiliate_gold_invitees = 30
affiliate_gold_rate = 15
```

Assert updates reject negative qualification amounts, non-increasing thresholds, decreasing tier rates, and rates outside 0-100. The previous settings must remain unchanged after a rejected request.

**Step 2: Run focused tests and confirm failure**

Run: `cd backend && go test -tags unit ./internal/service ./internal/handler/admin -run 'AffiliateTier|AffiliateRebate' -count=1`

Expected: FAIL because tier settings and validation do not exist.

**Step 3: Add a typed tier configuration**

Introduce `AffiliateTierConfig` with `QualificationAmount`, base/Bronze/Silver/Gold thresholds and rates, plus helpers that return the automatic level for a qualified count. Keep level identifiers stable and language-neutral: `standard`, `bronze`, `silver`, `gold`.

Change `AffiliateRebateRateDefault` from 20 to 8. Parse all new keys through `SettingService`, return safe defaults for missing/malformed values, and validate the complete configuration before persisting any settings.

**Step 4: Wire DTOs, auditing, and contracts**

Expose the settings through administrator GET/PUT contracts. Add each changed field to setting audit output. Update API contract fixtures to expect the new fields and the 8% default.

**Step 5: Run focused tests**

Run: `cd backend && go test -tags unit ./internal/service ./internal/handler/admin ./internal/server -run 'AffiliateTier|SettingsContract|AffiliateRebate' -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/internal/service backend/internal/handler backend/internal/server/api_contract_test.go
git commit -m "feat: configure affiliate promotion tiers"
```

### Task 3: Reconcile qualification and resolve automatic levels

**Files:**
- Modify: `backend/internal/service/affiliate_service.go`
- Modify: `backend/internal/service/affiliate_service_test.go`
- Modify: `backend/internal/repository/affiliate_repo.go`
- Modify: `backend/internal/repository/affiliate_repo_test.go`
- Modify: `backend/internal/repository/affiliate_repo_integration_test.go`
- Modify: test stubs that implement `service.AffiliateRepository`

**Step 1: Write failing service boundary tests**

Test automatic rates for qualified counts 0, 2, 3, 9, 10, 29, and 30. Assert a custom rate overrides the automatic tier and clearing it returns to the calculated tier. Assert NaN/invalid custom values safely fall back to the automatic tier, not directly to the base rate.

**Step 2: Write failing repository reconciliation tests**

Create invitees with cumulative completed amounts of 49, 50, and 70, including multiple orders and partial refunds. Assert reconciliation stores the correct net amount and qualification timestamp, is idempotent, and `CountQualifiedInvitees(inviterID, threshold)` returns the correct count under concurrent calls.

**Step 3: Extend affiliate models and repository interface**

Add qualification amount/timestamp to `AffiliateSummary` and `AffiliateInvitee`. Add repository operations equivalent to:

```go
ReconcileInviteeQualification(ctx context.Context, inviteeUserID int64, threshold float64) (*AffiliateQualification, error)
CountQualifiedInvitees(ctx context.Context, inviterID int64, threshold float64) (int, error)
```

Use one authoritative SQL aggregate over payment orders. Lock/update only the invitee affiliate row, and update `qualified_at` only on an actual below-to-at/above threshold transition. Clear it when corrected net value falls below the current threshold.

**Step 4: Resolve tier and effective rates**

Add a service helper returning automatic level, automatic rate, qualified count, next threshold, and remaining count. Resolve effective rates in this order: finite administrator override, automatic level rate, base rate.

Do not use `aff_count` for levels; it is total registrations, not qualified invitees.

**Step 5: Run service and repository tests**

Run: `cd backend && go test -tags unit ./internal/service ./internal/repository -run 'Affiliate.*(Tier|Qualification|Rate)' -count=1`

Run when the integration database is available: `cd backend && go test -tags integration ./internal/repository -run 'AffiliateRepository.*Qualification' -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/internal/service/affiliate_service.go backend/internal/service/affiliate_service_test.go backend/internal/repository/affiliate_repo.go backend/internal/repository/affiliate_repo_test.go backend/internal/repository/affiliate_repo_integration_test.go
git commit -m "feat: calculate affiliate promotion levels"
```

### Task 4: Update qualification after payments and refunds

**Files:**
- Modify: `backend/internal/service/payment_fulfillment.go`
- Modify: `backend/internal/service/payment_fulfillment_test.go`
- Modify: `backend/internal/service/payment_refund.go`
- Modify: `backend/internal/service/payment_refund_test.go`

**Step 1: Write failing payment-flow tests**

Cover an invitee crossing $50 with one order and with multiple orders. Assert callback retries do not duplicate qualification. Assert the threshold-crossing order uses the inviter's pre-upgrade rate, while the next eligible order uses the upgraded rate.

Cover partial/full refund completion: reconciliation reduces the qualifying amount, removes qualification below $50, and recalculates the inviter level. A pending or failed refund must not change qualification.

**Step 2: Run focused tests and confirm failure**

Run: `cd backend && go test -tags unit ./internal/service -run 'Affiliate.*(Fulfillment|Qualification|Refund|Upgrade)' -count=1`

Expected: FAIL because payment/refund paths do not reconcile qualification.

**Step 3: Integrate successful payment reconciliation**

Keep rebate accrual for the current order inside its existing idempotent transaction. After that transaction commits, reconcile the invitee's qualifying total so a newly reached level affects only subsequent orders. Log reconciliation failure with order/user identifiers and leave the existing rebate result intact; the next reconciliation or historical repair operation must recover the state.

Before resolving a later order's rate, reconcile previously completed payments when stored qualification is stale, excluding the currently settling order if necessary to preserve the boundary rule.

**Step 4: Integrate successful refund reconciliation**

After `markRefundOk` persists a partial/full refund, reconcile the order user's qualification. Do not reconcile for requested, pending, failed, or rolled-back refunds. This feature changes promotion-level qualification only; it does not introduce retroactive affiliate-ledger clawbacks.

**Step 5: Run payment tests**

Run: `cd backend && go test -tags unit ./internal/service -run 'Affiliate|Refund' -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/internal/service/payment_fulfillment.go backend/internal/service/payment_fulfillment_test.go backend/internal/service/payment_refund.go backend/internal/service/payment_refund_test.go
git commit -m "feat: reconcile affiliate levels from payments"
```

### Task 5: Expose promotion progress in user and administrator APIs

**Files:**
- Modify: `backend/internal/service/affiliate_service.go`
- Modify: `backend/internal/repository/affiliate_repo.go`
- Modify: `backend/internal/handler/user_handler.go`
- Modify: `backend/internal/handler/admin/affiliate_handler.go`
- Modify: `backend/internal/server/api_contract_test.go`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/affiliates.ts`

**Step 1: Write failing API contract tests**

Assert `/user/aff` returns automatic level/rate, effective rate, custom-rate flag, qualified count, next threshold, remaining count, tier definitions, and per-invitee qualifying amount/status.

Assert administrator invite records and user overview return total invite count, qualified count, automatic level/rate, custom override, and effective rate as separate fields.

**Step 2: Extend service response models and SQL projections**

Add the new fields without renaming or removing existing fields. Preserve masked invitee email behavior. Compute qualification status against the current configured threshold so changing the threshold is reflected immediately.

**Step 3: Extend TypeScript contracts**

Add a shared `AffiliateTier` union and tier-definition/progress interfaces. Keep all existing API fields for backward compatibility.

**Step 4: Run contract tests**

Run: `cd backend && go test -tags unit ./internal/handler ./internal/handler/admin ./internal/server -run 'Affiliate|APIContract' -count=1`

Run: `cd frontend && pnpm typecheck`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/affiliate_service.go backend/internal/repository/affiliate_repo.go backend/internal/handler frontend/src/types/index.ts frontend/src/api/admin/affiliates.ts
git commit -m "feat: expose affiliate tier progress"
```

### Task 6: Build the responsive user promotion-level experience

**Files:**
- Modify: `frontend/src/views/user/AffiliateView.vue`
- Create: `frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`

**Step 1: Write failing component tests**

Assert the page shows current level, effective rate, qualified count, next-level progress, all configured levels, and invitee `$35 / $50` progress. Assert a custom rate is labeled without hiding automatic level progress and Gold shows a completed/max-level state.

Add mobile assertions that the level presentation and long invitee values do not force horizontal page overflow at 320px.

**Step 2: Implement the level summary and progress**

Use compact existing card styles with at most 8px radius. Render four level entries as an unframed responsive band or compact grid, not nested cards. Use a progress bar for the next level and status text for qualified/in-progress invitees.

Replace the fixed `min-w-[560px]` invitee table on mobile with a responsive representation consistent with the project's existing `DataTable`/mobile patterns. Preserve the desktop table at wider breakpoints.

**Step 3: Add Chinese and English copy**

Describe cumulative qualification, permanent retention, the $50 cumulative-payment rule, and special-rate precedence. Do not mention the deferred first-recharge bonus or QQ trial credit.

**Step 4: Run focused tests and type checking**

Run: `cd frontend && pnpm test:run src/views/user/__tests__/AffiliateView.tiers.spec.ts`

Run: `cd frontend && pnpm typecheck`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/user/AffiliateView.vue frontend/src/views/user/__tests__/AffiliateView.tiers.spec.ts frontend/src/i18n/locales/zh/dashboard.ts frontend/src/i18n/locales/en/dashboard.ts
git commit -m "feat: show affiliate promotion progress"
```

### Task 7: Add administrator tier configuration and reporting

**Files:**
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`
- Modify: `frontend/src/views/admin/affiliates/AdminAffiliateRecordsTable.vue`
- Create: `frontend/src/views/admin/affiliates/__tests__/AdminAffiliateRecordsTable.tiers.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/en/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/overview.ts`
- Modify: `frontend/src/i18n/locales/en/admin/overview.ts`

**Step 1: Write failing administrator tests**

Assert the settings form loads/saves the $50 threshold and 3/10/30 thresholds with 8/10/12/15 rates. Assert invalid threshold ordering blocks submission and displays a localized error. Assert invite records display automatic level, qualified status/count, special override, and effective rate distinctly.

**Step 2: Extend settings contracts and form state**

Add all tier fields to `SystemSettings`, `UpdateSettingsRequest`, initial form values, hydration, and normalized save payload. Keep the existing affiliate enable, freeze, duration, cap, custom code, and custom rate controls intact.

**Step 3: Implement the configuration controls**

Group qualification amount and the four tier rows in the existing affiliate settings section. Use numeric inputs with clear units and inline validation. Base rate remains the existing global rebate setting, now labeled as the Standard level rate.

**Step 4: Extend administrator reporting**

Add compact level/status columns to invite records and the user overview modal. On mobile, collapse secondary fields under the primary inviter/invitee labels rather than expanding table minimum width.

**Step 5: Run focused frontend tests**

Run: `cd frontend && pnpm test:run src/views/admin/__tests__/SettingsView.spec.ts src/views/admin/affiliates/__tests__/AdminAffiliateRecordsTable.tiers.spec.ts`

Run: `cd frontend && pnpm typecheck`

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/api/admin/settings.ts frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/__tests__/SettingsView.spec.ts frontend/src/views/admin/affiliates frontend/src/i18n/locales/zh/admin frontend/src/i18n/locales/en/admin
git commit -m "feat: manage affiliate promotion tiers"
```

### Task 8: Verify backfill, behavior, and local runtime

**Files:**
- Modify only planned files if verification reveals defects.

**Step 1: Run backend verification**

Run: `cd backend && go test ./migrations -count=1`

Run: `cd backend && go test -tags unit ./internal/service ./internal/repository ./internal/handler ./internal/handler/admin ./internal/server -count=1`

Run repository integration tests when the configured PostgreSQL integration database is available.

Expected: PASS.

**Step 2: Run frontend verification**

Run: `cd frontend && pnpm test:run`

Run: `cd frontend && pnpm typecheck`

Run: `cd frontend && pnpm build`

Expected: PASS.

**Step 3: Validate migration/backfill on a database snapshot**

Before production deployment, restore a recent database snapshot into an isolated PostgreSQL database. Run migration 175 and compare, for sampled inviters:

```sql
SELECT inviter_id,
       COUNT(*) FILTER (WHERE qualifying_payment_amount >= 50) AS qualified_count
FROM user_affiliates
WHERE inviter_id IS NOT NULL
GROUP BY inviter_id;
```

Cross-check sampled invitee totals against completed balance/subscription orders net of successful refunds. Re-run the migration/reconciliation and confirm counts do not change.

**Step 4: Review the complete diff**

Run: `git diff --check && git status --short && git diff --stat`

Confirm unrelated mobile fixes remain intact and no generated or local runtime files enter the feature commits.

**Step 5: Update the local development runtime**

Keep Vite HMR available at `http://127.0.0.1:3000`. Rebuild/reinject the backend from the final source and recreate only `sub2api-dev` with `--no-deps`; do not restart PostgreSQL or Redis.

Verify:

- a historical inviter is assigned from backfilled payment totals;
- 2/3, 9/10, and 29/30 boundaries resolve to the correct rates;
- the threshold-crossing order uses the previous rate and the next order uses the new one;
- a custom administrator rate wins and clearing it restores the automatic tier;
- a completed refund can remove invalid qualification;
- 320px and 390px pages have no horizontal viewport overflow;
- desktop affiliate and settings pages remain aligned.

**Step 6: Final feature commit if verification required fixes**

```bash
git add <only affiliate-tier files changed during verification>
git commit -m "fix: complete affiliate tier verification"
```
