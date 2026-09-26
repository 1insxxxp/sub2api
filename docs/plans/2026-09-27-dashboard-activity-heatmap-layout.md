# Dashboard Activity Heatmap Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复客户仪表盘年度使用热力图的宽度分配、图例位置和日期详情浮层裁切问题。

**Architecture:** 保留现有数据网格和响应式横向滚动，仅增加一个紧凑的日历内容轨道，让网格、月份和图例共享同一宽度；根据单元格所在行和列为详情浮层选择上下及左右定位，避免溢出卡片边界。统计接口和指标计算不变。

**Tech Stack:** Vue 3、`<style scoped>`、Vitest、现有本地 Vite 开发环境。

---

### Task 1: Lock the layout behavior with component tests

**Files:**
- Modify: `frontend/src/components/user/dashboard/__tests__/UserDashboardActivityHeatmap.spec.ts`

**Step 1:** Add assertions for the calendar track and placement classes on a lower-row and edge-column cell.

**Step 2:** Run the focused component test and confirm the new assertions fail before the template/style change.

### Task 2: Tighten the heatmap layout and popover placement

**Files:**
- Modify: `frontend/src/components/user/dashboard/UserDashboardActivityHeatmap.vue`

**Step 1:** Add a compact track around the calendar and legend, keeping the existing horizontal overflow behavior on narrow screens.

**Step 2:** Add placement helpers/classes so lower rows open above and edge columns align inward.

**Step 3:** Reduce unnecessary vertical spacing and keep the legend directly beneath the grid.

### Task 3: Verify the local dashboard

**Files:**
- No additional files.

**Step 1:** Run the focused heatmap Vitest suite and `vue-tsc --noEmit`.

**Step 2:** Rebuild/restart the local backend only if needed, reload `http://localhost:3000/dashboard`, and visually verify the grid, legend, tooltip, and date controls.

**Step 3:** Run `git diff --check` and record the final changed files.
