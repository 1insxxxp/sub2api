# Self-hosted Gemini Model Alias Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make account `自营号池` serve `gemini-3.5-flash` through upstream model `gemini-3.5-flash-low` without changing other accounts.

**Architecture:** Update the existing account-level model mapping in production while preserving all current entries. Clear only this account's stale rate-limit fields, then verify both the direct target and public alias through live requests.

**Tech Stack:** PostgreSQL, Sub2API, Antigravity-Manager HTTP API

---

### Task 1: Snapshot the account configuration

**Files:**
- Modify: production database account row for `自营号池`

**Step 1:** Query the account ID, existing model mapping, schedulable state, and rate-limit timestamps.

**Step 2:** Save the original mapping value for rollback and confirm there is exactly one matching account.

### Task 2: Add the account-level alias

**Files:**
- Modify: production database account row for `自营号池`

**Step 1:** Merge `gemini-3.5-flash: gemini-3.5-flash-low` into the existing mapping without replacing other entries.

**Step 2:** Read the row back and verify the persisted source and target names.

### Task 3: Restore and verify scheduling

**Files:**
- Modify: production database account row for `自营号池`

**Step 1:** Clear only the stale rate-limit timestamp and reset timestamp created by the prior false 429.

**Step 2:** Call `gemini-3.5-flash-low` and expect HTTP 200.

**Step 3:** Call the public alias `gemini-3.5-flash` through Sub2API and expect HTTP 200.

**Step 4:** Re-query account state and recent errors; expect the account to remain schedulable with no new rate-limit timestamp.
