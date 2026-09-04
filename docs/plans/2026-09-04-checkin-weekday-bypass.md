# 签到调用次数星期豁免 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow admins to exempt selected weekdays from the minimum daily request count required for check-in.

**Architecture:** Store ISO weekday numbers in the existing check-in settings. Normalize and validate them in the service, use the same calendar-date helper for status and transactional check-in validation, and expose the setting through the existing admin form.

**Tech Stack:** Go service, Ent-backed settings repository, Vue 3 + TypeScript, Vitest.

---

### Task 1: Add service configuration and rule evaluation

**Files:** `backend/internal/service/domain_constants.go`, `backend/internal/service/checkin_service.go`, `backend/internal/service/checkin_reward_campaign_service.go`

**Steps:** Add the setting key and config/status fields, load and persist JSON weekday values, normalize valid ISO weekdays, and skip only the daily-count check for configured dates in both policy and transaction paths. Copy the field when merging campaign config.

### Task 2: Add backend regression coverage

**Files:** `backend/internal/service/checkin_service_test.go`

**Steps:** Test Saturday/Sunday ISO matching and normalization/deduplication. Run the targeted service tests and document any unrelated repository test blocker.

### Task 3: Add admin API and UI controls

**Files:** `backend/internal/handler/admin/checkin_handler.go`, `frontend/src/api/admin/checkins.ts`, `frontend/src/views/admin/CheckinsView.vue`, `frontend/src/i18n/locales/zh/restored.ts`, `frontend/src/i18n/locales/en/restored.ts`

**Steps:** Accept and return the weekday list through the existing config endpoint, add weekday checkboxes, and provide localized labels and an explanation that only the daily count is waived.

### Task 4: Update user-facing status and tests

**Files:** `frontend/src/api/checkin.ts`, `frontend/src/components/layout/AppHeader.vue`, `frontend/src/views/admin/__tests__/CheckinsView.spec.ts`

**Steps:** Expose a daily-count exemption flag, hide misleading progress criteria on exempt days, show a clear message, and test load/save behavior.

### Task 5: Verify and commit

**Steps:** Run Go formatting/build, frontend lint/tests/build, inspect `git diff --check`, then commit the completed feature on local `dev`. Do not push or deploy without a separate request.
