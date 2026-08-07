# Empty Response Self-Service Compensation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a privacy-preserving self-service system that automatically refunds eligible empty or interrupted charged responses, routes ambiguous cases to admins, and prevents duplicate compensation.

**Architecture:** Gateway response handlers emit a protocol-neutral response outcome that is persisted with the final usage charge. A claim service evaluates ownership, time, group policy, response evidence, and abuse limits; an idempotent repository transaction restores the original wallet/subscription/API-key quota debit while preserving gross usage and recording a compensation ledger. User and admin workflows are embedded in the existing usage pages.

**Tech Stack:** Go 1.26, Gin, PostgreSQL 18, Ent, Redis balance/auth caches, Vue 3, TypeScript, Pinia-compatible API modules, Vitest, Tailwind CSS.

---

## Invariants to preserve throughout implementation

- Never store response text, prompt text, tool arguments, image bytes, or other user content in outcome evidence.
- Never trust a client-provided refund amount, user ID, group ID, or classification.
- Never recompute a refund from current model pricing; use the immutable original billing snapshot.
- Never decrement account/upstream cost or account quota: the operator may still have paid the upstream for an empty response.
- A usage log may be compensated successfully at most once.
- Client cancellation is not automatically compensable.
- Missing evidence routes to manual review; it must never auto-approve.
- The feature defaults off for every group and must be rolled out by explicit group opt-in.

### Task 1: Add compensation persistence and group policy

**Files:**
- Create: `backend/ent/schema/usage_response_outcome.go`
- Create: `backend/ent/schema/empty_response_claim.go`
- Modify: `backend/ent/schema/usage_log.go`
- Modify: `backend/ent/schema/group.go`
- Modify: `backend/ent/schema/user.go`
- Modify: `backend/ent/schema/api_key.go`
- Modify: `backend/ent/schema/account.go`
- Modify: `backend/ent/schema/user_subscription.go`
- Create: `backend/migrations/196_empty_response_compensation.sql`
- Create: `backend/migrations/empty_response_compensation_migration_test.go`
- Regenerate: `backend/ent/`

**Step 1: Write the failing migration contract test**

Assert that migration 196 creates:

- `usage_response_outcomes` with a unique `(request_id, api_key_id)` key and no response-content column.
- `empty_response_claims` with a unique `usage_log_id`, status/reason checks, billing snapshot fields, evidence JSON, and review timestamps.
- `usage_logs.compensated_cost DECIMAL(20,10) NOT NULL DEFAULT 0` with `0 <= compensated_cost <= actual_cost`.
- `groups.empty_response_compensation_enabled BOOLEAN NOT NULL DEFAULT FALSE`.
- indexes for claim status/time, user/day, group, account, and outcome lookup.

**Step 2: Run the migration test to verify it fails**

Run: `cd backend && go test -tags unit ./migrations -run EmptyResponseCompensation -count=1`

Expected: FAIL because migration 196 and schemas do not exist.

**Step 3: Add the SQL migration and Ent schemas**

Use stable string enums:

```go
const (
    EmptyResponseClaimEvaluating   = "evaluating"
    EmptyResponseClaimManualReview = "manual_review"
    EmptyResponseClaimApproved     = "approved"
    EmptyResponseClaimRejected     = "rejected"
    EmptyResponseClaimCompensated  = "compensated"
)
```

The claim row stores `original_actual_cost`, `balance_refund`, `subscription_refund`, `api_key_quota_refund`, `rule_version`, `reason_code`, `evidence`, and reviewer metadata. Add nullable edges for outcome and subscription and required edges for usage log and user.

**Step 4: Generate Ent code**

Run: `cd backend && go generate ./ent`

Expected: generated builders, predicates, mutations, and entity types include both new schemas and the two new fields.

**Step 5: Run schema and migration tests**

