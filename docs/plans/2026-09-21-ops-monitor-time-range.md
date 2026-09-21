# 运维监控时间范围记忆 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Persist and restore the admin operations dashboard's top-level time range, including custom start/end timestamps, while preserving URL deep-link precedence.

**Architecture:** Add a small versioned localStorage snapshot helper inside `OpsDashboard.vue`. Initialization uses the existing route query first, then a validated local snapshot, then the existing `1h` default. Existing route synchronization and API parameter construction remain unchanged.

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Vitest, browser `localStorage`.

---

### Task 1: Add failing persistence coverage

**Files:**
- Create: `frontend/src/views/admin/ops/__tests__/OpsDashboard.timeRangePersistence.spec.ts`
- Test: `frontend/src/views/admin/ops/OpsDashboard.vue`

**Steps:**
1. Add source-level tests that assert the component declares a versioned storage key and validation path.
2. Add cases for default `1h`, valid preset restoration, URL `tr` precedence, valid custom start/end restoration, and invalid snapshot fallback.
3. Run the focused test and verify it fails before the implementation is present.

### Task 2: Implement validated local persistence

**Files:**
- Modify: `frontend/src/views/admin/ops/OpsDashboard.vue`

**Steps:**
1. Define the storage key, snapshot type, and safe JSON/date validation helpers next to `allowedTimeRanges`.
2. Read the snapshot during initial state setup only when no valid URL time range exists.
3. Persist the range whenever the top-level range or custom endpoints change; clear custom endpoints when selecting a preset.
4. Keep URL query state authoritative and preserve current API behavior.

### Task 3: Verify and commit

**Files:**
- Modify: `frontend/src/views/admin/ops/__tests__/OpsDashboard.timeRangePersistence.spec.ts`

**Steps:**
1. Run the focused persistence and auto-refresh tests.
2. Run the relevant admin ops test suite or frontend type/build checks.
3. Review the diff and commit only the feature files and plan updates.
