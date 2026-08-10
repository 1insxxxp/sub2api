# Available Channel Flat Offerings Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the nested legacy price table in expanded model cards with a flat responsive offering list.

**Architecture:** Create a presentation-only offering component that consumes `CatalogModelOffering` and existing money formatting helpers. Mount it from `AvailableChannelModelList` without changing catalog construction or billing logic.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest, Vue Test Utils.

---

### Task 1: Flat offering component

1. Add failing tests for channel/group/platform/rate, token/request/image/tier prices, zero/unpriced and responsive flat DOM.
2. Implement `AvailableChannelOfferingCard.vue` using existing catalog values and formatter.
3. Run focused tests, ESLint and typecheck.

### Task 2: Model card integration

1. Add a failing model-list test asserting the legacy price component is absent from expanded offerings.
2. Replace `AvailableChannelModelPrice` with the flat offering component.
3. Run all channel tests and production build.
4. Commit, merge to `dev`, and verify the running local page.
