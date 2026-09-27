# Gemini Pool Failover Limit Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Gemini pool-mode accounts retry twice on the same account and allow up to ten distinct Gemini account attempts per request.

**Architecture:** Reuse the existing bounded failover loop. Set the Gemini switch default to 9 because the first selected account plus 9 switches equals 10 distinct accounts. Lower only the Gemini fallback default for `pool_mode_retry_count`; keep other platform defaults at 3 and explicit account credential values authoritative.

**Tech Stack:** Go backend, Viper configuration, Vue 3 account modals, Go unit tests, Vitest component tests.

---

### Task 1: Lock backend defaults with failing tests

**Files:**
- Modify: `backend/internal/service/account_pool_mode_test.go`
- Modify: `backend/internal/config/config_test.go`

**Step 1: Write the failing assertions**

- Assert a Gemini API-key pool account without an explicit retry count resolves to `2`, while another platform still resolves to `3`.
- Assert loaded gateway configuration resolves `max_account_switches_gemini` to `9`.

**Step 2: Run focused tests and verify they fail**

Run: `go test -tags=unit ./internal/service ./internal/config`

Expected: failures show the current defaults `3`.

### Task 2: Implement backend retry and switch defaults

**Files:**
- Modify: `backend/internal/service/account.go`
- Modify: `backend/internal/config/config.go`

**Step 1: Change the minimal defaults**

- Add a Gemini-specific default of `2` while keeping the general default at `3`.
- Set `gateway.max_account_switches_gemini` default to `9`.

**Step 2: Run focused tests**

Run: `go test -tags=unit ./internal/service ./internal/config ./internal/handler`

Expected: pass, including explicit retry-count and existing failover-limit tests.

### Task 3: Synchronize account UI defaults

**Files:**
- Modify: `frontend/src/components/account/CreateAccountModal.vue`
- Modify: `frontend/src/components/account/EditAccountModal.vue`

**Step 1: Update the shared form defaults**

- Keep the general `DEFAULT_POOL_MODE_RETRY_COUNT` at `3` and use a Gemini-specific default of `2`.
- Keep the maximum at `10` and preserve explicit saved values.

**Step 2: Run focused frontend tests and type checks**

Run: `pnpm exec vitest run frontend/src/components/account --passWithNoTests`

Expected: pass, or report if this repository does not expose component tests for these large modals.

### Task 4: Verify the diff and local behavior

**Files:**
- No additional files.

**Step 1: Run verification**

Run:

```bash
go test -tags=unit ./internal/service ./internal/config ./internal/handler
git diff --check
```

Expected: all targeted Go tests pass and the diff has no whitespace errors.

**Step 2: Confirm scope**

- Verify only the Gemini default switch budget changed; generic and OpenAI switch defaults remain unchanged.
- Verify no explicit account credentials or production configuration were modified.
