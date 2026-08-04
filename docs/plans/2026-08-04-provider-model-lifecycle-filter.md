# Provider Model Lifecycle Filter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Generalize retired-model filtering across supported providers while filtering only model IDs known to be unusable.

**Architecture:** Replace the Anthropic-specific helper with an account-aware provider lifecycle registry. Reuse it in upstream sync and gateway aggregation, and curate Gemini's no-mapping defaults to the same lifecycle state.

**Tech Stack:** Go, provider account metadata, testify.

---

### Task 1: Add failing Gemini lifecycle tests

**Files:**
- Modify: `backend/internal/service/anthropic_model_lifecycle_test.go`
- Modify: `backend/internal/pkg/geminicli/models_test.go`

Add tests for Gemini mapping aggregation, upstream sync, Antigravity isolation, and current default models. Run focused tests and verify shutdown IDs are still returned.

### Task 2: Generalize the lifecycle registry

**Files:**
- Rename: `backend/internal/service/anthropic_model_lifecycle.go` to `backend/internal/service/model_lifecycle.go`
- Rename: `backend/internal/service/anthropic_model_lifecycle_test.go` to `backend/internal/service/model_lifecycle_test.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/upstream_models.go`

Add exact per-platform shutdown sets and a shared account-aware predicate/filter. Preserve Bedrock, Grok redirect, OpenAI deprecated, and Antigravity behavior.

### Task 3: Curate Gemini defaults

**Files:**
- Modify: `backend/internal/pkg/geminicli/models.go`
- Modify: `backend/internal/pkg/geminicli/models_test.go`

Remove shutdown defaults and move `DefaultTestModel` to an active Gemini model.

### Task 4: Verify and commit

Run `gofmt`, `git diff --check`, focused tests, and `go test ./...`. Commit with `fix: filter unavailable provider models`.
