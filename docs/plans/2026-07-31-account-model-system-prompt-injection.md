# Account Model System Prompt Injection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow an administrator to configure one plain-text system prompt per real upstream model on an account and prepend it exactly once to matching OpenAI, Claude, and Gemini text requests.

**Architecture:** Persist `model_system_prompts` as a dedicated JSONB account field and carry it through existing account/scheduler snapshots. A shared pure transformer matches the mapped upstream model and prepends protocol-specific system content; each text gateway invokes it after account/model resolution and before constructing the final wire body.

**Tech Stack:** Go 1.24, Ent, PostgreSQL JSONB, Gin, gjson/sjson, Vue 3, TypeScript, Vitest, vue-i18n.

---

### Task 1: Persist model prompt rules on accounts

**Files:**
- Create: `backend/migrations/191_account_model_system_prompts.sql`
- Modify: `backend/ent/schema/account.go`
- Modify (generated): `backend/ent/**`
- Modify: `backend/internal/service/account.go`
- Modify: `backend/internal/repository/account_repo.go`
- Test: `backend/internal/repository/account_repo_integration_test.go`
- Test: `backend/internal/repository/migrations_schema_integration_test.go`

**Step 1: Write the failing persistence tests**

Create an account with `ModelSystemPrompts: map[string]string{"gpt-5.4": "Stay in character."}`, read it back, replace it with a Claude rule, and assert exact round-trip equality. Extend the migration schema test to assert that `accounts.model_system_prompts` is non-null JSONB with `{}` as its default.

**Step 2: Run tests and verify failure**

Run: `cd backend && go test ./internal/repository -run 'TestAccountRepository_ModelSystemPrompts|TestMigrationSchema' -count=1`

Expected: FAIL because the field and column do not exist.

**Step 3: Add the migration and Ent field**

Use:

```sql
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS model_system_prompts JSONB NOT NULL DEFAULT '{}'::jsonb;
```

Add this Ent field:

```go
field.JSON("model_system_prompts", map[string]string{}).
    Default(func() map[string]string { return map[string]string{} }).
    SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
```

Run `cd backend && go generate ./ent`. Add `ModelSystemPrompts map[string]string` to `service.Account`. Update repository create/update/entity conversion to persist it and deep-copy maps so cached account objects do not share mutable state.

**Step 4: Run tests and verify success**

Run: `cd backend && go test ./internal/repository -run 'TestAccountRepository_ModelSystemPrompts|TestMigrationSchema' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/migrations/191_account_model_system_prompts.sql backend/ent backend/internal/service/account.go backend/internal/repository/account_repo.go backend/internal/repository/account_repo_integration_test.go backend/internal/repository/migrations_schema_integration_test.go
git commit -m "feat: persist account model system prompts"
```

### Task 2: Validate and expose admin account configuration

**Files:**
- Create: `backend/internal/service/account_model_system_prompts.go`
- Create: `backend/internal/service/account_model_system_prompts_test.go`
- Modify: `backend/internal/service/admin_service.go`
- Modify: `backend/internal/service/admin_account.go`
- Modify: `backend/internal/handler/admin/account_handler.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Test: `backend/internal/handler/admin/account_handler_test.go`
- Test: `backend/internal/handler/dto/mappers_test.go`

**Step 1: Write failing validation and DTO tests**

Test trimming, normalized duplicate model names, empty model/prompt rejection, the 32 KiB prompt limit, create round-trip, omitted update preservation, explicit `{}` clearing, and admin DTO exposure.

```go
func TestNormalizeModelSystemPrompts(t *testing.T) {
    got, err := NormalizeModelSystemPrompts(map[string]string{
        "  gpt-5.4  ": "  Stay in character.  ",
    })
    require.NoError(t, err)
    require.Equal(t, map[string]string{"gpt-5.4": "Stay in character."}, got)
}
```

**Step 2: Run tests and verify failure**

Run: `cd backend && go test ./internal/service ./internal/handler/admin ./internal/handler/dto -run 'ModelSystemPrompt' -count=1`

Expected: FAIL because validation and DTO fields are absent.

**Step 3: Implement validation and API plumbing**

Implement:

```go
const MaxModelSystemPromptBytes = 32 * 1024

