# Domestic Model Pricing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add official non-zero fallback pricing for the domestic models used by the production `国模测试` group while preserving USD accounting and existing group/account multipliers.

**Architecture:** Extend `BillingService`'s existing per-token fallback registry and most-specific-first model matcher. Encode Qwen3.7 Plus's second context tier with the existing session long-context fields so input, output, cache read, and cache creation all scale consistently above 256K tokens.

**Tech Stack:** Go, `testify/require`, build-tagged unit tests, existing `BillingService` pricing and cost calculation APIs.

---

### Task 1: Lock Down Domestic Model Price Resolution

**Files:**
- Modify: `backend/internal/service/billing_service_test.go`

**Step 1: Write the failing fallback-price tests**

Add table cases to `TestGetFallbackPricing_FamilyMatching` for both canonical and `cline-pass/` production names. Assert these USD-per-token values:

```go
// Official list prices converted at USD 1 = CNY 7.14 where required.
{model: "kimi-k3", input: 2.80e-6, output: 14.00e-6, cacheRead: 0.28e-6}
{model: "kimi-k2.7-code", input: 0.91e-6, output: 3.78e-6, cacheRead: 0.182e-6}
{model: "qwen3.7-max", input: 1.68e-6, output: 5.04e-6, cacheRead: 0.336e-6, cacheCreation: 2.10e-6}
{model: "qwen3.7-plus", input: 0.28e-6, output: 1.12e-6, cacheRead: 0.056e-6, cacheCreation: 0.35e-6}
{model: "mimo-v2.5-pro", input: 0.42e-6, output: 0.84e-6, cacheRead: 0.0035e-6}
{model: "mimo-v2.5", input: 0.14e-6, output: 0.28e-6, cacheRead: 0.0028e-6}
{model: "glm-5.2", input: 1.40e-6, output: 4.40e-6, cacheRead: 0.26e-6}
```

For each price, add the exact prefixed production alias such as `cline-pass/kimi-k3`. Assert Qwen3.7 Max and Plus set `CacheCreationPriceExplicit` and their official cache-creation prices. Assert Qwen3.7 Plus sets threshold `256000` with input and output multipliers `3.0`.

**Step 2: Replace obsolete regression expectations**

Rename `TestGetModelPricing_GLM52FallsBackToGLM5Price` to describe exact GLM-5.2 pricing and change its expectations to `$1.40/$4.40/$0.26` per million tokens. Add a corresponding exact-price regression test for `cline-pass/kimi-k2.7-code` to prove it does not inherit generic Kimi K2 pricing.

