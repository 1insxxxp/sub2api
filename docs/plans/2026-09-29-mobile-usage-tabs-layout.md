# 移动端使用记录页签与布局统一 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 统一用户使用记录页三个移动端页签、筛选区和顶部间距，使记录入口更紧凑并符合 API 密钥页的视觉基准。

**Architecture:** 保留 `UsageView.vue` 的现有数据流、页签状态和筛选逻辑，只调整移动端 CSS 与页签结构。移动端使用三等分分段控件和统一工具区，桌面端继续沿用底部指示线页签；现有统计分析折叠逻辑保持不变。

**Tech Stack:** Vue 3 `<script setup>`, scoped CSS, Vitest, Vue Test Utils, vue-tsc, Vite。

---

### Task 1: Add regression coverage for the unified mobile tab surface

**Files:**
- Modify: `frontend/src/views/user/__tests__/UsageView.spec.ts`

**Step 1: Write the failing test**

Add assertions that the usage, error, and empty-response tabs all render inside one `usage-record-tabs` container, expose three tab buttons when error viewing is enabled, and preserve the active tab state when each button is clicked.

**Step 2: Run the focused test**

Run: `pnpm exec vitest run src/views/user/__tests__/UsageView.spec.ts`

Expected: the new structure/style hook assertions fail before the implementation changes.

**Step 3: Keep the test focused**

Use existing API mocks and mount helper. Do not assert implementation-only colors or pixel values; assert stable classes, button count, `aria-pressed`, and active-state transitions.

### Task 2: Implement the mobile tab and spacing system

**Files:**
- Modify: `frontend/src/views/user/UsageView.vue`

**Step 1: Normalize the tab markup**

Keep the current buttons and click handlers, but add stable class hooks for the segmented surface and make each button occupy one equal flex track on mobile. Preserve the existing desktop bottom-indicator treatment outside the mobile media query.

**Step 2: Add API-key-page material styling**

Use existing `--workspace-*` tokens for the tab surface, border, control background, active brand-blue state, focus ring, and dark mode. Keep the active state visually clear without adding a new component or changing the tab semantics.

**Step 3: Reduce mobile vertical density**

Tighten the mobile statistics-card padding/gaps, workspace toolbar spacing, date/granularity control spacing, filter grid gaps, and action-row spacing. Keep 40px minimum touch targets for icon actions and prevent tab labels or controls from wrapping or overflowing at 375px width.

**Step 4: Preserve existing behavior**

Do not change API calls, filters, pagination, error loading, empty-response claims, analytics loading, or the analytics collapse state. Keep desktop layout rules unchanged except for any shared token-level styling already used by the page.

### Task 3: Verify responsive behavior and integration

**Files:**
- Verify: `frontend/src/views/user/UsageView.vue`
- Verify: `frontend/src/views/user/__tests__/UsageView.spec.ts`

**Step 1: Run focused tests**

Run: `pnpm exec vitest run src/views/user/__tests__/UsageView.spec.ts`

Expected: all UsageView tests pass.

**Step 2: Run static checks**

Run: `pnpm run typecheck && pnpm run check:i18n`

Expected: both commands exit successfully.

**Step 3: Run lint and diff checks**

Run: `pnpm exec eslint src/views/user/UsageView.vue src/views/user/__tests__/UsageView.spec.ts src/i18n/locales/zh/dashboard.ts src/i18n/locales/en/dashboard.ts && git diff --check`

Expected: no lint errors and no whitespace errors.

**Step 4: Manually inspect mobile and desktop**

Open `/usage` at 375px and 440px widths. Confirm the three tabs are aligned, the record filter starts sooner, the active state is consistent across all three views, and the analytics section remains after the records. Check one desktop viewport to confirm the original tab treatment remains intact.

**Step 5: Commit the implementation**

```bash
git add frontend/src/views/user/UsageView.vue frontend/src/views/user/__tests__/UsageView.spec.ts frontend/src/i18n/locales/zh/dashboard.ts frontend/src/i18n/locales/en/dashboard.ts
git commit -m "style: unify mobile usage record tabs"
```