Run: `cd backend && go test -tags unit ./migrations ./ent/... -run 'EmptyResponse|Group' -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/ent backend/migrations/196_empty_response_compensation.sql backend/migrations/empty_response_compensation_migration_test.go
git commit -m "feat: add empty response compensation schema"
```

### Task 2: Build the protocol-neutral response outcome collector

**Files:**
- Create: `backend/internal/service/response_outcome.go`
- Create: `backend/internal/service/response_outcome_test.go`
- Create: `backend/internal/service/response_outcome_json.go`
- Create: `backend/internal/service/response_outcome_json_test.go`

**Step 1: Write table-driven failing tests**

Cover:

- whitespace-only text is not valid content;
- text, reasoning, tool calls, function calls, image/file parts, and generated media each count as valid output;
- completion without content is a pure empty response;
- upstream EOF before a terminal event is an upstream interruption;
- `context.Canceled` with a canceled downstream request is a client disconnect;
- 5xx and timeout classifications retain the upstream failure kind;
- snapshots contain booleans/counts only and never raw content.

**Step 2: Run the tests to verify they fail**

Run: `cd backend && go test -tags unit ./internal/service -run ResponseOutcome -count=1`

Expected: FAIL with undefined collector types.

**Step 3: Implement the minimal collector**

Use a request-scoped, non-global value:

```go
type ResponseOutcome struct {
    HTTPStatus       int
    UpstreamStatus   int
    HasText          bool
    HasToolCall      bool
    HasReasoning     bool
    HasMedia         bool
    OutputBytes      int64
    EventCount       int
    StreamCompleted  bool
    FinishReason     string
    DisconnectSource DisconnectSource
    UpstreamErrorKind UpstreamErrorKind
    CollectorVersion int
}

func (o ResponseOutcome) HasEffectiveOutput() bool {
    return o.HasText || o.HasToolCall || o.HasReasoning || o.HasMedia
}
```

Expose event methods rather than accepting raw payloads. JSON/SSE helpers may inspect payloads in memory but only return booleans/counts.

**Step 4: Run the focused tests**

Run: `cd backend && go test -tags unit ./internal/service -run ResponseOutcome -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/response_outcome*.go
git commit -m "feat: classify response outcomes without content retention"
```

### Task 3: Instrument Anthropic-compatible response paths

**Files:**
- Modify: `backend/internal/service/gateway_upstream_response.go`
- Modify: `backend/internal/service/gateway_anthropic_passthrough.go`
- Modify: `backend/internal/service/gateway_forward_as_chat_completions.go`
- Modify: `backend/internal/service/gateway_forward_as_responses.go`
- Modify: `backend/internal/service/antigravity_gateway_streaming.go`
- Modify: `backend/internal/service/gateway_streaming_test.go`
- Modify: `backend/internal/service/gateway_anthropic_apikey_passthrough_test.go`
- Create: `backend/internal/service/gateway_response_outcome_test.go`

**Step 1: Add failing response-path tests**

Feed real Anthropic SSE fixtures for:

- `content_block_delta.text_delta`;
- `content_block_start.tool_use`;
- thinking/reasoning deltas;
- `message_stop` without content;
- upstream EOF before `message_stop`;
- client writer cancellation after partial transfer.

Assert the returned result includes an outcome snapshot but forwarded bytes remain byte-for-byte compatible with current behavior.

**Step 2: Run the focused tests and verify failure**

Run: `cd backend && go test -tags unit ./internal/service -run 'Gateway.*ResponseOutcome|Anthropic.*Outcome' -count=1`

Expected: FAIL because streaming/non-streaming result structs have no outcome.

**Step 3: Thread the collector through Anthropic handlers**

Add `Outcome ResponseOutcome` to the existing result structs. Observe decoded events at the same point already used for usage extraction. Mark a normal terminal frame separately from EOF. Do not add a second body read or buffer an entire stream.

**Step 4: Run existing and new Anthropic tests**

Run: `cd backend && go test -tags unit ./internal/service -run 'GatewayStreaming|AnthropicAPIKeyPassthrough|ResponseOutcome' -count=1`

