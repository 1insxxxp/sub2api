# Balance Preauthorization Risk Gate Plan
> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce routine negative wallet balances for balance-billed gateway requests without limiting response output, using the latest 100 balance-billed samples, a 2.0 mean safety factor, P95 as a floor, and a one-request overdraft allowance capped at 50% of the computed preauthorization.

**Architecture:** Add a pure estimator in the service layer. Before an eligible balance request is forwarded upstream, load the user's recent positive balance-billed `actual_cost` values and compute `authorization = max(mean(samples)*2, p95(samples), minimum_balance_reserve)`. The request is admitted only when the current balance is at least `authorization * (1 - 0.5)`. This is a risk gate rather than output truncation: the existing post-response settlement still charges the actual cost, and the existing overdraft behavior remains available for outliers and races.

**Tech Stack:** Go, PostgreSQL, existing `BillingCacheService`, `UserRepository`, `usage_logs`, unit tests with existing test doubles.

---

### Task 1: Add estimator tests and implementation

- Add table-driven tests for empty samples, mean-driven estimates, P95-driven estimates, invalid samples, and quantization.
- Run the focused tests and confirm they fail before implementation.
- Implement the estimator with deterministic percentile indexing, finite/nonnegative sample filtering, and the configured safety/overdraft defaults.

### Task 2: Read recent balance usage samples

- Add an optional repository capability that reads the latest 100 positive `usage_logs.actual_cost` rows for one user where `billing_type = 0`.
- Keep the capability optional so existing service test doubles and non-production integrations continue to work; a missing capability fails open to the existing minimum-balance gate.
- Add repository SQL tests for ordering, limit, and balance billing filter.

### Task 3: Apply the risk gate to balance eligibility

- Extend billing configuration with sample count, safety factor, P95 floor, and overdraft fraction defaults and validation.
- In `CheckBillingEligibility`, keep subscription paths unchanged.
- For balance paths, calculate the preauthorization from recent samples and require balance >= authorization × (1 - overdraft fraction). Do not modify request bodies, `max_tokens`, streaming, or output handling.
- Log the computed gate at debug level without logging request content or secrets.

### Task 4: Verify and publish

- Run focused service/repository tests, config tests, and the relevant Go package tests.
- Run frontend checks only if the working tree includes unrelated UI changes; otherwise keep this release backend-only.
- Commit the implementation, push the active branch, build the production image, deploy with the documented rolling/fallback process, and verify both public health endpoints and the configured version.

