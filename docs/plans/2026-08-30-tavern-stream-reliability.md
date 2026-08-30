# Tavern Stream Reliability Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent SillyTavern Gemini requests from being reported as successful when Sub2 receives an empty or prematurely terminated upstream response.

**Architecture:** Harden the Gemini-to-OpenAI Chat Completions compatibility path at the protocol boundary. Delay stream commitment until useful output exists, require a real Gemini terminal reason before `[DONE]`, fail over empty pre-commit responses, and stop retries after downstream cancellation. Preserve the deployed reasoning customization and use the existing gateway failover mechanism.

**Tech Stack:** Go 1.27, Gin, OpenAI-compatible SSE, Gemini GenerateContent/SSE, Docker, Nginx.

---

### Task 1: Stop retries after request cancellation

**Files:**
- Modify: `backend/internal/service/gemini_chat_completions_compat_service.go:121-151`
- Test: `backend/internal/service/gemini_messages_compat_service_test.go`

**Step 1: Write the failing test**

Add a transport stub that returns `context.Canceled` and assert that
`ForwardAsChatCompletions` calls it exactly once and returns a cancellation
error.

**Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/service -run TestGeminiForwardAsChatCompletions_CanceledTransportDoesNotRetry -count=1`

Expected: FAIL because the current loop retries five times.

**Step 3: Implement the cancellation guard**

After `httpUpstream.Do` returns an error, return immediately when either the
error or `ctx.Err()` is `context.Canceled` or `context.DeadlineExceeded`.

**Step 4: Run the focused test to verify it passes**

Run: `go test ./internal/service -run TestGeminiForwardAsChatCompletions_CanceledTransportDoesNotRetry -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/gemini_chat_completions_compat_service.go backend/internal/service/gemini_messages_compat_service_test.go
git commit -m "fix: stop gemini retries after cancellation"
```

### Task 2: Normalize assistant-prefill requests

**Files:**
- Modify: `backend/internal/service/gemini_messages_compat_service.go:3388-3520`
- Test: `backend/internal/service/gemini_messages_compat_service_test.go`

**Step 1: Write the failing test**

Convert an Anthropic request whose last message is `assistant` and assert that
the final Gemini content has role `user` with the continuation instruction,
while the assistant prefill remains the preceding `model` turn.

**Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/service -run TestConvertClaudeMessagesToGeminiGenerateContent_AppendsContinuationAfterAssistantPrefill -count=1`

Expected: FAIL because the converted request currently ends in `model`.

**Step 3: Implement the minimal normalization**

After converting all messages, inspect the last valid content role. When it is
`model`, append:

```go
map[string]any{
    "role": "user",
    "parts": []any{map[string]any{
        "text": "Continue the assistant response from where it stopped without repeating prior text.",
    }},
}
```

Do not append anything when the request already ends in `user`.

**Step 4: Run conversion tests**

Run: `go test ./internal/service -run 'TestConvertClaudeMessagesToGeminiGenerateContent_(AppendsContinuationAfterAssistantPrefill|AddsThoughtSignatureForToolUse)' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/gemini_messages_compat_service.go backend/internal/service/gemini_messages_compat_service_test.go
git commit -m "fix: support gemini assistant prefill turns"
```

### Task 3: Reject empty and premature streaming success

**Files:**
- Modify: `backend/internal/service/gemini_chat_completions_compat_service.go:514-812`
- Test: `backend/internal/service/gemini_messages_compat_service_test.go`

**Step 1: Write failing stream tests**

Add three tests:

- EOF before output returns `UpstreamFailoverError` and does not write HTTP 200.
- EOF after partial text writes a structured `incomplete_stream` SSE error and
  does not write `[DONE]`.
- A normal stream with `finishReason: STOP` still emits content and `[DONE]`.

**Step 2: Run the stream tests to verify failure**

Run: `go test ./internal/service -run 'TestGeminiForwardAsChatCompletions_Stream(PrematureEOF|EmptyTerminal|Complete)' -count=1`

Expected: premature and empty tests FAIL because current code always sends a
normal stop sequence.

**Step 3: Implement terminal-aware streaming**

- Delay response headers and `message_start` until the first visible text or
  tool call.