Expected: PASS with no changes to existing response fixtures.

**Step 5: Commit**

```bash
git add backend/internal/service/gateway_* backend/internal/service/antigravity_gateway_streaming.go
git commit -m "feat: capture Anthropic response outcomes"
```

### Task 4: Instrument OpenAI, Gemini, image, and compatibility paths

**Files:**
- Modify: `backend/internal/service/openai_gateway_response_handling.go`
- Modify: `backend/internal/service/openai_gateway_passthrough.go`
- Modify: `backend/internal/service/openai_gateway_responses_chat_fallback.go`
- Modify: `backend/internal/service/openai_images.go`
- Modify: `backend/internal/service/gemini_messages_compat_service.go`
- Modify: `backend/internal/handler/gemini_v1beta_handler.go`
- Create: `backend/internal/service/openai_response_outcome_test.go`
- Create: `backend/internal/service/gemini_response_outcome_test.go`
- Modify: `backend/internal/service/openai_images_test.go`

**Step 1: Write failing fixtures for each valid output family**

Cover OpenAI `delta.content`, Responses `output_text`, reasoning, function calls, image generation outputs, Gemini `text`, `functionCall`, inline/file media, normal completion, empty completion, and interrupted streams.

**Step 2: Run tests to verify failure**

Run: `cd backend && go test -tags unit ./internal/service ./internal/handler -run '(OpenAI|Gemini|Images).*ResponseOutcome' -count=1`

Expected: FAIL because these paths do not report unified outcomes.

**Step 3: Implement adapters using the shared collector**

Observe payloads where they are already decoded for transformation or usage. For raw passthrough, parse individual SSE data frames incrementally; never retain a full transcript. Treat image URLs/base64 presence as `HasMedia` without retaining the value.

**Step 4: Run focused protocol regression tests**

Run: `cd backend && go test -tags unit ./internal/service ./internal/handler -run '(OpenAI|Gemini|Images|Responses).*Outcome|ImageGenerationControls|Gemini.*CustomGroup' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/openai_* backend/internal/service/gemini_messages_compat_service.go backend/internal/handler/gemini_v1beta_handler.go
git commit -m "feat: capture OpenAI and Gemini response outcomes"
```

### Task 5: Persist the outcome after finalized usage billing

