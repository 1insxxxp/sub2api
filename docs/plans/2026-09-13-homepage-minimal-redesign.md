# Minimal Homepage Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将默认首页重构为保留蓝青主题的简洁产品首屏，移除静态伪实时看板和重复内容，并加入轻量、可关闭的动画。

**Architecture:** 保留 HomeView 的自定义 HTML/iframe 与 compact 首页分支，只重构默认首页分支。默认分支使用动态站点设置和现有认证/导航状态，内容收敛为 Hero、三条价值点和一次 CTA；动画使用 scoped CSS 与 `prefers-reduced-motion`，不增加运行时依赖。

**Tech Stack:** Vue 3、TypeScript、Tailwind CSS、scoped CSS、Vitest、Vue Test Utils。

---

### Task 1: Define the minimal default homepage structure

**Files:**
- Modify: `frontend/src/views/HomeView.vue`
- Test: `frontend/src/views/__tests__/HomeView.spec.ts`
- Test: `frontend/src/views/__tests__/HomeView.compact.spec.ts`

**Step 1: Update failing homepage tests**

Assert the default branch renders the minimal hero, three value points, one primary CTA, and no static routing/billing board, channel rows, workflow cards, or code panel. Keep tests for custom content, compact mode, authentication links, theme toggle, and navigation behavior.

**Step 2: Run the focused tests to verify failure**

Run: `cd frontend && pnpm vitest run src/views/__tests__/HomeView.spec.ts src/views/__tests__/HomeView.compact.spec.ts`
Expected: the existing page still renders the removed dashboard and old copy.

**Step 3: Replace only the default homepage template**

Keep the existing `hasHomeContent` and `compactHomeEnabled` branches intact. Replace the default branch with a responsive header, a single hero section, three concise value points, a short quick-start panel, and a compact footer. Preserve dynamic site name/logo, docs URL, model plaza visibility, locale/theme controls, and dashboard/login destinations.

**Step 4: Remove obsolete static view data**

Delete computed arrays used only by the removed routing board, trust strip, capability cards, integration code sample, and workflow cards. Keep only data needed by the new value points and navigation.

**Step 5: Run focused tests**

Run the same Vitest command; all relevant tests pass.

### Task 2: Add restrained motion and mobile styling

**Files:**
- Modify: `frontend/src/views/HomeView.vue`
- Modify: `frontend/src/style.css` only if shared homepage classes require it

**Step 1: Add CSS motion hooks**

Implement staggered fade/translate reveals, a slow background glow, and hover feedback for the primary CTA and value points. Use CSS custom properties for timing and disable nonessential motion inside `prefers-reduced-motion: reduce`.

**Step 2: Add mobile-first layout rules**

Use single-column layout below `sm`, full-width CTA buttons, compact header controls, readable line lengths, and no horizontal overflow. Keep larger spacing and two-column decorative treatment only at `lg` widths.

**Step 3: Run lint and type checks**

Run: `cd frontend && pnpm typecheck && pnpm lint:check`
Expected: both commands exit successfully.

### Task 3: Full verification and commit

**Files:** None.

**Step 1: Run tests and production build**

```bash
cd frontend
pnpm vitest run src/views/__tests__/HomeView.spec.ts src/views/__tests__/HomeView.compact.spec.ts src/i18n/__tests__/homeLocales.spec.ts
pnpm build
```

**Step 2: Verify local server**

Run `curl -fsS -o /dev/null -w '%{http_code}\\n' http://localhost:3000/` and expect `200`.

**Step 3: Commit**

```bash
git add frontend/src/views/HomeView.vue frontend/src/views/__tests__/HomeView.spec.ts frontend/src/views/__tests__/HomeView.compact.spec.ts
# include shared styles or locale changes only if modified
git commit -m "feat: simplify homepage presentation"
```
