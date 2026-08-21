# User Redeem Code Actions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a generation result modal with copy/download actions and user-side bulk deletion for unused balance-transfer redeem codes, then merge the work into `dev`.

**Architecture:** The backend adds a current-user batch delete endpoint that shares the same ownership, source, type, and unused-code guarantees as single delete. The frontend keeps the existing redeem page layout, adds a focused generation-success modal, and adds selectable generated-code rows with a bulk delete command.

**Tech Stack:** Go/Gin/Ent service tests for backend behavior, Vue 3/TypeScript/Vitest for the user redeem page and API client.

---

### Task 1: Backend Batch Delete Contract

**Files:**
- Modify: `backend/internal/service/redeem_service_balance_transfer_test.go`
- Modify: `backend/internal/service/redeem_service.go`
- Modify: `backend/internal/repository/redeem_code_repo.go`
- Modify: `backend/internal/handler/redeem_handler.go`
- Modify: `backend/internal/server/routes/user.go`

**Steps:**
1. Write failing service tests for deleting multiple unused codes and for all-or-nothing rejection when a selected code is already used or not owned.
2. Run the targeted service test and confirm it fails because `DeleteGeneratedBalanceTransferCodes` does not exist.
3. Add a batch repository capability that locks matching rows, validates every selected code, deletes all matching rows, and returns deleted code records.
4. Add the service method that validates IDs, uses one transaction when available, refunds the summed value once, and invalidates creator caches.
5. Add `POST /api/v1/redeem/generated/batch-delete` handler and route.

### Task 2: Frontend API and Page Behavior

**Files:**
- Modify: `frontend/src/api/__tests__/redeem.spec.ts`
- Modify: `frontend/src/api/redeem.ts`
- Modify: `frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`
- Modify: `frontend/src/views/user/RedeemView.vue`
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts`

**Steps:**
1. Write failing API client test for `deleteGeneratedBatch([id...])`.
2. Write failing Vue tests for showing generated codes in a modal, copying/downloading the full generated list, and submitting selected generated codes to the batch delete endpoint.
3. Implement the API method using `POST /redeem/generated/batch-delete`.
4. Replace the inline latest-code panel with a modal that lists all codes and provides copy/download actions.
5. Add selection state, selectable generated-code rows, select-all, and bulk delete behavior. Prune selection after refreshes and after single deletion.
6. Add Chinese and English locale keys.

### Task 3: Verification, Commit, and Dev Merge

**Files:**
- Verify all modified backend and frontend test targets.

**Steps:**
1. Run Go service tests for balance-transfer redeem behavior.
2. Run frontend API and redeem page Vitest suites.
3. Run formatting where required.
4. Commit the feature branch.
5. Merge the committed feature branch into local `dev` and report the exact status.