func NormalizeModelSystemPrompts(in map[string]string) (map[string]string, error)
```

Add `ModelSystemPrompts map[string]string` to create inputs and `ModelSystemPrompts *map[string]string` to update inputs so nil means omitted and an empty map means clear. Add `model_system_prompts` to admin HTTP request/response DTOs and `AccountFromServiceShallow`. Normalize before persistence and ensure ordinary user DTOs do not expose the field.

**Step 4: Run tests and verify success**

Run: `cd backend && go test ./internal/service ./internal/handler/admin ./internal/handler/dto -run 'ModelSystemPrompt' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/account_model_system_prompts.go backend/internal/service/account_model_system_prompts_test.go backend/internal/service/admin_service.go backend/internal/service/admin_account.go backend/internal/handler/admin/account_handler.go backend/internal/handler/admin/account_handler_test.go backend/internal/handler/dto/types.go backend/internal/handler/dto/mappers.go backend/internal/handler/dto/mappers_test.go
git commit -m "feat: manage account model prompt rules"
```

### Task 3: Build the shared prepend transformer

**Files:**
- Create: `backend/internal/service/model_system_prompt_injection.go`
- Create: `backend/internal/service/model_system_prompt_injection_test.go`

**Step 1: Write failing table-driven tests**

Cover OpenAI Responses, Chat Completions, Claude string/block system fields, and Gemini with/without `systemInstruction.parts`. Assert unrelated JSON fields survive and existing system content follows the injected content. Include no-op tests for nil account, missing rule, whitespace prompt, and non-text protocol.

**Step 2: Run tests and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestPrependModelSystemPrompt|TestResolveModelSystemPrompt' -count=1`

Expected: FAIL because the transformer does not exist.

**Step 3: Implement the pure API**

```go
type ModelSystemPromptProtocol string

const (
    ModelSystemPromptOpenAIResponses ModelSystemPromptProtocol = "openai_responses"
    ModelSystemPromptOpenAIChat      ModelSystemPromptProtocol = "openai_chat"
    ModelSystemPromptClaude          ModelSystemPromptProtocol = "claude"
    ModelSystemPromptGemini          ModelSystemPromptProtocol = "gemini"
)

func (a *Account) ResolveModelSystemPrompt(mappedModel string) (string, bool)

func PrependModelSystemPrompt(body []byte, protocol ModelSystemPromptProtocol, prompt string) ([]byte, error)
```

Responses concatenates `prompt + "\n\n" + instructions`; Chat inserts a system message at index zero; Claude concatenates a string or inserts a text block; Gemini inserts a text part at `systemInstruction.parts[0]`. Return errors for malformed/incompatible JSON and never log prompt text.

**Step 4: Run tests and verify success**

Run: `cd backend && go test ./internal/service -run 'TestPrependModelSystemPrompt|TestResolveModelSystemPrompt' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/model_system_prompt_injection.go backend/internal/service/model_system_prompt_injection_test.go
git commit -m "feat: add model system prompt transformer"
```

### Task 4: Integrate Claude account forwarding

**Files:**
- Modify: `backend/internal/service/gateway_forward.go`
- Modify: `backend/internal/service/gateway_anthropic_passthrough.go`
- Test: `backend/internal/service/gateway_service_test.go`
- Test: `backend/internal/service/gateway_anthropic_apikey_passthrough_test.go`

**Step 1: Write failing forwarding tests**

Prove the rule matches the mapped model rather than the client alias, the injected block precedes existing Claude system content, another account is unaffected, passthrough and normal paths both work, and same-account retries contain one marker.

**Step 2: Run tests and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestGateway.*ModelSystemPrompt|TestAnthropic.*ModelSystemPrompt' -count=1`