**Files:**
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/service/gateway_usage_billing.go`
- Modify: `backend/internal/service/openai_gateway_usage.go`
- Modify: `backend/internal/repository/usage_log_repo_insert.go`
- Create: `backend/internal/repository/response_outcome_repo.go`
- Modify: `backend/internal/repository/usage_log_repo_unit_test.go`
- Modify: `backend/internal/repository/usage_log_repo_integration_test.go`

**Step 1: Write failing billing persistence tests**

Assert that:

- the final `UsageLog` carries an outcome snapshot derived by the server;
- a successfully billed usage log is inserted first, then its outcome is linked with the same `(request_id, api_key_id)` identity;
- billing retries do not duplicate outcomes;
- an outcome write failure does not roll back billing or the usage log and is retried by the existing best-effort fallback;
- if evidence still cannot be persisted, the charged usage remains auditable and any later claim routes to manual review;
- simple/degraded logging persists best-effort evidence but never auto-approves claims missing a verified link.

**Step 2: Run tests to verify failure**

Run: `cd backend && go test -tags unit ./internal/repository ./internal/service -run 'UsageLog.*Outcome|RecordUsage.*Outcome' -count=1`

Expected: FAIL.

**Step 3: Add the outcome snapshot to final usage logging**

Keep the existing billing transaction unchanged: response-evidence persistence must never decide whether a valid request is billed. Carry the outcome on `UsageLog`, commit the usage row first, then upsert `usage_response_outcomes` from server-derived fields. If outcome persistence fails, return a retryable logging error so `writeUsageLogBestEffort` retries against the already-deduplicated usage row. Resolve and link the existing row by `(request_id, api_key_id)` on retry.

**Step 4: Run billing tests**

Run: `cd backend && go test -tags unit ./internal/repository ./internal/service -run 'UsageLog.*Outcome|RecordUsage.*Outcome|UsageBilling' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/repository
git commit -m "feat: persist response evidence with usage billing"
```

### Task 6: Implement claim eligibility and automatic evaluation

**Files:**
- Create: `backend/internal/service/empty_response_claim.go`
- Create: `backend/internal/service/empty_response_claim_test.go`
- Create: `backend/internal/repository/empty_response_claim_repo.go`
- Create: `backend/internal/repository/empty_response_claim_repo_test.go`
- Modify: `backend/internal/service/errors.go`
- Modify: `backend/internal/repository/wire.go`

**Step 1: Write a decision-table test before implementation**

Cases must include:

- pure 200 empty response -> automatic approval;
- charged upstream 5xx/timeout/interruption -> automatic approval;
- client cancellation -> rejection;
- any valid text/tool/reasoning/media -> rejection;
- no charge, group disabled, older than 24 hours, or existing claim -> rejection;
- missing/conflicting evidence -> manual review;
- first 10 claims in the Asia/Shanghai business day use normal evaluation; claim 11 routes to manual review;
- historical usage rows without outcomes never auto-approve.

**Step 2: Run tests to verify failure**

Run: `cd backend && go test -tags unit ./internal/service ./internal/repository -run EmptyResponseClaim -count=1`

Expected: FAIL.

**Step 3: Implement a pure evaluator plus orchestration service**

Keep policy deterministic and versioned:

```go
type ClaimDecision struct {
    Status     string
    ReasonCode string
    RuleVersion int
}

func EvaluateEmptyResponseClaim(now time.Time, usage UsageLog, outcome *ResponseOutcome, group Group, dailyCount int) ClaimDecision
```

The orchestration service loads data server-side, checks ownership, creates the unique claim, and calls compensation only for automatic approval.

**Step 4: Run claim tests**

Run: `cd backend && go test -tags unit ./internal/service ./internal/repository -run EmptyResponseClaim -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/empty_response_claim* backend/internal/repository/empty_response_claim* backend/internal/service/errors.go backend/internal/repository/wire.go
git commit -m "feat: evaluate empty response claims"
```

### Task 7: Add the idempotent compensation ledger transaction

**Files:**
- Create: `backend/internal/service/empty_response_compensation.go`
- Create: `backend/internal/service/empty_response_compensation_test.go`
- Create: `backend/internal/repository/empty_response_compensation_repo.go`
- Create: `backend/internal/repository/empty_response_compensation_repo_unit_test.go`
- Create: `backend/internal/repository/empty_response_compensation_repo_integration_test.go`
- Modify: `backend/internal/service/billing_cache_service.go`
- Modify: `backend/internal/repository/wire.go`

**Step 1: Write failing transaction and concurrency tests**

Verify:

- balance billing adds exactly `actual_cost` back to `users.balance`;
- subscription billing decrements only active usage windows containing the original request time and clamps at zero;
- API-key cumulative quota is decremented by the original charged amount and clamps at zero;
- account/upstream quota and operator account cost are unchanged;
- `usage_logs.actual_cost` remains unchanged while `compensated_cost` is set once;
- claim row becomes `compensated` in the same transaction;
- two concurrent approvals produce one ledger effect;
- a forced mid-transaction failure changes neither balance nor claim status;
- cache invalidation occurs only after commit.

**Step 2: Run tests to verify failure**

Run: `cd backend && go test -tags unit ./internal/repository ./internal/service -run EmptyResponseCompensation -count=1`

Expected: FAIL.

**Step 3: Implement the transaction with row locks**

Lock claim, usage, user, API key, and subscription rows in a stable order. Update balance/quota with SQL arithmetic and constraints. Do not use the general admin balance-adjustment service. Return the affected user ID so the service can invalidate balance/auth/API-key caches after commit.

Do not reverse rate-limit windows: RPM/TPM windows represent actual traffic and should age out naturally. Do reverse the API key's cumulative dollar quota because it is a billing allowance.

**Step 4: Run transaction tests including integration mode**

Run: `cd backend && go test -tags unit ./internal/repository ./internal/service -run EmptyResponseCompensation -count=1`

If the integration database is configured, also run: `cd backend && go test ./internal/repository -run EmptyResponseCompensation -count=1`

Expected: PASS; the integration test may skip only when the repository's standard test database prerequisite is absent.

**Step 5: Commit**

```bash
git add backend/internal/service/empty_response_compensation* backend/internal/repository/empty_response_compensation* backend/internal/service/billing_cache_service.go backend/internal/repository/wire.go
git commit -m "feat: refund empty responses atomically"
```

### Task 8: Expose secure user and admin claim APIs

**Files:**
- Modify: `backend/internal/handler/usage_handler.go`
- Modify: `backend/internal/handler/admin/usage_handler.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Create: `backend/internal/handler/empty_response_claim_handler_test.go`
- Create: `backend/internal/handler/admin/empty_response_claim_handler_test.go`

