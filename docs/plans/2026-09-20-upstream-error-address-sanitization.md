# 上游错误地址脱敏 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure upstream URLs and address fragments in error responses, passthrough responses, Ops details, and optional error logs are sanitized before leaving the service while preserving diagnostic status and fields.

**Architecture:** Extend the shared upstream error sanitizer to recursively sanitize JSON and plain text, then apply it at error-body output and storage/logging boundaries. Rule matching continues to use the original upstream body, while all returned messages and bodies use sanitized copies.

**Tech Stack:** Go, Gin, `encoding/json`, existing service tests with `testify`/`httptest`.

---

### Task 1: Add failing sanitizer and passthrough regression tests

**Files:**
- Modify: `backend/internal/service/upstream_error_sanitizer_test.go`
- Modify: `backend/internal/service/error_passthrough_runtime_test.go`
- Test target: `backend/internal/service`

**Step 1: Write the failing tests**

Add tests that assert:

- Nested JSON strings containing `https://provider.example:8443/v1/...`, bracketed domains/IPs, and sensitive query values lose the original address while preserving JSON fields.
- Plain-text bodies containing a URL are sanitized.
- A configured error passthrough rule can still match the original response body, but its returned message no longer contains the upstream URL.

**Step 2: Run tests to verify failure**

Run:

```bash
cd backend
go test ./internal/service -run 'Test(Sanitize|ErrorPassthrough)' -count=1
```

Expected: the new passthrough/body assertions fail because current rule output and some body paths still expose the original value.

**Step 3: Commit the tests**

```bash
git add backend/internal/service/upstream_error_sanitizer_test.go backend/internal/service/error_passthrough_runtime_test.go
git commit -m "test: cover upstream error address redaction boundaries"
```

### Task 2: Strengthen the shared sanitizer

**Files:**
- Modify: `backend/internal/service/upstream_error_sanitizer.go`
- Modify: `backend/internal/service/gemini_messages_compat_service.go` only if the shared helper is currently duplicated there
- Test: `backend/internal/service/upstream_error_sanitizer_test.go`

**Step 1: Implement the minimal sanitizer changes**

- Export a body-level helper for service boundary callers without duplicating regex logic.
- Keep recursive JSON traversal and plain-text fallback.
- Extend host matching only as needed for URL forms seen in upstream errors; preserve URL paths and redact query credentials.
- Keep existing `sanitizeUpstreamErrorMessage` callers behavior-compatible.

**Step 2: Run targeted tests**

```bash
cd backend
go test ./internal/service -run 'TestSanitize|TestForwardEmbeddings' -count=1
```

Expected: all sanitizer tests pass.

### Task 3: Sanitize error passthrough messages and raw bodies

**Files:**
- Modify: `backend/internal/service/error_passthrough_runtime.go`
- Modify: `backend/internal/service/gateway_upstream_response.go`
- Modify: `backend/internal/service/openai_gateway_upstream_errors.go`
- Modify: `backend/internal/service/gemini_messages_compat_service.go`
- Modify: `backend/internal/service/openai_alpha_search.go`
- Modify: `backend/internal/service/gateway_anthropic_passthrough.go`
- Modify: other direct error-body writers identified by `rg` during implementation
- Tests: relevant service tests plus new boundary tests

**Step 1: Write failing boundary tests**

Cover representative paths:

- generic gateway 400 body;
- Gemini native upstream error body;
- OpenAI cyber-policy/raw body path;
- a search/direct passthrough non-2xx body.

Each test must assert the response retains status and JSON shape but does not contain the original upstream host.

**Step 2: Run tests to verify failure**

Run the focused test names and confirm they fail on the original host.

**Step 3: Implement one boundary change at a time**

- Build a sanitized body immediately after reading an upstream error body and before `c.Data`/raw response writes.
- Pass sanitized body to `applyErrorPassthroughRule` output only after matching has completed; do not alter rule matching semantics.
- Sanitize cyber-policy and other intentional raw passthrough bodies while preserving their protocol-specific envelope and status.
- Ensure any returned extracted message is passed through the shared message sanitizer.

**Step 4: Run focused tests**

```bash
cd backend
go test ./internal/service -run 'Test(Sanitize|ForwardEmbeddings|.*Passthrough|.*UpstreamError)' -count=1
```

Expected: all boundary tests pass.

### Task 4: Sanitize Ops storage and optional error logs

**Files:**
- Modify: `backend/internal/service/ops_service.go`
- Modify: `backend/internal/handler/ops_error_logger.go` only where raw body is queued before service storage
- Tests: `backend/internal/service/ops_user_error_test.go` or a new focused Ops sanitizer test file

**Step 1: Write failing storage tests**

Assert that JSON error bodies and upstream event details stored through the Ops preparation path no longer contain the original URL, while credentials remain redacted and payload bounds remain enforced.

**Step 2: Run tests to verify failure**

```bash
cd backend
go test ./internal/service -run 'Test.*Ops|Test.*ErrorBody' -count=1
```

Expected: stored JSON still contains the original URL before the implementation change.

**Step 3: Implement storage/log sanitization**

- Run the shared body sanitizer before `redactSensitiveJSON` and truncation.
- Ensure queue helpers and persistence preparation share the same path.
- Replace raw body strings used by optional upstream error logging with sanitized copies; do not change request/response success logging.

**Step 4: Run focused tests**

```bash
cd backend
go test ./internal/service ./internal/handler -run 'Test.*(Ops|ErrorBody|UpstreamError)' -count=1
```

Expected: all focused tests pass.

### Task 5: Full verification and review

**Files:**
- No new production files expected.
- Review: all files changed in Tasks 1–4.

**Step 1: Run the complete backend service tests**

```bash
cd backend
go test ./internal/service ./internal/handler
```

Expected: PASS.

**Step 2: Check formatting and diff hygiene**

```bash
gofmt -w <changed Go files>
git diff --check
git status --short
```

Expected: no formatting or whitespace errors; unrelated pre-existing modifications remain untouched.

**Step 3: Review data-flow invariants**

Confirm that:

- only error paths sanitize bodies;
- status/type/code/param/request ID are preserved;
- passthrough rule matching still sees the original body;
- no success body or billing/failover logic changed.

**Step 4: Commit the implementation**

```bash
git add <changed files>
git commit -m "fix: sanitize upstream addresses in error responses"
```
