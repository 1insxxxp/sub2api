# API Key Group Selector Mobile Sheet Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Render the API-key group selector as a bottom sheet on narrow screens while preserving the anchored desktop dropdown.

**Architecture:** Select presentation mode when opening based on a 768px viewport breakpoint. Reuse the existing teleported selector content, adding a dismissible backdrop and conditional fixed positioning for mobile.

**Tech Stack:** Vue 3, TypeScript, Vitest, Tailwind CSS

---

### Task 1: Add responsive selector presentation

**Files:**
- Modify: `frontend/src/views/user/KeysView.vue`
- Test: `frontend/src/views/user/__tests__/KeysViewLayout.spec.ts`

**Steps:**

1. Add failing layout assertions for a mobile-mode state, backdrop, bottom inset positioning, and 768px breakpoint.
2. Run the focused test and confirm it fails for the missing sheet behavior.
3. Add the mobile-mode state and conditional bottom-sheet/desktop-dropdown classes and styles.
4. Run focused tests, type checking, and production build.
5. Restart the local embedded frontend backend and run health checks.
6. Commit the implementation.

