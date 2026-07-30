# Group Model Per-Request Billing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the existing channel pricing system a verified and clearly configurable path for charging a fixed successful-request price per group and exact player-requested model.

**Architecture:** Reuse `channel_groups`, `channel_model_pricing`, `BillingModelSourceRequested`, `ModelPricingResolver`, and the existing post-success usage billing transaction. Add regression coverage at the resolver, OpenAI usage, shared Claude/Gemini usage, and handler success boundaries; improve the channel pricing UI copy so administrators deliberately select requested-model pricing and understand fallback behavior.

**Tech Stack:** Go, Gin, Ent/PostgreSQL repositories, Testify, Vue 3, TypeScript, Vitest, pnpm.

---

### Task 1: Lock Down Group + Requested Model Pricing Resolution

**Files:**
- Modify: `backend/internal/service/model_pricing_resolver_test.go`
- Modify: `backend/internal/service/channel_service_test.go`

**Step 1: Write failing resolver tests**

Add table-driven tests proving:

```go
func TestModelPricingResolver_GroupRequestedModelPerRequest(t *testing.T) {
    // group 10 has exact gpt-5.5 per_request price 0.05
    // Resolve(model="gpt-5.5", group=10) => per_request, channel source, 0.05
    // Resolve(model="GPT-5.5", group=10) => same result
    // Resolve(model="gpt-5.4", group=10) => token fallback
}
```

Also cover two groups attached to different channels using different prices for the same model.

**Step 2: Run tests and verify current behavior**

Run:

```bash
cd backend
go test ./internal/service -run 'Test(ModelPricingResolver_GroupRequestedModelPerRequest|ChannelService.*PerRequest)' -count=1
```

Expected: tests either pass and characterize the existing implementation, or fail at the exact missing behavior without unrelated failures.

**Step 3: Implement the smallest resolver/cache correction if required**

Keep pricing lookup keyed by normalized `group_id + platform + requested model`. Do not add a second pricing store. Preserve token fallback when no exact model entry is present.

**Step 4: Run focused tests**

Run the command from Step 2.

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/model_pricing_resolver_test.go backend/internal/service/channel_service_test.go backend/internal/service/model_pricing_resolver.go backend/internal/service/channel_service.go
git commit -m "test: cover group model per-request pricing resolution"
```

### Task 2: Verify OpenAI Requested-Model Per-Request Billing

**Files:**
- Modify: `backend/internal/service/openai_gateway_record_usage_test.go`
- Modify if required: `backend/internal/service/openai_gateway_usage.go`

**Step 1: Write failing usage tests**

Add tests where:

```go
input.OriginalModel = "tavern-gpt"
input.ChannelMappedModel = "gpt-5.5"
input.BillingModelSource = BillingModelSourceRequested
```

Configure `tavern-gpt` as `$0.05/request` for the group and provide non-zero input/output/cache tokens. Assert:

- `ActualCost` is the fixed request price multiplied only by the existing effective text multiplier;
- token costs do not contribute to the amount;
- usage log token fields retain their values;
- usage log `RequestedModel` is `tavern-gpt`;
- usage log `UpstreamModel` remains the actual upstream model;
- usage log `BillingMode` is `per_request`.

Add a second test where `tavern-gpt` has no channel rule and assert token billing remains active.

**Step 2: Run tests and verify failure/pass state**

Run:

```bash
cd backend
go test ./internal/service -run 'TestOpenAI.*RequestedModel.*PerRequest' -count=1
```

Expected: new assertions expose any billing-model or log snapshot gap.

**Step 3: Implement the smallest correction if required**

Ensure `BillingModelSourceRequested` selects `OriginalModel` before resolver lookup and does not later fall back to the mapped model when an explicit channel per-request price exists.

**Step 4: Run focused OpenAI usage tests**

Run the command from Step 2 plus:

```bash
go test ./internal/service -run 'TestOpenAI.*RecordUsage' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/openai_gateway_record_usage_test.go backend/internal/service/openai_gateway_usage.go
git commit -m "test: verify OpenAI requested-model per-request billing"
```

### Task 3: Verify Claude and Gemini Requested-Model Billing

**Files:**
- Modify: `backend/internal/service/gateway_usage_billing_test.go`
- Modify if required: `backend/internal/service/gateway_usage_billing.go`

**Step 1: Write failing shared billing tests**

Create table-driven cases for Anthropic Messages and Gemini text results. Each case uses:

```go
OriginalModel: "tavern-roleplay-pro"
ChannelMappedModel: "claude-opus-4.8" // or Gemini upstream model
BillingModelSource: BillingModelSourceRequested
```

Assert a configured `$0.08/request` price replaces token cost while token counts remain in the usage log. Add an unconfigured requested model case that falls back to token billing.

**Step 2: Run focused tests**

Run:

```bash
cd backend
go test ./internal/service -run 'TestGateway.*RequestedModel.*PerRequest' -count=1
```

Expected: FAIL only if the shared billing path loses the requested model or ignores per-request pricing.

**Step 3: Implement the smallest shared-path correction if required**

Keep the selected billing model as `OriginalModel` for resolver lookup while retaining final model fields for routing and analytics.

**Step 4: Run focused tests**

Run the command from Step 2.

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/gateway_usage_billing_test.go backend/internal/service/gateway_usage_billing.go
git commit -m "test: verify shared requested-model per-request billing"
```

