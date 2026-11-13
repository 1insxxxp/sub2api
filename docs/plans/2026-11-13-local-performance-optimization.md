# Local Performance Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Identify and reduce slow local page APIs using measured PostgreSQL plans before changing application behavior.

**Architecture:** Measure representative API/database queries against the copied production database, refresh planner statistics, compare plans and latency, then add only evidence-based indexes or caching changes. Keep production untouched until separately approved.

**Tech Stack:** Go, Gin, GORM/raw PostgreSQL queries, PostgreSQL 16, Vue 3/Vite, launchd local runtime.

---

### Task 1: Capture baseline

**Files:**
- Read: `backend/internal/handler/`
- Read: `backend/internal/repository/`
- Read: `frontend/src/views/`

**Step 1:** Identify representative endpoints for users, accounts, groups, usage logs, and dashboard data.

**Step 2:** Run each endpoint or equivalent SQL query three times against the local copied database and record elapsed time, rows, and query plan.

**Step 3:** Save the baseline outside the repository under `/tmp/sub2-performance-baseline.txt`.

### Task 2: Refresh local planner statistics

**Files:** None.

**Step 1:** Run `ANALYZE` on the largest affected local tables only.

**Step 2:** Repeat the baseline queries and compare execution plans and timings.

**Step 3:** Record whether the improvement comes from statistics alone.

### Task 3: Add evidence-based database indexes if needed

**Files:**
- Create: `backend/migrations/<next>_local_page_performance_indexes.sql`
- Test: `backend/migrations/*index*_test.go` when migration structure needs regression coverage

**Step 1:** Write a failing migration-content test for every required index.

**Step 2:** Add only composite indexes matching observed WHERE, ORDER BY, and JOIN predicates.

**Step 3:** Run migration tests and `EXPLAIN` comparisons.

**Step 4:** Commit the migration separately.

### Task 4: Optimize endpoint and frontend waterfalls only if measured

**Files:**
- Modify: the specific handler/repository files identified by Task 1
- Modify: the specific Vue view/store identified by Task 1
- Test: matching Go/Vitest tests

**Step 1:** Add a regression test showing the slow or duplicated behavior.

**Step 2:** Implement the smallest change, such as delayed noncritical counts, short-lived cache, request deduplication, or virtualized rendering.

**Step 3:** Run targeted tests, backend build, frontend typecheck/build, and repeat the latency benchmark.

**Step 4:** Commit the behavior change separately.
