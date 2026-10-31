# Low-Balance Admission Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let low positive balances fund short requests without historical-cost false rejections, while preserving billing and concurrency protections.

**Architecture:** Remove the historical-cost hot-path gate. Add bounded/coalesced authoritative reads only before wallet rejection, recover stale auth with copies, and retry denied atomic in-flight admission once against real balance. Keep exact-charge and batch holds and all final charging unchanged.

**Tech Stack:** Go, Gin, PostgreSQL, Redis, existing singleflight and release workflows.

---

### Task 1: Wallet eligibility and authoritative recovery

Files: `backend/internal/service/billing_cache_service.go`, `billing_cache_service_balance_test.go`, new `wallet_balance_recheck.go` and tests, `backend/internal/config/config.go`, `deploy/config.example.yaml`.

1. Add a regression expecting a small positive balance to pass despite expensive history, and verify it fails against the old gate with `go test -p 2 -tags=unit ./internal/service -run '^TestCheckBillingEligibility' -count=1`.
2. Add stale-zero, true-zero and database-failure cases. Assert no cache SET on recovery.
3. Implement a zero-value rechecker with per-user singleflight, a 3-second DB timeout and a bounded number of simultaneous reads. Remove the historical lookup from admission; preserve positive/minimum reserve rules and fail unavailable on DB failures.
4. Mark historical preauthorization settings compatibility-only, keeping existing configuration parsing.
5. Run the targeted tests, including concurrency/cancellation/error recovery tests.

### Task 2: Auth recovery

Files: `backend/internal/service/api_key_service.go`, new `api_key_balance_recheck.go`, `backend/internal/server/middleware/api_key_auth.go`, `api_key_auth_google.go`, corresponding tests.

1. Reproduce stale-zero rejection for both normal and Google auth.
2. Recheck the wallet only at a balance rejection. Copy fresh user/key state, preserve user status and group restrictions and invalidate only the current auth key entry. Leave quotas, expiry, ACLs and subscriptions enforced.
3. Do not retain verified balances in request/WS context. Check DB outage becomes HTTP 503.
4. Run middleware and service regression tests.

### Task 3: In-flight diagnostics and recovery

Files: `backend/internal/service/billing_inflight_reservation.go` and tests, `backend/internal/handler/gateway_inflight_reservation.go`, `gateway_handler.go:billingErrorDetails`, matching tests.

1. Reproduce stale cached balance rejecting an otherwise affordable second request.
2. On denial, reread authoritative balance and retry the atomic reservation once with the same reservation ID. Return `ErrBalanceInFlight` (429, retry after 2 seconds) if still occupied; preserve actual wallet-empty errors and fail unavailable on DB outage.
3. Fail-closed unknown pricing returns service unavailable. Preserve fixed-charge coverage and default Redis failure handling.
4. Verify first low-balance request, concurrent denial and pending settlement lifecycle.

### Task 4: Review, gates and production deployment

1. Review requirements and code, correct findings, run targeted race tests and all unit/integration/lint/security/frontend release checks. Keep test outputs outside the repository.
2. Commit, fast-forward local dev and push. Verify the remote exact SHA and all CI jobs; use the existing deployment-image workflow to build with VERSION and COMMIT.
3. Verify artifact digest, image version and actual production routes. Preserve container runtime settings, mounts/network and Nginx payment bans/static fallbacks. Back up before switching.
4. Start parallel application containers, verify health and balance-admission behavior using dedicated test resources, switch both routes, verify public assets and health, retain rollback and drain old instances.
5. Record deployed SHA, time, release evidence and remaining single-request overdraft behavior.
