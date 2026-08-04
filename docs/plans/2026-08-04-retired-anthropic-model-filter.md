# Retired Anthropic Model Filter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Hide officially retired Anthropic direct-API models from synced and channel-facing model lists without deleting saved mappings.

**Architecture:** Add a centralized exact-ID lifecycle filter scoped by account type. Apply it while ingesting upstream model lists and while aggregating schedulable account mappings, preserving models supplied by partner-platform accounts.

**Tech Stack:** Go, testify, existing account and gateway services.

---

### Task 1: Add failing lifecycle-filter tests

**Files:**
- Create: `backend/internal/service/anthropic_model_lifecycle_test.go`
- Modify: `backend/internal/service/upstream_models_test.go`
- Modify: `backend/internal/service/gateway_hotpath_optimization_test.go`

**Step 1:** Add tests for direct Anthropic filtering, Bedrock preservation, multi-account union behavior, and upstream sync filtering.

**Step 2:** Run focused tests and verify they fail because retired IDs are still returned.

Run: `go test ./internal/service -run 'RetiredAnthropic|UpstreamSupportedModels.*Retired' -count=1`

Expected: FAIL on retired model presence.

### Task 2: Implement exact-ID filtering

**Files:**
- Create: `backend/internal/service/anthropic_model_lifecycle.go`
- Modify: `backend/internal/service/upstream_models.go`
- Modify: `backend/internal/service/gateway_service.go`

**Step 1:** Define the exact retired Anthropic model IDs and an account-scoped predicate.

**Step 2:** Filter upstream sync results before returning them.

**Step 3:** Filter each account's mapping keys during gateway model aggregation, before caching.

**Step 4:** Run focused tests and verify PASS.

### Task 3: Verify and commit

**Step 1:** Run `gofmt` on changed Go files and `git diff --check`.

**Step 2:** Run `go test ./...` from `backend`.

**Step 3:** Commit with `fix: hide retired anthropic models`.
