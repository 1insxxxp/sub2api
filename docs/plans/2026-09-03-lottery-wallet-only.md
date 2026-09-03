# Wallet-Only Lottery Attempts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove activity-based and daily-reset lottery attempts so every draw uses attempts earned from consecutive check-ins or granted by an administrator.

**Architecture:** Preserve the existing activity columns and API fields for backward compatibility, but normalize saved/read activity policy to a zero activity quota and make the persistent attempt wallet the only authorization source. Update the service/repository aggregation, admin configuration surface, and user/admin copy while keeping wallet credit, draw idempotency, prize delivery, and historical records unchanged.

**Tech Stack:** Go service/Ent/PostgreSQL, Gin handlers, Vue 3 `<script setup>`, TypeScript, Tailwind, Vitest, pnpm.

---

### Task 1: Lock the wallet-only behavior with failing tests

**Files:**
- Modify: `backend/internal/service/lottery_service_test.go`
- Modify: `backend/internal/service/lottery_attempt_grants_test.go`
- Modify: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/views/user/__tests__/LotteryView.spec.ts`

**Step 1: Write service regression tests**

Replace daily/total summary expectations with a wallet-only summary test: any legacy activity limit/usage is ignored, activity remaining is zero, reward remaining equals the wallet balance, and the next source is always `wallet` when balance is positive. Update the admin balance query test to expect no activity-policy fields.

**Step 2: Write UI regression tests**

Assert the admin activity form no longer renders the attempt mode or attempt limit controls and saves activity without those fields. Assert the user page renders only the wallet/check-in/admin-grant attempt wording and does not render daily/total labels. Keep the existing no-weight assertion.

**Step 3: Run focused tests to verify the red state**

```bash
cd backend && go test ./internal/service -run 'LotteryAttempt|LotteryAdminAttemptBalances' -count=1
cd ../frontend && pnpm exec vitest run src/views/admin/__tests__/LotteryView.spec.ts src/views/user/__tests__/LotteryView.spec.ts
```

Expected: the new assertions fail because the service still calculates activity attempts and the admin/user templates still expose the old policy.

### Task 2: Make lottery service and repository wallet-only

**Files:**
- Modify: `backend/internal/service/lottery.go`
- Modify: `backend/internal/repository/lottery_repo.go`
- Modify: `backend/internal/handler/admin/lottery_handler.go` (only if request normalization needs a handler boundary)
- Test: `backend/internal/service/lottery_service_test.go`
- Test: `backend/internal/service/lottery_attempt_grants_test.go`

**Step 1: Normalize legacy activity policy**

In `SaveActivity`, force the persisted policy to `attempt_mode=total` and `attempt_limit=0` before validation/repository delegation. Normalize service activity responses the same way so old daily settings cannot be presented as active behavior.

**Step 2: Replace activity quota summaries**

Change the attempt summary helper to accept only the wallet balance and always return `ActivityRemaining=0`, `RewardRemaining=walletBalance`, `TotalRemaining=walletBalance`, and `NextSource=wallet` when available. Remove daily start calculations from public-state and draw paths.

**Step 3: Update public state and draw accounting**

Read the wallet as the only remaining-attempt source. Count current-activity draws without filtering out wallet draws so `attempts_used` remains meaningful. Always persist `attempt_source=wallet`, debit exactly one wallet attempt for each new draw, increment the used-draw count, and preserve existing idempotency and transaction rollback behavior.

**Step 4: Simplify admin balance aggregation**

Stop loading activity policy for the admin balance query and return zero activity remaining plus the wallet balance for reward/total remaining. Keep legacy query fields in the internal type if needed by adapters, but do not populate or use them.

**Step 5: Run backend focused tests**

```bash
cd backend && go test ./internal/service ./internal/repository ./internal/handler/admin -run 'Lottery|lottery' -count=1
```

Expected: wallet-only service, repository, and handler tests pass.

### Task 3: Remove obsolete admin configuration controls

**Files:**
- Modify: `frontend/src/views/admin/LotteryView.vue`
- Modify: `frontend/src/api/admin/lottery.ts`
- Modify: `frontend/src/views/admin/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/api/__tests__/lottery.spec.ts`

**Step 1: Remove policy controls from the form**

Delete the attempt mode select and attempt limit input from the activity settings section. Remove their local form state/imports and save them neither from the UI nor from the admin API request type. Keep response fields optional for old server responses if TypeScript compatibility requires it.

**Step 2: Update attempt balance labels**

Show the wallet value as the available attempt count and remove the obsolete activity-remaining column from the admin table. Update the explanatory copy to say attempts come from consecutive check-ins or administrator grants.

**Step 3: Update API/view tests**

Change the old zero-limit save test to assert the controls are absent and the payload contains the activity metadata only. Keep grant, user balance, and draw audit coverage unchanged.

**Step 4: Run focused frontend tests**

```bash
cd frontend && pnpm exec vitest run src/api/__tests__/lottery.spec.ts src/views/admin/__tests__/LotteryView.spec.ts
```

Expected: PASS.

### Task 4: Align the user lottery display and localization

**Files:**
- Modify: `frontend/src/views/user/LotteryView.vue`
- Modify: `frontend/src/views/user/__tests__/LotteryView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/lottery.ts`
- Modify: `frontend/src/i18n/locales/en/lottery.ts`

**Step 1: Update the attempt summary**

Remove daily/total labels from the hero. Display the wallet/check-in/admin-grant count and keep the total used-draw count without implying a reset schedule.

**Step 2: Update localized copy**

Change the breakdown/hints and admin descriptions to explain the two valid sources. Retain deprecated translation keys only where needed by older clients; no API behavior changes are required on the client.

**Step 3: Run the user lottery tests**

```bash
cd frontend && pnpm exec vitest run src/views/user/__tests__/LotteryView.spec.ts
```

Expected: PASS with no daily/total wording in rendered user markup.

### Task 5: Verify, inspect, and commit

**Files:**
- Modify only files required by verification failures.

**Step 1: Run backend checks**

```bash
cd backend && gofmt -w internal/service/lottery.go internal/repository/lottery_repo.go internal/handler/admin/lottery_handler.go && go test ./internal/service ./internal/repository ./internal/handler/admin -count=1
```

Expected: all targeted backend packages pass.

**Step 2: Run frontend checks**

```bash
cd frontend && pnpm run typecheck && pnpm run lint:check && pnpm run test:run && pnpm run build
```

Expected: typecheck, lint, and build pass. Report unrelated pre-existing full-suite failures separately if they occur.

**Step 3: Check local services and patch hygiene**

```bash
cd .. && git diff --check && git status --short --branch
curl -sS -o /dev/null -w 'backend /health: %{http_code}\n' http://127.0.0.1:18081/health
curl -sS -o /dev/null -w 'frontend /: %{http_code}\n' http://127.0.0.1:3000/
```

Expected: no whitespace errors, only intended files changed, and both local endpoints return 200.

**Step 4: Commit the implementation**

```bash
git add backend frontend/src docs/plans/2026-09-03-lottery-wallet-only-design.md docs/plans/2026-09-03-lottery-wallet-only.md
git commit -m "fix: use wallet-only lottery attempts"
```