Expected: FAIL.

**Step 3: Apply injection after Claude model mapping**

In `gateway_forward.go`, inject after `reqModel` becomes the mapped model and before cache/filter/upstream request construction. Synchronize `body` and `ParsedRequest`. In `gateway_anthropic_passthrough.go`, inject into `input.Body` before the retry loop using `input.RequestModel`, then update `input.Parsed`. Reuse the injected body on same-account retries.

**Step 4: Run tests and verify success**

Run: `cd backend && go test ./internal/service -run 'TestGateway.*ModelSystemPrompt|TestAnthropic.*ModelSystemPrompt' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/gateway_forward.go backend/internal/service/gateway_anthropic_passthrough.go backend/internal/service/gateway_service_test.go backend/internal/service/gateway_anthropic_apikey_passthrough_test.go
git commit -m "feat: inject prompts for Claude accounts"
```

### Task 5: Integrate OpenAI Responses and Chat Completions

**Files:**
- Modify: `backend/internal/service/openai_gateway_forward.go`
- Modify: `backend/internal/service/openai_gateway_passthrough.go`
- Modify: `backend/internal/service/openai_gateway_chat_completions.go`
- Modify: `backend/internal/service/openai_gateway_chat_completions_raw.go`
- Modify: `backend/internal/service/openai_gateway_messages.go`
- Modify: `backend/internal/service/openai_gateway_messages_chat_fallback.go`
- Test: `backend/internal/service/openai_gateway_service_test.go`
- Test: `backend/internal/service/openai_gateway_chat_completions_test.go`
- Test: `backend/internal/service/openai_gateway_chat_completions_raw_test.go`

**Step 1: Write a failing route-matrix test**

Cover `/v1/responses` normal/passthrough, `/v1/chat/completions` Responses-conversion/raw, and `/v1/messages` Responses-conversion/raw fallback. Assert final `upstreamModel` matching, prepend ordering, exactly one marker in the wire body, and no injection for explicit image generation.

**Step 2: Run tests and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestOpenAI.*ModelSystemPrompt' -count=1`

Expected: FAIL.

**Step 3: Integrate at post-mapping boundaries**

Add a small OpenAI method that accepts account, final upstream model, body protocol, and text/non-text intent. Resolve and apply the prompt only for text intent. For Chat/Claude compatibility routes, inject once before conversion so existing converters carry the system content forward. For direct Responses inject `instructions`; for raw Chat inject a system message. Apply before retry loops and preserve a clean source body for account failover.

**Step 4: Run tests and verify success**

Run: `cd backend && go test ./internal/service -run 'TestOpenAI.*ModelSystemPrompt' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/openai_gateway_forward.go backend/internal/service/openai_gateway_passthrough.go backend/internal/service/openai_gateway_chat_completions.go backend/internal/service/openai_gateway_chat_completions_raw.go backend/internal/service/openai_gateway_messages.go backend/internal/service/openai_gateway_messages_chat_fallback.go backend/internal/service/openai_gateway_service_test.go backend/internal/service/openai_gateway_chat_completions_test.go backend/internal/service/openai_gateway_chat_completions_raw_test.go
git commit -m "feat: inject prompts for OpenAI accounts"
```

### Task 6: Integrate Gemini native and compatibility forwarding

**Files:**
- Modify: `backend/internal/service/gemini_messages_compat_service.go`
- Modify: `backend/internal/service/gemini_chat_completions_compat_service.go`
- Test: `backend/internal/service/gemini_messages_compat_service_test.go`
- Test: `backend/internal/service/gemini_chat_completions_compat_service_test.go`

**Step 1: Write failing Gemini tests**

Test native GenerateContent, Claude Messages compatibility, and Chat Completions compatibility. Cover API Key/service-account mapping, OAuth model identity, an existing `systemInstruction.parts`, no matching rule, and one-marker retry behavior.

**Step 2: Run tests and verify failure**

Run: `cd backend && go test ./internal/service -run 'TestGemini.*ModelSystemPrompt' -count=1`

