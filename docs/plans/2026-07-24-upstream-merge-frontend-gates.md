# Upstream Merge Frontend Gates Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restore every frontend quality gate after the upstream `v0.1.164` merge without changing intended runtime behavior.

**Architecture:** Treat the existing 12 failures as the red baseline. Repair each contract at its source: align API tests with the intentional timeout, complete locale trees, and migrate only reported legacy UI fragments to existing shared admin classes.

**Tech Stack:** Vue 3, TypeScript, Vue I18n, Vitest, ESLint, pnpm

---

### Task 1: Rollback API Contract

**Files:**
- Modify: `frontend/src/api/__tests__/admin.system.rollback.spec.ts`
- Reference: `frontend/src/api/admin/system.ts`

1. Keep the production 15-minute timeout unchanged.
2. Update both failing expectations to require `{ timeout: 900000 }`.
3. Run `pnpm vitest run src/api/__tests__/admin.system.rollback.spec.ts` and require all tests to pass.

### Task 2: Locale Completeness

**Files:**
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/ops.ts`
- Modify: `frontend/src/i18n/locales/en/admin/ops.ts`
- Test: `frontend/src/i18n/__tests__/localeParity.spec.ts`
- Test: `frontend/src/i18n/__tests__/productionLocaleKeys.spec.ts`

1. Add Chinese counterparts for the 34 English account messages.
2. Add `attemptedKeyPrefix` and `deletedKeyOwner` in both ops locale files.
3. Run both locale quality-gate tests and require them to pass.

### Task 3: Shared Admin Surfaces

**Files:**
- Modify only source files named by the seven failing surface assertions.
- Test the five affected surface spec files.

1. Replace each reported legacy hover, panel, card, or action-menu fragment with the nearest existing `admin-*` shared class.
2. Preserve events, conditions, responsive behavior, and visible text.
3. Run the affected surface specs and require them to pass.

### Task 4: Full Verification And Delivery

**Files:**
- Verify all modified files with `git diff --check`.

1. Run `pnpm run lint:check`, `pnpm run typecheck`, `pnpm run test:run`, and `pnpm run build` in `frontend`.
2. Run `go test ./...` and `go build -tags embed ./cmd/server` in `backend`.
3. Commit only tracked repair files and the two plan documents; leave root `package.json` and `package-lock.json` untracked.
4. Push `codex/server-700d295e` normally and verify local and remote commit hashes match.
5. Do not deploy or restart production services.
