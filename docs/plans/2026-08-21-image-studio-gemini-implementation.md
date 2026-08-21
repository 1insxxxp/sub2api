# Image Studio Gemini Models Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make AI Image Studio show and execute Gemini image models for Gemini image-enabled groups.

**Architecture:** Add provider-aware model classification, then add a Gemini execution branch to Image Studio while preserving the existing OpenAI/Grok image gateway path. Frontend selection can stay as-is because it already consumes per-group model options from the backend.

**Tech Stack:** Go, Gin, existing Sub2API service layer, existing Gemini account/token helpers, existing Image Studio storage and billing flow, Vue 3 frontend with current options API.

---

### Task 1: Provider-Aware Image Studio Models

**Files:**
- Modify: `backend/internal/service/image_studio_service.go`
- Modify: `backend/internal/service/image_studio_service_test.go`

**Step 1: Write failing tests**

Add tests showing that a Gemini group with `gemini-3.1-flash-image-preview` in its configured models returns that model from `GetOptions`, and that a non-image Gemini model is still hidden.

Run:

```bash
cd backend
go test -tags unit ./internal/service -run 'TestImageStudioServiceGetOptions.*Gemini|TestIsImageGenerationModel' -count=1
```

Expected: FAIL because Image Studio filters only `isOpenAIImageGenerationModel`.

**Step 2: Implement classifier**

Add a helper that checks the group platform before deciding whether a model is valid for Image Studio. Reuse existing Gemini `isImageGenerationModel` for Gemini image IDs and the existing OpenAI/Grok helpers for those platforms.

**Step 3: Verify**

Run the same tests and confirm PASS.

### Task 2: Gemini Image Studio Execution Branch

**Files:**
- Modify: `backend/internal/service/image_studio_gateway.go`
- Modify: `backend/internal/service/image_studio_gateway_test.go`
- Read as references: `backend/internal/service/gemini_chat_completions_compat_service.go`, `backend/internal/service/gemini_token_provider.go`, `backend/internal/service/antigravity_gateway_gemini.go`

**Step 1: Write failing tests**

Add an executor test with a Gemini API key/group/account stub proving:

- The executor accepts `gemini-3.1-flash-image-preview`.
- It builds a Gemini image-generation request.
- It extracts inline base64 image data.
- It defers `RecordUsage` until `CommitUsage`.

Run:

```bash
cd backend
go test -tags unit ./internal/service -run 'TestImageStudioGatewayExecutorGenerate.*Gemini' -count=1
```

Expected: FAIL because no Gemini branch exists.

**Step 2: Implement minimal Gemini branch**

Route `ImageStudioGatewayExecutor.Generate` and `Edit` through a Gemini branch when the selected key's group platform is Gemini and the requested model is a Gemini image model. Use the existing API key resolution and billing check. Inject the existing Gemini compat service through a narrow interface so the branch can reuse `ForwardNative` and existing Gemini API Key/OAuth/ServiceAccount handling.

**Step 3: Verify**

Run the focused executor tests and confirm PASS.

### Task 3: End-To-End Service Validation

**Files:**
- Modify: `backend/internal/service/image_studio_service_test.go`
- Modify as needed: `backend/internal/service/image_studio_types.go`

**Step 1: Add service test**

Cover a request that selects a Gemini image-enabled group and Gemini model. It should pass validation and send the normalized model to the executor.

Run:

```bash
cd backend
go test -tags unit ./internal/service -run 'TestImageStudioService.*Gemini' -count=1
```

Expected: PASS after Tasks 1 and 2.

### Task 4: Regression Verification

Run the focused Image Studio service tests plus a server compile check:

```bash
cd backend
go test -tags=unit ./internal/service -run 'TestImageStudio|TestExtractImageStudio|TestImageStudioPrompt' -count=1
go test ./cmd/server
```

Expected: all selected tests PASS. A full `go test -tags=unit ./internal/service` may currently expose unrelated pre-existing concurrent map access in the Image Studio task repository test stub.

### Task 5: Commit

After verification:

```bash
git status --short
git add docs/plans/2026-08-21-image-studio-gemini-design.md docs/plans/2026-08-21-image-studio-gemini-implementation.md backend/internal/service
git commit -m "add gemini models to image studio"
```