Expected: FAIL.

**Step 3: Apply Gemini prompt after mapping**

In `Forward`, `ForwardAsChatCompletions`, and `ForwardNative`, resolve the final `mappedModel`. Compatibility routes may inject before conversion or immediately after conversion, but use one location per route and assert only one part is added. Native requests prepend directly to `systemInstruction.parts`. Perform injection before retry/request-builder closures capture the body.

**Step 4: Run tests and verify success**

Run: `cd backend && go test ./internal/service -run 'TestGemini.*ModelSystemPrompt' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/gemini_messages_compat_service.go backend/internal/service/gemini_chat_completions_compat_service.go backend/internal/service/gemini_messages_compat_service_test.go backend/internal/service/gemini_chat_completions_compat_service_test.go
git commit -m "feat: inject prompts for Gemini accounts"
```

### Task 7: Add account editor controls

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/components/account/EditAccountModal.vue`
- Modify: `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/en/admin/accounts.ts`

**Step 1: Write failing UI tests**

Mount OpenAI, Anthropic, and Gemini accounts and assert the section appears. Test load/add/remove, blank/duplicate/overlength validation, and submission of `model_system_prompts`. Deleting all rows must submit `{}` rather than omit the field.

**Step 2: Run tests and verify failure**

Run: `cd frontend && pnpm vitest run src/components/account/__tests__/EditAccountModal.spec.ts`

Expected: FAIL.

**Step 3: Implement the editor**

Add `model_system_prompts?: Record<string, string>` to relevant frontend account/request types. In `EditAccountModal.vue`, maintain rows with stable local IDs:

```ts
interface ModelSystemPromptRow {
  id: string
  model: string
  prompt: string
}
```

Render a real-model input, multiline prompt, remove button, and add button in a platform-neutral section. Convert map-to-rows on load and rows-to-trimmed-map on submit. Always set `updatePayload.model_system_prompts`. Add Chinese/English guidance that matching uses the mapped real upstream model and merge mode is fixed to prepend.

**Step 4: Run tests and type check**

Run: `cd frontend && pnpm vitest run src/components/account/__tests__/EditAccountModal.spec.ts && pnpm type-check`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/types/index.ts frontend/src/components/account/EditAccountModal.vue frontend/src/components/account/__tests__/EditAccountModal.spec.ts frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/i18n/locales/en/admin/accounts.ts
git commit -m "feat: configure account model prompts in admin"
```

### Task 8: Verify failover, privacy, and regressions

**Files:**
- Modify: tests introduced or extended in Tasks 2–7

**Step 1: Add cross-cutting regression tests**

Assert same-account retries contain one marker; account A to B failover rebuilds from a clean request and applies only B's rule; no-rule accounts inherit nothing; non-text endpoints never inject; only admin DTOs expose configuration; logs and errors never include prompt text.

**Step 2: Run focused suites**

Run: `cd backend && go test ./internal/repository ./internal/handler/admin ./internal/handler/dto ./internal/service -count=1`

Run: `cd frontend && pnpm vitest run src/components/account/__tests__/EditAccountModal.spec.ts && pnpm type-check`

Expected: PASS.

**Step 3: Format and run full verification**

Run: `cd backend && gofmt -w internal/service/account_model_system_prompts.go internal/service/account_model_system_prompts_test.go internal/service/model_system_prompt_injection.go internal/service/model_system_prompt_injection_test.go && go test ./... -count=1`

Run: `cd frontend && pnpm build`

Expected: all commands exit 0.

**Step 4: Inspect the final state**

Run: `git diff --check && git status --short && git log --oneline -8`

Expected: no whitespace errors, only intended files changed, and migration 191 follows the current highest migration number.

**Step 5: Commit final regression adjustments**

```bash
git add backend/internal/service backend/internal/handler/dto frontend/src/components/account/__tests__/EditAccountModal.spec.ts
git commit -m "test: cover account model prompt injection"
```
