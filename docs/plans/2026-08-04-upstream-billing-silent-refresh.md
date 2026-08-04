# Upstream Billing Silent Refresh Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Keep the accounts table visible while silently synchronizing fresh account data after upstream billing probes.

**Architecture:** Extend the shared table loader with an optional silent load mode that preserves request cancellation and data replacement semantics without enabling table loading UI. Use that mode only in the post-probe synchronization path.

**Tech Stack:** Vue 3, TypeScript, Vitest

---

### Task 1: Add silent loading to the shared table loader

**Files:**
- Modify: `frontend/src/composables/useTableLoader.ts`
- Test: `frontend/src/composables/__tests__/useTableLoader.spec.ts`

**Steps:** Write failing silent-loading and cancellation tests, implement the optional load argument, then run the focused tests.

### Task 2: Use silent synchronization after billing probes

**Files:**
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Test: `frontend/src/views/admin/__tests__/AccountsView.upstreamBillingRefresh.spec.ts`

**Steps:** Add a failing source-contract test, update the account wrapper and post-probe helper to call silent load, then verify single and batch paths share it.

### Task 3: Verify and commit

Run focused tests, type checking, production build, restart the local backend, check health, and commit the implementation.