**Step 3: Run tests to verify RED**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'TestGetFallbackPricing_FamilyMatching|TestGetModelPricing_GLM52|TestGetModelPricing_KimiK27' -count=1
```

Expected: FAIL because the new models either return nil or inherit an older family price.

**Step 4: Commit the failing tests**

```bash
git add backend/internal/service/billing_service_test.go
git commit -m "test: cover domestic model fallback pricing"
```

### Task 2: Add Exact Official Fallback Prices

**Files:**
- Modify: `backend/internal/service/billing_service.go`

**Step 1: Add the fallback registry entries**

Add `ModelPricing` entries for `glm-5.2`, `kimi-k3`, `kimi-k2.7-code`, `qwen3.7-max`, `qwen3.7-plus`, `mimo-v2.5-pro`, and `mimo-v2.5`. Store all prices as USD per token. For Qwen cache creation set `CacheCreationPriceExplicit: true`; for Qwen3.7 Plus set:

```go
LongContextInputThreshold:  256000,
LongContextInputMultiplier: 3.0,
LongContextOutputMultiplier: 3.0,
```

**Step 2: Add most-specific-first matcher branches**

Match `glm-5.2` before `glm-5`, `kimi-k2.7-code` before `kimi-k2`, and `mimo-v2.5-pro` before `mimo-v2.5`. Add explicit branches for Kimi K3 and Qwen3.7 Max/Plus. Continue using substring matching so `cline-pass/` aliases resolve without introducing provider-specific duplication.

**Step 3: Format and verify GREEN**

Run:

```bash
gofmt -w backend/internal/service/billing_service.go backend/internal/service/billing_service_test.go
cd backend
go test -tags=unit ./internal/service -run 'TestGetFallbackPricing_FamilyMatching|TestGetModelPricing_GLM52|TestGetModelPricing_KimiK27' -count=1
```

Expected: PASS.

**Step 4: Commit the implementation**

```bash
git add backend/internal/service/billing_service.go backend/internal/service/billing_service_test.go
git commit -m "feat: price domestic model fallbacks"
```

### Task 3: Verify Qwen Tiering and Multiplier Semantics

**Files:**
- Modify: `backend/internal/service/billing_service_test.go`

**Step 1: Write focused cost tests**

Add one below-threshold test and one above-threshold test for `cline-pass/qwen3.7-plus`. Below 256K, assert base input/output/cache-read/cache-creation prices. Above 256K total input tokens, assert all four components use the 3x session multiplier and `LongContextBillingApplied` is true.

Add a representative `cline-pass/kimi-k3` calculation with rate multiplier `3.0`. Assert `TotalCost > 0`, `TotalCost` remains the official base cost, and `ActualCost == TotalCost * 3`.

**Step 2: Run the new tests to verify behavior**

Run:

```bash
cd backend
go test -tags=unit ./internal/service -run 'TestCalculateCost_(Qwen37Plus|DomesticModel)' -count=1
```

Expected: PASS using the implementation from Task 2. If a test fails, make the smallest billing-service correction and rerun until green.

**Step 3: Commit tier and multiplier coverage**

```bash
git add backend/internal/service/billing_service.go backend/internal/service/billing_service_test.go
git commit -m "test: verify domestic model billing tiers"
```

### Task 4: Run Repository Verification

**Files:**
- Verify: `backend/internal/service/billing_service.go`
- Verify: `backend/internal/service/billing_service_test.go`

**Step 1: Check formatting and focused tests**

```bash
test -z "$(gofmt -l backend/internal/service/billing_service.go backend/internal/service/billing_service_test.go)"
cd backend
go test -tags=unit ./internal/service -count=1
```

Expected: both commands exit 0.

**Step 2: Run the full backend unit suite**

```bash
cd backend
go test -tags=unit ./... -count=1
```

Expected: PASS. Record any pre-existing unrelated failure separately rather than changing unrelated code.

**Step 3: Review the final diff and commit any remaining test-only adjustment**

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only intended files plus the user's pre-existing untracked files are present.

### Task 5: Deploy Without Interrupting Existing Traffic

**Files:**
- Production source: `/opt/sub2api-custom-prod`
- Compose: `/opt/sub2api-custom-prod/deploy/docker-compose.local.yml`
- Compose override: `/opt/sub2api-custom-prod/deploy/docker-compose.custom.yml`

**Step 1: Capture production state and back up changed source files**

Over SSH with proxy variables unset, record the running image/container health, current Git revision, and compose configuration. Create timestamped copies of the production billing service files before changing them.

**Step 2: Transfer the verified commit and rebuild the app image**

Update only the intended source revision, build `sub2api-custom:custom-prod`, and confirm the image completes successfully before replacing the app container. Do not restart PostgreSQL or Redis.

**Step 3: Replace only the application container**

Use the existing compose files to recreate `sub2api`, then verify container health, HTTP readiness, and recent logs. If readiness fails, restore the previous image/source and recreate only `sub2api`.

### Task 6: Verify Real Production Billing

**Files:**
- Verify production `usage_logs` rows only; do not alter historical rows.

**Step 1: Send a minimal request through group 71**

Use an existing authorized test key/account and one newly priced model. Keep token usage minimal.

**Step 2: Inspect the new usage row**

Assert the row records the intended model, non-zero `total_cost`, non-zero `actual_cost`, and multiplier `3.0`. Confirm `actual_cost` is approximately `total_cost * 3` (subject to any already-configured account multiplier).

**Step 3: Check service health after the request**

Verify the app remains healthy and logs show no new pricing or billing errors. Do not retroactively charge historical zero-cost usage.