**Step 1: Write failing authorization and contract tests**

Add contracts for:

- `POST /api/v1/usage/:id/empty-response-claim`;
- `GET /api/v1/admin/usage/empty-response-claims`;
- `POST /api/v1/admin/usage/empty-response-claims/:id/approve`;
- `POST /api/v1/admin/usage/empty-response-claims/:id/reject`;
- `POST /api/v1/admin/usage/empty-response-claims/batch`.

Assert users cannot claim another user's record, submit an amount, or view internal evidence. Assert admin actions require admin auth and a rejection reason. Put static admin claim routes before any `/:id` route to avoid Gin route ambiguity.

**Step 2: Run tests to verify failure**

Run: `cd backend && go test -tags unit ./internal/handler ./internal/server/routes -run EmptyResponseClaim -count=1`

Expected: FAIL.

**Step 3: Implement handlers, DTOs, routes, and dependency injection**

User DTOs expose eligibility, status, estimated refund, and a sanitized reason. Admin DTOs additionally expose outcome flags, upstream status/error kind, account/group identity, rule version, and review metadata. Never expose raw request/response content.

**Step 4: Run API tests**

Run: `cd backend && go test -tags unit ./internal/handler ./internal/server/routes -run EmptyResponseClaim -count=1`

Expected: PASS.

**Step 5: Regenerate server wiring and compile**

Run: `cd backend && go generate ./cmd/server && go build ./cmd/server`

Expected: PASS.

**Step 6: Commit**

```bash
git add backend/internal/handler backend/internal/server/routes backend/cmd/server
git commit -m "feat: expose empty response claim APIs"
```

### Task 9: Add group configuration end to end

**Files:**
- Modify: `backend/internal/service/group.go`
- Modify: `backend/internal/service/admin_group.go`
- Modify: `backend/internal/service/group_service.go`
- Modify: `backend/internal/repository/group_repo.go`
- Modify: `backend/internal/handler/admin/group_handler.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `backend/internal/service/admin_group_test.go`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/groups.ts`
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/views/admin/__tests__/GroupsView.spec.ts`

**Step 1: Write failing backend and frontend tests**

Assert create/update/list preserve `empty_response_compensation_enabled`, defaults remain false, duplication copies the setting deliberately, and the admin toggle is rendered with explanatory text.

**Step 2: Run tests to verify failure**

Run backend: `cd backend && go test -tags unit ./internal/service ./internal/handler/admin -run 'Group.*EmptyResponse' -count=1`

Run frontend: `cd frontend && pnpm vitest run src/views/admin/__tests__/GroupsView.spec.ts`

Expected: FAIL.

**Step 3: Implement mapping, API types, and the group form toggle**

Keep the toggle off by default. Place it near request/billing controls and label it as a user-refund policy, not an upstream refund guarantee.

**Step 4: Run focused tests**

Run backend and frontend commands from Step 2.

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service backend/internal/repository/group_repo.go backend/internal/handler frontend/src/types/index.ts frontend/src/api/admin/groups.ts frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/__tests__/GroupsView.spec.ts
git commit -m "feat: configure empty response compensation per group"
```