- Track `streamStarted`, `sawVisibleOutput`, and non-empty `finishReason`.
- Before headers, return `UpstreamFailoverError{StatusCode: 502}` for EOF without
  a terminal reason or for terminal output with no visible content.
- After partial output, emit an OpenAI error object with code
  `incomplete_stream`, flush, and return an error without stop events or `[DONE]`.
- Preserve the current complete-stream conversion and usage accounting.

**Step 4: Run focused and existing stream tests**

Run: `go test ./internal/service -run 'TestGeminiForwardAsChatCompletions_(Stream|Streams)' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/gemini_chat_completions_compat_service.go backend/internal/service/gemini_messages_compat_service_test.go
git commit -m "fix: detect incomplete gemini chat streams"
```

### Task 4: Fail over empty non-streaming responses

**Files:**
- Modify: `backend/internal/service/gemini_chat_completions_compat_service.go:266-300,460-512`
- Test: `backend/internal/service/gemini_messages_compat_service_test.go`

**Step 1: Write failing tests**

Cover both JSON `generateContent` and collected OAuth SSE responses that contain
a terminal reason and usage but no visible text, media, or tool call. Assert an
`UpstreamFailoverError` and an uncommitted recorder.

**Step 2: Run tests to verify failure**

Run: `go test ./internal/service -run TestGeminiForwardAsChatCompletions_EmptyNonStreamingFailsOver -count=1`

Expected: FAIL because current code writes HTTP 200.

**Step 3: Add a visible-output guard**

Inspect the converted Chat Completions choices for non-empty content or tool
calls before `c.JSON`. Return a 502 `UpstreamFailoverError` when no visible
output exists.

**Step 4: Run focused tests**

Run: `go test ./internal/service -run 'TestGeminiForwardAsChatCompletions_(EmptyNonStreamingFailsOver|OAuthRoutesToGeminiAndReturnsChatFormat)' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/gemini_chat_completions_compat_service.go backend/internal/service/gemini_messages_compat_service_test.go
git commit -m "fix: fail over empty gemini chat responses"
```

### Task 5: Preserve production customization and run regression tests

**Files:**
- Modify: `backend/internal/service/gemini_chat_completions_compat_service.go:209`

**Step 1: Preserve the deployed reasoning call**

Keep `extractCCReasoningEffortFromBody(originalChatBody, mappedModel)` exactly as
deployed on production.

**Step 2: Format and run focused tests**

Run: `gofmt -w internal/service/gemini_chat_completions_compat_service.go internal/service/gemini_messages_compat_service.go internal/service/gemini_messages_compat_service_test.go`

Run: `go test ./internal/service -run 'Gemini.*(ChatCompletions|AssistantPrefill|Premature|Empty)' -count=1`

Expected: PASS.

**Step 3: Run the full backend unit suite**

Run: `go test ./internal/service ./internal/handler/... -count=1`

Expected: PASS.

**Step 4: Commit**

```bash
git add backend/internal/service
git commit -m "test: cover tavern gemini response reliability"
```

### Task 6: Deploy and verify

**Files:**
- Remote modify: Sub2 Nginx virtual host configuration
- Remote deploy: `/opt/sub2api-custom-prod`

**Step 1: Back up runtime state**

Record the current image tag, container inspect output, Nginx config, and recent
response-outcome counts. Tag the existing image for rollback.

**Step 2: Stage the exact patch**

Deploy only the tested service files to the source tree. Confirm the staged diff
contains the reliability changes plus the pre-existing reasoning customization.

**Step 3: Build a versioned image**

Build a new `sub2api-custom` image with a timestamped reliability suffix. Do not
overwrite the rollback image.

**Step 4: Update Nginx timeouts**

Change stream-related read/send timeouts from `300s` to `900s`, run
`nginx -t`, then reload Nginx.

**Step 5: Restart and smoke test**

Start the new image, wait for Docker health, and test:

- streaming request through `direct.passionapi.com`;
- streaming request through `api.passionapi.com`;
- non-streaming request;
- assistant-prefill request;
- intentional client cancellation.

Expected: valid requests complete with `[DONE]`, assistant-prefill no longer
returns the model-turn 400, and cancellations generate no retry loop.

**Step 6: Observe outcomes**

Compare incomplete, empty, disconnect, 400, 499, and 502 outcomes after deploy.
Rollback immediately if health checks fail or normal completion rate regresses.
