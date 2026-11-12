# Gemini Ultra Model Mapping and Tool Role Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 Gemini 上游返回的模型 Provider 未知错误纳入模型级切号，并确保 functionResponse 始终位于 user 角色消息中。

**Architecture:** 扩展现有 Gemini 模型不存在识别逻辑，只对明确的 400 `unknown provider for model` 放行到既有模型冷却和下一账号流程。扩展现有 Gemini 原生内容归一化，在合并历史前校正 functionCall/functionResponse 的消息角色，保持现有工具调用顺序和协议转换路径。

**Tech Stack:** Go、Gin、testify、现有 Gemini service 单元测试。

---

### Task 1: Model Provider error failover

**Files:**
- Modify: `backend/internal/service/model_not_found_error.go`
- Modify: `backend/internal/service/gemini_model_not_found_failover.go`
- Test: `backend/internal/service/gemini_model_not_found_failover_test.go`

**Steps:**
1. Add a failing test for HTTP 400 `unknown provider for model gemini-3.8-flash-high`.
2. Run the focused Gemini failover test and verify it fails because the current logic only accepts HTTP 404.
3. Update model error classification to recognize only the explicit unknown-provider-for-model shape at HTTP 400.
4. Reuse the existing model cooldown, ops event, and next-account failover path.
5. Run the focused test and the existing model-not-found tests.

### Task 2: Function response role normalization

**Files:**
- Modify: `backend/internal/service/gemini_native_request_normalizer.go`
- Modify: `backend/internal/service/gemini_messages_compat_service.go`
- Test: `backend/internal/service/gemini_native_request_normalizer_test.go`
- Test: `backend/internal/service/gemini_messages_compat_service_test.go` when the compatibility conversion path needs coverage.

**Steps:**
1. Add a failing normalizer test where a functionResponse is carried by a model-role content and assert it becomes user-role.
2. Run the focused normalizer test and verify the current implementation leaves the invalid model role unchanged.
3. Add role normalization before function history merging. Function response-only contents become user role, function call-only contents become model role, and mixed contents are split into valid turns.
4. Run focused normalizer and Gemini compatibility tests.
5. Run the affected backend service package tests with the repository's `-p 2` limit.

### Task 3: Verification

**Steps:**
1. Run `gofmt` on modified Go files.
2. Run focused tests for model failover and native request normalization.
3. Run the affected service package test suite with `go test -p 2 ./internal/service`.
4. Review the diff and confirm no deployment or production configuration changed.
5. Commit the implementation after tests pass.