### Task 4: Prove Only Successful Requests Are Charged

**Files:**
- Modify: `backend/internal/handler/openai_chat_completions_test.go`
- Modify: `backend/internal/handler/gateway_handler_chat_completions_test.go`
- Modify handler files only if tests identify a regression.

**Step 1: Write success-boundary tests**

For the OpenAI-specific and shared gateway handlers, add cases asserting `RecordUsage` is called exactly once after a successful result and never called for:

- local validation/restriction rejection;
- scheduler/account selection failure;
- upstream network error;
- upstream 4xx/5xx result;
- timeout/cancellation before a successful result.

Use mocks/spies around the usage recorder rather than checking only HTTP status.

**Step 2: Run tests and confirm behavior**

Run:

```bash
cd backend
go test ./internal/handler -run 'Test.*(PerRequest|RecordUsage).*Success' -count=1
```

Expected: successful requests record once; every failure case records zero times.

**Step 3: Correct handler ordering only if required**

Keep usage recording after the forward result has been accepted as successful. Do not add pre-charge or reservation logic.

**Step 4: Run handler package tests**

Run:

```bash
go test ./internal/handler/... -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/openai_chat_completions_test.go backend/internal/handler/gateway_handler_chat_completions_test.go backend/internal/handler/openai_chat_completions.go backend/internal/handler/gateway_handler_chat_completions.go
git commit -m "test: charge per-request pricing only after success"
```

### Task 5: Clarify the Channel Pricing Administration UI

**Files:**
- Modify: `frontend/src/components/admin/channel/PricingEntryCard.vue`
- Modify: `frontend/src/views/admin/ChannelsView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/channels.ts`
- Modify: `frontend/src/i18n/locales/en/admin/channels.ts`
- Modify: `frontend/src/views/admin/__tests__/ChannelsView.spec.ts`
- Modify or create: `frontend/src/components/admin/channel/__tests__/PricingEntryCard.spec.ts`

**Step 1: Write failing UI tests**

Assert that selecting `per_request` displays concise help text stating:

- price is USD per successful request;
- it replaces token charges;
- models without a per-request rule retain token billing;
- for player-facing model names, billing model source must be `requested`.

Add a form-level warning when any text model uses `per_request` while `billing_model_source` is not `requested`. The warning should not block legacy configurations because existing administrators may intentionally bill mapped/upstream models.

**Step 2: Run tests and verify failure**

Run:

```bash
cd frontend
pnpm exec vitest run src/views/admin/__tests__/ChannelsView.spec.ts src/components/admin/channel/__tests__/PricingEntryCard.spec.ts
```

Expected: FAIL because the explanatory state is not rendered yet.

**Step 3: Implement the UI copy and warning**

Add localized help text under the per-request price input and a visible warning near the billing model source selector when the recommended requested-model setting is not selected.

Do not add a second pricing editor to the group page.

**Step 4: Run focused frontend tests**

Run the command from Step 2.

Expected: PASS.

**Step 5: Run frontend typecheck**

Run:

```bash
pnpm typecheck
```

Expected: PASS.

**Step 6: Commit**

```bash
git add frontend/src/components/admin/channel/PricingEntryCard.vue frontend/src/views/admin/ChannelsView.vue frontend/src/i18n/locales/zh/admin/channels.ts frontend/src/i18n/locales/en/admin/channels.ts frontend/src/views/admin/__tests__/ChannelsView.spec.ts frontend/src/components/admin/channel/__tests__/PricingEntryCard.spec.ts
git commit -m "feat: clarify requested-model per-request pricing"
```

### Task 6: End-to-End Verification and Local Test Setup

**Files:**
- No expected source changes.

**Step 1: Run backend focused suites**

```bash
cd backend
go test ./internal/service ./internal/handler/... -count=1
```

Expected: PASS.

**Step 2: Run backend full suite**

```bash
go test ./... -count=1
```

Expected: PASS.

**Step 3: Run frontend verification**

```bash
cd ../frontend
pnpm typecheck
pnpm exec vitest run src/views/admin/__tests__/ChannelsView.spec.ts src/components/admin/channel/__tests__/PricingEntryCard.spec.ts
pnpm build
```

Expected: PASS, allowing existing non-fatal Browserslist/Vite warnings.

**Step 4: Run formatting and repository checks**

```bash
cd ..
git diff --check
git status --short --branch
```

Expected: no whitespace errors; root `package.json` and `package-lock.json` remain untouched and untracked.

**Step 5: Restart local development services**

Restart the existing Go server on `127.0.0.1:18081` and keep Vite running on `127.0.0.1:3000`. Confirm:

```bash
curl -fsS http://127.0.0.1:18081/health
curl -fsSI http://127.0.0.1:3000/
```

Expected: backend returns `{"status":"ok"}` and frontend returns HTTP 200.

**Step 6: Manual billing smoke test**

In the local admin UI:

1. Open Channel Pricing.
2. Associate the target group.
3. Set billing model source to Requested Model.
4. Add `gpt-5.5` with `per_request` price `$0.01`.
5. Send one successful request and one deliberately failing request.
6. Confirm only the successful request creates a `per_request` usage row and deducts `$0.01` times the effective text multiplier.
7. Send an unconfigured model and confirm it records `billing_mode=token`.

**Step 7: Commit any verification-only fixture changes if needed**

No commit is expected. Do not commit local test data or credentials.