### Task 10: Show eligibility and net cost in user usage records

**Files:**
- Modify: `backend/internal/repository/usage_log_repo_query.go`
- Modify: `backend/internal/repository/usage_log_repo_stats.go`
- Modify: `backend/internal/repository/usage_log_repo_trend.go`
- Modify: `backend/internal/repository/usage_log_repo_dashboard.go`
- Modify: `backend/internal/repository/usage_log_repo_request_type_test.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/usage.ts`
- Modify: `frontend/src/views/user/UsageView.vue`
- Modify: `frontend/src/views/user/__tests__/UsageView.spec.ts`
- Create: `frontend/src/components/usage/EmptyResponseClaimDialog.vue`
- Create: `frontend/src/components/usage/__tests__/EmptyResponseClaimDialog.spec.ts`

**Step 1: Write failing repository and UI tests**

Require each user usage row to expose:

- `compensation_eligibility`;
- `claim_status` and sanitized reason;
- `compensated_cost` and `net_actual_cost`.

Require aggregate queries to return both gross and net values, with net defined as `GREATEST(actual_cost - compensated_cost, 0)`. Test the 24-hour boundary and mobile dialog layout.

**Step 2: Run tests to verify failure**

Run backend: `cd backend && go test -tags unit ./internal/repository ./internal/handler/dto -run 'Usage.*Compensat|NetActualCost' -count=1`

Run frontend: `cd frontend && pnpm vitest run src/views/user/__tests__/UsageView.spec.ts src/components/usage/__tests__/EmptyResponseClaimDialog.spec.ts`

Expected: FAIL.

**Step 3: Implement query projections and user UI**

Keep original `actual_cost` visible. Render original charge, refunded amount, and net charge. Show the action only when the backend marks it eligible; never recreate policy in TypeScript. After submission, update the single row in place without a full-table loading overlay.

On mobile, use a bottom-sheet/full-width dialog with model, time, original charge, expected refund, and rules. Do not add a new left navigation item.

**Step 4: Run focused tests and typecheck**

Run: `cd frontend && pnpm vitest run src/views/user/__tests__/UsageView.spec.ts src/components/usage/__tests__/EmptyResponseClaimDialog.spec.ts && pnpm vue-tsc -b`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/repository/usage_log_repo* backend/internal/handler/dto frontend/src/types/index.ts frontend/src/api/usage.ts frontend/src/views/user/UsageView.vue frontend/src/components/usage
git commit -m "feat: add user empty response claim workflow"
```

### Task 11: Add admin review workflow and operational metrics

**Files:**
- Modify: `frontend/src/api/admin/usage.ts`
- Modify: `frontend/src/views/admin/UsageView.vue`
- Modify: `frontend/src/views/admin/__tests__/UsageView.spec.ts`
- Create: `frontend/src/components/admin/usage/EmptyResponseClaimsPanel.vue`
- Create: `frontend/src/components/admin/usage/EmptyResponseClaimReviewDialog.vue`
- Create: `frontend/src/components/admin/usage/__tests__/EmptyResponseClaimsPanel.spec.ts`
- Create: `backend/internal/service/empty_response_claim_metrics.go`
- Create: `backend/internal/service/empty_response_claim_metrics_test.go`
- Modify: `backend/internal/service/ops_system_log_service.go`
- Modify: `backend/internal/service/ops_system_log_service_test.go`

**Step 1: Write failing UI and observability tests**

Cover status filters, evidence rendering, single approve/reject, batch actions, required rejection notes, mobile card rendering, structured audit events, and aggregated empty-response rate/refund amount by group/account/model.

**Step 2: Run tests to verify failure**

Run backend: `cd backend && go test -tags unit ./internal/service -run 'EmptyResponseClaimMetrics|EmptyResponseClaimAudit' -count=1`

Run frontend: `cd frontend && pnpm vitest run src/views/admin/__tests__/UsageView.spec.ts src/components/admin/usage/__tests__/EmptyResponseClaimsPanel.spec.ts`

Expected: FAIL.

**Step 3: Implement the existing-page admin tab**

Add an “空回申请” tab inside admin usage. Use cards on narrow screens and the existing table primitives on desktop. Show only structured evidence. Include account/group/model rankings and warning thresholds without exposing response content.

**Step 4: Run focused tests**

Run the commands from Step 2, then `cd frontend && pnpm vue-tsc -b`.

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/api/admin/usage.ts frontend/src/views/admin/UsageView.vue frontend/src/views/admin/__tests__/UsageView.spec.ts frontend/src/components/admin/usage backend/internal/service/empty_response_claim_metrics* backend/internal/service/ops_system_log_service*
git commit -m "feat: add empty response claim administration"
```

