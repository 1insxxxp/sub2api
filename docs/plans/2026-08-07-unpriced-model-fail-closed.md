# Unpriced Model Fail-Closed Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure every schedulable request has resolvable pricing before an upstream account is invoked, while removing the two currently unpriced Gemini models from the unsafe state.

**Architecture:** Add a shared service-layer pricing-admission helper that evaluates channel, requested, channel-mapped, and account-mapped model candidates. Integrate it into the generic and OpenAI scheduling paths so unpriced accounts are excluded before forwarding, while explicit image/per-request pricing remains valid. Apply narrowly scoped production channel/account configuration changes after tests pass.

**Tech Stack:** Go, PostgreSQL, Redis scheduler cache, Vue user-facing channel data

---

### Task 1: Specify pricing admission behavior

**Files:**
- Create: `backend/internal/service/model_pricing_admission_test.go`
- Create: `backend/internal/service/model_pricing_admission.go`

**Step 1: Write failing tests**

Cover these cases:

- explicit channel token pricing is accepted;
- explicit channel image/per-request pricing is accepted without token fields;
- a requested alias with no price is accepted when its concrete account mapping has a global price;
- known Gemini aliases are accepted through canonical pricing;
- a requested and concrete model with no price is rejected with `ErrModelPricingUnavailable`;
- an empty channel token-pricing row does not count as usable pricing.

**Step 2: Run the focused tests and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestModelPricingAdmission' -count=1`

Expected: FAIL because the admission helper does not exist.

**Step 3: Implement the smallest shared helper**

The helper receives group ID, requested model, channel mapping, and concrete
account model. It checks explicit usable channel pricing first, then each unique
model candidate through `BillingService.GetModelPricing`. Empty token rows are
not usable; image/per-request rows with a configured unit price are usable.

**Step 4: Run focused tests**

Run: `cd backend && go test ./internal/service -run 'TestModelPricingAdmission' -count=1`

Expected: PASS.

### Task 2: Enforce admission before generic upstream forwarding

**Files:**
- Modify: `backend/internal/service/gateway_scheduling.go`
- Modify: `backend/internal/service/gateway_forward.go`
- Test: `backend/internal/service/gateway_model_pricing_admission_test.go`

**Step 1: Write a failing scheduling test**

Create one priced and one unpriced account candidate. Assert that the unpriced
candidate is excluded, the priced mapped candidate is selected, and an all-
unpriced candidate set returns `ErrModelPricingUnavailable` without invoking an
upstream client.

**Step 2: Run the focused test and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestGateway.*PricingAdmission' -count=1`

Expected: FAIL because unpriced candidates remain schedulable.

**Step 3: Apply the admission helper during candidate filtering**

Evaluate the effective model after channel and account mappings. Track pricing
exclusions separately from model/availability exclusions so the terminal error
reports a local pricing configuration problem and does not trigger account
cooldown.

**Step 4: Run focused and gateway tests**

Run: `cd backend && go test ./internal/service -run 'TestGateway.*(PricingAdmission|ModelAvailability|Channel)' -count=1`

Expected: PASS.

### Task 3: Enforce admission in OpenAI-compatible scheduling

**Files:**
- Modify: `backend/internal/service/openai_account_scheduler.go`
- Modify: `backend/internal/service/openai_gateway_scheduling.go`
- Test: `backend/internal/service/openai_account_scheduler_pricing_test.go`

**Step 1: Write failing tests**

Verify unknown OpenAI-compatible model candidates are excluded before forward,
mapped models with pricing remain usable, and explicit image/per-request channel
pricing remains schedulable.

**Step 2: Run the focused tests and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestOpenAI.*PricingAdmission' -count=1`

Expected: FAIL.

**Step 3: Integrate the shared helper into OpenAI candidate filtering**

Use the same error classification as the generic scheduler and do not report the
failure as an upstream account health event.

**Step 4: Run focused tests**

Run: `cd backend && go test ./internal/service -run 'TestOpenAI.*(PricingAdmission|ModelAvailability|Channel)' -count=1`

Expected: PASS.

### Task 4: Preserve usage billing defense in depth

**Files:**
- Modify: `backend/internal/service/gateway_usage_billing.go`
- Test: `backend/internal/service/gateway_record_usage_test.go`

**Step 1: Add a regression test**

Assert an unexpected pricing miss is returned as `ErrModelPricingUnavailable`
instead of silently constructing a zero-cost breakdown.

**Step 2: Run the focused test and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestGateway.*PricingUnavailable' -count=1`

Expected: FAIL because the existing path returns zero cost.

**Step 3: Propagate the pricing error**

Keep usage logging best-effort, but never treat an unresolved model as a valid
zero-cost calculation. Existing successful zero-price subscription semantics,
if explicitly configured, remain unchanged.

**Step 4: Run billing tests**

Run: `cd backend && go test ./internal/service -run '(Billing|Pricing|RecordUsage)' -count=1`

Expected: PASS.

### Task 5: Verify the complete backend

**Files:**
- No new files

**Step 1: Format modified Go files**

Run: `cd backend && gofmt -w internal/service/model_pricing_admission.go internal/service/*pricing*test.go`

**Step 2: Run the full backend suite**

Run: `cd backend && go test ./...`

Expected: PASS with zero failures.

**Step 3: Commit implementation**

Stage only the relevant backend files and commit with:

`fix: reject requests without resolvable pricing`

### Task 6: Apply production pricing and visibility configuration

**Files:**
- Modify: production `channel_model_pricing` for channel 8
- Modify: production account mappings that expose `gemini-2.5-flash-image-preview`

**Step 1: Snapshot affected rows**

Save the current `gemini-3-pro`/preview pricing and account mappings for rollback.

**Step 2: Configure `gemini-3-pro` pricing**

Set token prices to input `0.000002`, output `0.000012`, and cache read
`0.0000002` USD per token.

**Step 3: Remove the unsafe preview image model**

Delete only `gemini-2.5-flash-image-preview` from affected account mappings and
remove its empty channel-pricing entry.

**Step 4: Verify scheduler-cache refresh and available-channel output**

Confirm the account snapshot no longer exposes the removed model and
`gemini-3-pro` has a non-null display price.

### Task 7: Deployment and production verification

**Files:**
- No source changes

**Step 1:** Confirm the branch is clean except for user-owned unrelated files.

**Step 2:** Build the backend image and run deployment readiness checks.

**Step 3:** Do not deploy until the user separately authorizes deployment.

**Step 4:** After deployment, verify an unknown model is rejected before any
account attempt, `gemini-3-pro` records non-zero cost, and no successful unknown
model usage has `actual_cost = 0`.
