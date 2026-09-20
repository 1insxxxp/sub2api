# Batch Account Model Test Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a serial “test all models” workflow to the admin account test modal, showing per-model success and failure results while preserving the existing single-model test.

**Architecture:** Keep the existing `POST /api/v1/admin/accounts/:id/test` SSE endpoint as the only test execution path. Extend the admin Vue modal with a batch runner that snapshots the current model options, executes one request at a time, records each model result, and renders an aggregate summary. No backend route or persistence changes are required.

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Vitest, Vue Test Utils, vue-i18n, existing fetch/SSE client flow.

---

### Task 1: Add failing batch-runner component tests

**Files:**
- Modify: `frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts`

**Step 1: Write the failing tests**

Add tests that mount the admin modal with three models and assert:
- the batch action is available when a model mode is active;
- starting the batch sends one request per model in model-list order;
- a failed SSE result is recorded while the runner continues to the next model;
- the aggregate result exposes the expected success and failure counts;
- stopping a running batch aborts the active request and marks models that have not started as cancelled.

Use distinct mocked stream responses per `fetch` call so the assertions exercise the real component SSE parser and not an isolated helper.

**Step 2: Run the focused tests to verify they fail**

Run: `pnpm --dir frontend vitest run src/components/admin/account/__tests__/AccountTestModal.spec.ts`

Expected: FAIL because the batch action, state, and runner do not exist yet.

**Step 3: Commit the red tests**

```bash
git add frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts
git commit -m "test: cover batch account model testing"
```

### Task 2: Add batch state, request orchestration, and cancellation

**Files:**
- Modify: `frontend/src/components/admin/account/AccountTestModal.vue`

**Step 1: Implement the minimal batch state**

Add typed batch status/result state for `pending`, `testing`, `success`, `failed`, and `cancelled`, plus computed totals and a batch-running flag. Keep this state separate from the existing single-test `status`, output lines, media previews, and selected model.

**Step 2: Implement one-request SSE execution**

Extract the existing fetch/SSE loop into a small component-local function that accepts a model ID and returns a success/error result. Preserve the existing request-body rules for OpenAI mode, Grok mode, prompt, and media. Ensure `test_complete` with `success: false`, `error` events, non-2xx responses, stream failures, and aborts produce explicit results.

**Step 3: Implement the serial batch loop**

Snapshot `modelOptionsForMode`, initialize all entries as pending, then await each model request in order. Update the current item to testing before its request and success/failed afterward. Continue after ordinary failures. Use a dedicated abort controller and mark unstarted items cancelled when the user stops or closes the dialog.

**Step 4: Run the focused tests to verify they pass**

Run: `pnpm --dir frontend vitest run src/components/admin/account/__tests__/AccountTestModal.spec.ts`

Expected: PASS, including the pre-existing single-model tests.

### Task 3: Render batch controls and results

**Files:**
- Modify: `frontend/src/components/admin/account/AccountTestModal.vue`

**Step 1: Add the batch action and stop action**

Render “Test all models” next to the existing single-model action only when `showModelSelect` is true and there are at least two available model options. Disable both actions while the other workflow is active. Render a stop action during a batch run.

**Step 2: Add the result summary**

Render total, success, failed, and cancelled counts plus a compact per-model list with status and error text. Reuse the existing neutral surface, status colors, icons, and responsive spacing. Keep the existing terminal output for the single-model workflow.

**Step 3: Handle mode changes and reset**

Clear batch results when the modal opens, when the selected Grok mode changes, or when a new batch starts. Ensure `abortStream` also cancels an active batch and prevents stale events from updating a reopened modal.

### Task 4: Add localized labels and run verification

**Files:**
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/en/admin/accounts.ts`

**Step 1: Add matching locale keys**

Add labels for testing all models, stopping the batch, batch summary counts, per-model statuses, and cancelled/failed messages in both locales.

**Step 2: Run focused and static checks**

Run:

```bash
pnpm --dir frontend vitest run src/components/admin/account/__tests__/AccountTestModal.spec.ts
pnpm --dir frontend vitest run src/i18n/__tests__/localeKeyCompleteness.spec.ts
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check -- src/components/admin/account/AccountTestModal.vue src/components/admin/account/__tests__/AccountTestModal.spec.ts
```

Expected: all commands pass with no locale key mismatch or TypeScript errors.

**Step 3: Review the final diff and commit**

```bash
git diff --check
git status --short
git add frontend/src/components/admin/account/AccountTestModal.vue frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/i18n/locales/en/admin/accounts.ts
git commit -m "feat: test all account models"
```