### Task 12: Add translations, privacy regression tests, and release gates

**Files:**
- Modify: `frontend/src/i18n/locales/zh/misc.ts`
- Modify: `frontend/src/i18n/locales/en/misc.ts`
- Modify: `frontend/src/i18n/__tests__/localeParity.spec.ts`
- Create: `backend/internal/service/response_outcome_privacy_test.go`
- Modify: `docs/plans/2026-08-07-empty-response-compensation-design.md` only if implementation discoveries require an approved clarification

**Step 1: Add failing locale and privacy tests**

Assert Chinese/English key parity and serialize every outcome/claim user DTO to prove fixture secrets, text, tool arguments, URLs, and base64 payloads are absent.

**Step 2: Run tests to verify failure**

Run backend: `cd backend && go test -tags unit ./internal/service ./internal/handler/dto -run 'ResponseOutcomePrivacy|EmptyResponse.*DTO' -count=1`

Run frontend: `cd frontend && pnpm vitest run src/i18n/__tests__/localeParity.spec.ts`

Expected: FAIL until translations/privacy guards are complete.

**Step 3: Add translations and close privacy leaks**

Use stable reason-code translations. Do not interpolate internal error bodies into user-visible messages.

**Step 4: Run the complete release verification**

```bash
cd backend
go test -tags unit ./internal/service ./internal/repository ./internal/handler ./internal/server/routes ./migrations -count=1
go build ./cmd/server

cd ../frontend
pnpm vitest run
pnpm vue-tsc -b
pnpm vite build

cd ..
git diff --check
```

Expected: all commands exit 0. If repository-wide pre-existing failures occur, record the exact failing test, prove it also fails on the `dev` baseline, and still require every new focused suite to pass.

**Step 5: Run migration and blue-green preflight**

- Audit duplicate outcome/claim keys before migration.
- Apply migration in an isolated/staging database first.
- Deploy with all group switches off.
- Verify outcome rows contain no content and billing latency does not regress materially.
- Enable one test group and exercise: normal content, tool-only, pure empty, upstream 5xx, upstream interruption, and client cancellation.
- Confirm balances/subscription counters/API-key quota, gross/net reporting, and audit logs.

**Step 6: Commit**

```bash
git add frontend/src/i18n backend/internal/service/response_outcome_privacy_test.go docs/plans/2026-08-07-empty-response-compensation-design.md
git commit -m "test: gate empty response compensation release"
```

## Integration workflow

1. Implement in `codex/empty-response-compensation`, always rebased/merged from `dev` as needed.
2. After all focused and full tests pass, merge the feature branch into `dev`.
3. Build and blue-green deploy the `dev` commit with all group switches disabled.
4. Enable only a dedicated test group and run the production smoke matrix.
5. After production verification, merge the exact deployed `dev` commit into `master`.
