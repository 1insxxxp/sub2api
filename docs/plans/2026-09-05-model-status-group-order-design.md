# Model Status Group Order Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Keep the model status report group order identical to Sub2API's configured `sort_order`.

**Architecture:** Sub2API will include each active group's `sort_order` in the signed catalog snapshot and preserve the repository order when building the snapshot. The report service will validate and retain that field, and the UI/report output will order groups by `sort_order` with group ID as a deterministic tie-breaker instead of sorting by name or relying on incidental dictionary order.

**Tech Stack:** Go catalog producer, Python FastAPI report service, React/Vite report UI, Go/Python/JS tests.

---

### Task 1: Extend the catalog contract and producer ordering

**Files:**
- Modify: `backend/internal/service/catalog_push.go`
- Test: `backend/internal/service/catalog_push_test.go` (or the existing catalog producer test file)

**Steps:**
1. Add a failing test showing snapshots carry `sort_order` and retain repository order when IDs do not match the configured order.
2. Run the focused Go test and confirm it fails because the field is absent or the producer re-sorts by ID.
3. Add `SortOrder` to `CatalogGroup`, populate it from `Group.SortOrder`, and remove the ID-based snapshot sort so `ListActive`'s `sort_order, id` order is preserved.
4. Add validation for a deterministic non-negative sort order if the existing group model requires it.
5. Run the focused Go tests and confirm they pass.

### Task 2: Preserve and apply order in the report backend

**Files:**
- Modify: `/Users/alien/Workspace/robot/passion-qq-bot/model-status-report/app/catalog_snapshot.py`
- Modify: `/Users/alien/Workspace/robot/passion-qq-bot/model-status-report/app/catalog.py`
- Test: `/Users/alien/Workspace/robot/passion-qq-bot/model-status-report/tests/test_catalog.py`

**Steps:**
1. Add a failing test with groups whose IDs/names conflict with their configured sort order.
2. Run the focused Python tests and confirm the report order is currently wrong.
3. Validate and retain `sort_order` from signed snapshots, then sort report groups by `(sort_order, id)` only at the final output boundary.
4. Keep the existing fallback catalog deterministic using the same key where available.
5. Run the focused Python tests and confirm they pass.

### Task 3: Keep the UI's group selector and rendered sections aligned

**Files:**
- Modify: `/Users/alien/Workspace/robot/passion-qq-bot/model-status-report/src/main.jsx`
- Test: `/Users/alien/Workspace/robot/passion-qq-bot/model-status-report/src/main.test.jsx`

**Steps:**
1. Add a failing UI test proving the selector and sections follow `sort_order` rather than name/ID order.
2. Implement a shared ordered-group projection used by the selector, search results, and rendered sections.
3. Run Vitest and confirm the new ordering test passes.
4. Run the production Vite build.

### Task 4: Commit, push, deploy, and verify

**Steps:**
1. Review the diff and run focused backend, report Python, Vitest, and build checks.
2. Commit and push both repositories' `dev` branches.
3. Deploy the Sub2API producer and report service using the existing production procedure, preserving the current catalog signing secret.
4. Verify health endpoints, snapshot contents, and that the public report exposes groups in the same `sort_order` sequence.

