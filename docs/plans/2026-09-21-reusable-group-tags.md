# 可复用自定义分组标签 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Surface existing custom group tags as reusable options in group create/edit forms while keeping each group's applied label and color independent.

**Architecture:** Reuse the existing `/admin/groups/all?include_inactive=true` data through `adminAPI.groups.getAllIncludingInactive`. `GroupsView` deduplicates custom tags and passes them into `GroupTagField`; the field renders selectable custom options and emits the selected tag and color. No backend schema or endpoint changes are required.

**Tech Stack:** Vue 3, TypeScript, Vue Test Utils, Vitest, existing admin groups API.

---

### Task 1: Add failing component coverage

**Files:**
- Modify: `frontend/src/components/admin/group/__tests__/GroupTagField.spec.ts`
- Create: `frontend/src/views/admin/__tests__/GroupsView.groupTags.spec.ts`

**Steps:**
1. Add a failing `GroupTagField` case for rendering a reusable custom option and emitting its stored color when selected.
2. Add pure helper coverage for deduplicating custom tags, filtering presets/blank values, and choosing a valid color.
3. Run the focused tests and confirm the new assertions fail before implementation.

### Task 2: Implement reusable options in the field

**Files:**
- Modify: `frontend/src/components/admin/group/GroupTagField.vue`
- Modify: `frontend/src/i18n/locales/zh/common.ts`
- Modify: `frontend/src/i18n/locales/en/common.ts`

**Steps:**
1. Add a typed `reusableTags` prop containing tag text, color, and usage count.
2. Render custom options separately from fixed presets, with the existing tag badge style and usage count.
3. Selecting a custom option emits both model value and normalized color; preserve manual input and clear behavior.
4. Add localized labels for the reusable section and usage count.

### Task 3: Load and pass tag options from GroupsView

**Files:**
- Modify: `frontend/src/views/admin/GroupsView.vue`

**Steps:**
1. Add a normalized reusable-tag type and pure deduplication helper.
2. Load all groups including inactive groups on page mount and after create/update/delete/refresh.
3. Ignore empty and preset tags, deduplicate by exact trimmed text, prefer valid colors, and sort by usage then label.
4. Pass the options into create and edit `GroupTagField` instances; keep form submission payloads unchanged.
5. Treat tag-list loading errors as non-fatal.

### Task 4: Verify and commit

**Steps:**
1. Run focused GroupTagField and GroupsView tests.
2. Run i18n completeness, frontend typecheck, and frontend build.
3. Review diff, commit the feature, push the branch, and deploy after health verification.
