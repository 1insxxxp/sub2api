# OpenAI-Compatible Stream Mode Relay Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add opt-in OpenAI-compatible base URL aliases that force downstream stream or non-stream mode.

**Architecture:** The route layer marks requests from `/relay-stream/v1` or `/relay-nonstream/v1` in `gin.Context`. OpenAI-compatible Chat Completions and Responses handlers rewrite only the top-level `stream` field after JSON parsing and before stream validation, audit, routing, and billing.

**Tech Stack:** Go, Gin, tidwall/gjson/sjson, existing Sub2API OpenAI-compatible gateway handlers and route middleware.

---

### Task 1: Add Relay Mode Helper

**Files:**
- Create: `backend/internal/handler/relay_stream_mode.go`
- Test: `backend/internal/handler/relay_stream_mode_test.go`

**Step 1: Write failing helper tests**

Cover:
- `ForceStream` sets missing `stream` to `true`.
- `ForceStream` converts `stream:false` to `true`.
- `ForceNonStream` converts `stream:true` to `false`.
- Normal mode leaves the body unchanged.
- Invalid JSON returns an error.

**Step 2: Run helper tests and verify failure**

Run:

```bash
go test ./internal/handler -run TestApplyOpenAICompatibleRelayStreamMode -count=1
```

Expected: FAIL because helper does not exist.

**Step 3: Implement helper**

Create:

- `type openAICompatibleRelayStreamMode string`
- constants `normal`, `force_stream`, `force_nonstream`
- `markOpenAICompatibleRelayStreamMode(c, mode)`
- `openAICompatibleRelayStreamModeFromContext(c)`
- `applyOpenAICompatibleRelayStreamMode(c, body) ([]byte, bool, error)`

Use structured JSON (`sjson`) to set top-level `stream`.

**Step 4: Run helper tests and verify pass**

Run:

```bash
go test ./internal/handler -run TestApplyOpenAICompatibleRelayStreamMode -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/relay_stream_mode.go backend/internal/handler/relay_stream_mode_test.go
git commit -m "feat: add OpenAI relay stream mode helper"
```

### Task 2: Apply Mode in OpenAI-Compatible Handlers

**Files:**
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Test: `backend/internal/handler/openai_stream_validation_test.go` or focused new tests.

**Step 1: Write failing handler tests**

Add tests that call the existing handlers with a context marked by relay mode and assert the observed request stream mode changes before validation.

Preferred tests:
- Chat Completions force-stream accepts `{"stream":false}` and proceeds as stream.
- Chat Completions force-nonstream accepts `{"stream":true}` and proceeds as sync.
- Responses force-stream accepts `{"stream":false}` and proceeds as stream.
- Unmarked normal Chat Completions keeps existing invalid stream validation behavior.

Use lightweight handler test scaffolding already present in `openai_stream_validation_test.go` where possible.

**Step 2: Run handler tests and verify failure**

Run:

```bash
go test ./internal/handler -run 'TestOpenAICompatibleRelayStreamMode|TestOpenAICompatibleHandlers' -count=1
```

Expected: FAIL because handlers do not apply the relay mode.

**Step 3: Implement handler integration**

In `ChatCompletions`, after `gjson.ValidBytes(body)` and before `parseOpenAICompatibleStream(body)`, call:

```go
if rewritten, changed, err := applyOpenAICompatibleRelayStreamMode(c, body); err != nil {
    h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
    return
} else if changed {
    body = rewritten
}
```

Apply the same pattern in `Responses` after compact normalization and before stream parsing.

**Step 4: Run handler tests and verify pass**

Run:

```bash
go test ./internal/handler -run 'TestOpenAICompatibleRelayStreamMode|TestOpenAICompatibleHandlers' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/openai_chat_completions.go backend/internal/handler/openai_gateway_handler.go backend/internal/handler/*relay*test.go
git commit -m "feat: apply OpenAI relay stream mode"
```

### Task 3: Register Public Alias Routes

**Files:**
- Modify: `backend/internal/server/routes/gateway.go`
- Test: `backend/internal/server/routes/gateway_test.go` or a focused route test file.

**Step 1: Write failing route tests**

Test that:
- `POST /relay-stream/v1/chat/completions` reaches the same OpenAI-compatible handler chain with relay mode set to force stream.
- `POST /relay-nonstream/v1/chat/completions` reaches the same chain with relay mode set to force non-stream.
- `POST /relay-stream/v1/responses` and `POST /relay-nonstream/v1/responses` are registered.

Prefer a route-level test that installs a small Gin handler chain and asserts the context mode when the route is hit.

**Step 2: Run route tests and verify failure**

Run:

```bash
go test ./internal/server/routes -run TestOpenAICompatibleRelayStreamModeRoutes -count=1
```

Expected: FAIL because routes are not registered.

**Step 3: Implement routes**

In `RegisterGatewayRoutes`, factor the existing `/v1` OpenAI-compatible conversation registration into a small helper if practical, or add two route groups:

- `/relay-stream/v1`
- `/relay-nonstream/v1`

Each group uses the same core middleware chain as `/v1` for body limit, client request ID, ops logging, endpoint normalization, API key auth, system/custom/composite target selection, and platform guard. Add a tiny middleware before handlers:

```go
func markRelay(mode openAICompatibleRelayStreamMode) gin.HandlerFunc {
    return func(c *gin.Context) {
        markOpenAICompatibleRelayStreamMode(c, mode)
        c.Next()
    }
}
```

Register at least:

- `POST /chat/completions`
- `POST /responses`
- `POST /responses/*subpath`
- `GET /models`

**Step 4: Run route tests and verify pass**

Run:

```bash
go test ./internal/server/routes -run TestOpenAICompatibleRelayStreamModeRoutes -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/server/routes/gateway.go backend/internal/server/routes/*relay*test.go
git commit -m "feat: register OpenAI relay stream mode routes"
```

### Task 4: Full Verification and Deployment Prep

**Files:**
- Modify if needed: docs/admin or release notes only if route naming changes.

**Step 1: Run focused backend tests**

```bash
go test ./internal/handler ./internal/server/routes -count=1
```

Expected: PASS.

**Step 2: Run formatting and diff checks**

```bash
gofmt -w backend/internal/handler/relay_stream_mode.go backend/internal/handler/relay_stream_mode_test.go backend/internal/handler/openai_chat_completions.go backend/internal/handler/openai_gateway_handler.go backend/internal/server/routes/gateway.go
git diff --check
```

Expected: no output from `git diff --check`.

**Step 3: Run a lightweight production-style smoke test locally or against the staging container**

Use a non-secret test key if available:

```bash
curl -N https://api.passionapi.com/relay-stream/v1/chat/completions \
  -H 'Authorization: Bearer <redacted>' \
  -H 'Content-Type: application/json' \
  -d '{"model":"次[0.03/次]gemini-3.1-pro-preview","stream":false,"messages":[{"role":"user","content":"hi"}]}'
```

Expected: SSE response instead of one JSON object.

**Step 4: Final commit**

```bash
git status --short
git log --oneline -5
```

Expected: only intentional tracked changes committed; pre-existing untracked files remain untouched.

**Step 5: Deploy**

After commits are pushed to `origin/dev`, build the production image and use the existing blue-green procedure. Verify:

- `https://api.passionapi.com/health`
- `https://api.passionapi.com/relay-stream/v1/models` with a valid key, if route is registered for models.
- A Chat Completions smoke request returns streamed data.
