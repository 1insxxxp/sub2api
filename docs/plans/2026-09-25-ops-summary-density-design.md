# Ops Error Summary Density Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement the plan task-by-task.

**Goal:** Make the upstream error summary modal show more actionable records per screen with all hierarchy levels expanded by default.

**Architecture:** Keep the existing summary data and toggle behavior. Initialize group, model, and account expansion sets from the loaded response, then tighten the modal hierarchy spacing and use responsive grid layouts for model/account rows while retaining mobile single-column flow.

**Tech Stack:** Vue 3, TypeScript, Tailwind utility classes, existing BaseDialog and ops summary types.

---

### Task 1: Expand and densify the summary modal

**Files:**
- Modify: `frontend/src/views/admin/ops/components/OpsUpstreamErrorSummaryModal.vue`

**Steps:**
1. Initialize groups, models, and accounts as expanded after loading.
2. Reduce vertical padding and nested gaps while keeping accessible buttons and manual collapse.
3. Render model and account content in responsive grids; keep reasons readable with compact spacing.
4. Run the focused frontend typecheck/test command and inspect the local route `/admin/ops`.

