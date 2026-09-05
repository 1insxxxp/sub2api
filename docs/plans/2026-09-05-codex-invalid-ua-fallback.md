# Codex Invalid UA Fallback Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent an invalid OpenAI Codex User-Agent setting from downgrading OAuth model requests to the stale built-in client version.

**Architecture:** Keep the configured client version as the source of truth. When the optional UA setting cannot be parsed as an official Codex identity, rebuild the standard Codex TUI UA from the effective version instead of returning the invalid value.

**Tech Stack:** Go, service-layer unit tests.

---

### Task 1: Add the regression test

**Files:**
- Modify: `backend/internal/service/openai_codex_version_sync_service_test.go`

**Step 1:** Add a test showing an invalid UA value (an email address) produces a standard UA using the synced version.

**Step 2:** Run the focused test and confirm it fails because the current implementation returns the invalid UA unchanged.

### Task 2: Implement the fallback

**Files:**
- Modify: `backend/internal/service/setting_gateway_runtime.go`

**Step 1:** Change the invalid-UA branch of `GetOpenAICodexCanonicalUserAgent` to return `buildCodexCLIUserAgent(version)`.

**Step 2:** Run the focused test and the related Codex identity/version test packages.

### Task 3: Verify the change

**Files:**
- No additional files.

**Step 1:** Run `gofmt` on modified Go files.

**Step 2:** Run focused service tests and a compile-only backend test.

**Step 3:** Review the diff and confirm no production configuration or credentials changed.

**Step 4:** Commit the code fix after verification.
