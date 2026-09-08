# 兑换码多选批量复制 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a mobile-friendly batch-copy action for selected user-generated redeem codes.

**Architecture:** Reuse `RedeemView.vue`'s existing selected ID state and generated-code list. A computed selection derives the currently visible selected records in display order, and a handler writes their codes joined by newlines through the existing clipboard API and notification pattern. The new toolbar action is disabled when no code is selected.

**Tech Stack:** Vue 3 SFC, TypeScript, Vue Test Utils, Vitest, vue-i18n.

---

### Task 1: Add failing coverage for selected-code batch copying

**Files:**
- Modify: `frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`

**Step 1: Write the failing test**

Add generated-code fixtures and a test that mounts `RedeemView`, asserts `generated-codes-copy-selected` is disabled before selection, selects two generated codes, clicks the button, and expects `navigator.clipboard.writeText` to receive the two codes separated by `\\n`.

**Step 2: Run test to verify it fails**

Run: `pnpm --dir frontend exec vitest run src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`

Expected: FAIL because the batch-copy button does not exist yet.

### Task 2: Implement selected-code batch copying

**Files:**
- Modify: `frontend/src/views/user/RedeemView.vue:270-325`
- Modify: `frontend/src/views/user/RedeemView.vue:1280-1370`

**Step 1: Add the toolbar action**

Add a `data-test="generated-codes-copy-selected"` button beside the existing selection actions. Bind `disabled` to an empty selection or an active batch delete, and use the copy icon plus the new translation key.

**Step 2: Derive selected codes in display order**

Add a computed value that filters `generatedCodes` by `selectedGeneratedCodeIds`, preserving the current page's order.

**Step 3: Add the copy handler**

Write the selected codes joined by newlines with `navigator.clipboard.writeText`. Return early when the filtered selection is empty. Reuse `redeem.balanceTransfer.copied` and `redeem.balanceTransfer.copyFailed` notifications and the existing error logging style.

### Task 3: Add translations

**Files:**
- Modify: `frontend/src/i18n/locales/zh/dashboard.ts:1018-1025`
- Modify: `frontend/src/i18n/locales/en/dashboard.ts:1014-1021`

Add `copySelected: '批量复制'` in Chinese and `copySelected: 'Copy Selected'` in English under `redeem.balanceTransfer`.

### Task 4: Verify and commit

**Files:**
- All files above

**Step 1: Run focused tests**

Run: `pnpm --dir frontend exec vitest run src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts`

Expected: PASS.

**Step 2: Run type and locale checks**

Run: `pnpm --dir frontend run typecheck` and `pnpm --dir frontend run check:i18n`

Expected: PASS with no new diagnostics.

**Step 3: Inspect the diff**

Run: `git diff --check` and `git status --short`.

Expected: only the planned feature files are changed, plus the committed plan/design docs.

**Step 4: Commit**

Run: `git add frontend/src/views/user/RedeemView.vue frontend/src/views/user/__tests__/RedeemView.balanceTransfer.spec.ts frontend/src/i18n/locales/zh/dashboard.ts frontend/src/i18n/locales/en/dashboard.ts && git commit -m "feat: add batch copy for redeem codes"`

