# Client Truncation Billing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Cancel upstream generation on downstream truncation and charge disconnected streams by successfully delivered output tokens.

**Architecture:** Extend the downstream output collector to own a cancellation callback tied to the upstream request context. Use a billing-only usage snapshot for disconnected streams so customer charges align with delivered output while provider usage remains available for diagnostics and account cost accounting.

**Tech Stack:** Go, Gin, HTTP/SSE and WebSocket gateways, testify-based unit tests.

---

### Task 1: Propagate downstream cancellation upstream

**Files:**
- Modify: `backend/internal/service/downstream_output_tokens.go`
- Test: `backend/internal/service/downstream_output_tokens_test.go`

1. Add a failing test proving `Freeze` invokes an attached upstream cancel callback exactly once.
2. Run the focused test and confirm it fails because no callback is attached.
3. Add an idempotent upstream cancellation binding to the collector and invoke it from cancellation, write-error, and explicit freeze paths.
4. Run the focused collector tests and confirm they pass.

### Task 2: Tie forward contexts to the collector

**Files:**
- Modify: gateway handler files that attach `DownstreamOutputTokenCollector`
- Test: the corresponding gateway streaming tests

1. Add failing HTTP streaming tests where the downstream context is canceled before the upstream terminal usage event; assert the upstream request context is canceled promptly and the result is marked `ClientDisconnect`.
2. Run focused gateway and OpenAI gateway tests and confirm the old drain behavior fails the new assertions.
3. Create a cancelable forward context at each collector attachment point, bind its cancel function to the collector, and pass it into the forwarder.
4. Replace protocol-specific post-disconnect drain behavior with prompt return where detached contexts or WebSocket relays bypass the shared HTTP context.
5. Run focused protocol tests and update old drain-expectation tests to assert immediate cancellation.

### Task 3: Bill disconnected streams by delivered output

**Files:**
- Modify: `backend/internal/service/gateway_usage_billing.go`
- Modify: `backend/internal/service/openai_gateway_usage.go`
- Modify: `backend/internal/service/downstream_output_tokens.go`
- Test: gateway and OpenAI usage billing tests

1. Add failing tests with provider output `13811` and delivered output `1935`; assert the usage log and customer charge use `1935` while `upstream_output_tokens` remains `13811`.
2. Add a normal-completion test proving provider output remains authoritative when `ClientDisconnect` is false.
3. Run focused billing tests and confirm the disconnected case fails under the current provider-based billing.
4. Add helpers that clone the usage used for customer cost calculation and replace output tokens only when the stream disconnected and a delivered snapshot exists.
5. Keep provider usage available for account-cost calculation and diagnostic persistence.
6. Run focused billing tests and confirm all cases pass.

### Task 4: Regression verification

**Files:**
- Test only

1. Run all downstream collector, gateway streaming, OpenAI streaming, and usage billing tests.
2. Run `go test ./internal/service/...` from `backend`.
3. Run repository formatting and static checks used by the project.
4. Inspect `git diff --check` and the final diff for unrelated changes.

