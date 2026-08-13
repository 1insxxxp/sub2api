# System Custom Group Independent Scroll Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the system custom group source and model columns scroll independently while showing every selected source as a navigable model section.

**Architecture:** Keep `selectedSourceIDs` and `selectedCandidates` as the single source of truth. Restructure only the desktop workspace so each column owns an `overflow-y-auto overscroll-contain` child, render all selected candidates in the right column, and use refs to scroll only the right model container when a source navigation button is clicked. Preserve the existing stacked mobile layout and all route draft/snapshot behavior.

**Tech Stack:** Vue 3 Composition API, TypeScript, Tailwind CSS, Vue Test Utils, Vitest.

---

### Task 1: Reproduce the independent-scroll and multi-source contract

**Files:**
- Modify: `frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts`

**Step 1: Write the failing layout test**

Add a test which mounts the create dialog, selects source IDs `11` and `22`, and asserts:

```ts
expect(wrapper.get('[data-testid="system-custom-source-scroll"]').classes()).toContain('overflow-y-auto')
expect(wrapper.get('[data-testid="system-custom-source-scroll"]').classes()).toContain('overscroll-contain')
expect(wrapper.get('[data-testid="system-custom-model-scroll"]').classes()).toContain('overflow-y-auto')
expect(wrapper.get('[data-testid="system-custom-model-scroll"]').classes()).toContain('overscroll-contain')
expect(wrapper.findAll('[data-testid="system-custom-source-section"]')).toHaveLength(2)
expect(wrapper.findAll('[data-testid="system-custom-source-nav"]')).toHaveLength(2)
```

Also assert the two sections carry source IDs `11` and `22` and contain their respective model rows.

**Step 2: Run the test to verify RED**

Run:

```bash
pnpm --dir frontend vitest run src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts -t "scrolls source and model columns independently"
```

Expected: FAIL because the new scroll containers, sections, and source navigation do not exist.

### Task 2: Reproduce right-column-only source navigation

**Files:**
- Modify: `frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts`

**Step 1: Write the failing navigation test**

Mock the right scroll element and the source section geometry, click the navigation button for source `22`, and assert that the right model container receives `scrollTo` while the source scroll container does not. Confirm both source checkboxes remain selected and both source sections remain rendered.

```ts
await wrapper.get('[data-testid="system-custom-source-nav"][data-source-id="22"]').trigger('click')
expect(modelScrollTo).toHaveBeenCalledTimes(1)
expect(sourceScrollTo).not.toHaveBeenCalled()
expect(wrapper.findAll('[data-testid="system-custom-source-section"]')).toHaveLength(2)
```

**Step 2: Run the navigation test to verify RED**

Run:

```bash
pnpm --dir frontend vitest run src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts -t "navigates inside the model column"
```

Expected: FAIL because the source navigation button and scroll handler do not exist.

### Task 3: Implement the desktop workspace layout

**Files:**
- Modify: `frontend/src/components/admin/groups/SystemCustomGroupDialog.vue`

**Step 1: Give the desktop grid a bounded height**

Change the workspace grid to use a desktop-only fixed height and min-height containment:

```html
<div class="grid gap-4 lg:h-[34rem] lg:min-h-0 lg:grid-cols-[15rem_minmax(0,1fr)]">
```

**Step 2: Split the source card header from its scroll child**

Make the aside a desktop flex column with hidden overflow. Put the loading/empty/candidate list inside a child marked `data-testid="system-custom-source-scroll"` with desktop `min-h-0 flex-1 overflow-y-auto overscroll-contain`. On narrow screens, keep natural height.

**Step 3: Split the model card header/navigation from its scroll child**

Make the model section a desktop flex column with hidden overflow. Add a compact selected-source navigation row derived from `selectedCandidates`. Put the empty state or all source sections inside a child marked `data-testid="system-custom-model-scroll"` with desktop `min-h-0 flex-1 overflow-y-auto overscroll-contain`.

**Step 4: Mark every selected source section**

Render the existing `v-for="candidate in selectedCandidates"` with:

```html
<section
  :ref="(element) => setSourceSectionRef(candidate.group.id, element)"
  :data-source-id="candidate.group.id"
  data-testid="system-custom-source-section"
>
```

Keep all sections in the DOM simultaneously. Make each source heading sticky inside the model scroll container.

### Task 4: Implement right-column source navigation

**Files:**
- Modify: `frontend/src/components/admin/groups/SystemCustomGroupDialog.vue`

**Step 1: Store DOM refs without creating a second business state**

Add:

```ts
const modelScrollRef = ref<HTMLElement | null>(null)
const sourceSectionRefs = new Map<number, HTMLElement>()
```

Implement `setSourceSectionRef` to add/remove only DOM element refs.

**Step 2: Scroll within the right model container**

Implement `scrollToSource(sourceID)` by resolving the right container and section and calling:

```ts
container.scrollTo({
  top: Math.max(0, section.offsetTop - container.offsetTop),
  behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
})
```

The handler must not change `selectedSourceIDs`, route drafts, or the outer dialog scroll position.

**Step 3: Clear DOM refs with the dialog session**

Clear `sourceSectionRefs` during `reset()` so close/reopen and group switches cannot retain stale elements.

### Task 5: Verify and commit the fix

**Files:**
- Modify: `frontend/src/components/admin/groups/SystemCustomGroupDialog.vue`
- Modify: `frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts`

**Step 1: Run focused tests**

```bash
pnpm --dir frontend vitest run src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts
```

Expected: all dialog tests PASS.

**Step 2: Run related and full frontend gates**

```bash
pnpm --dir frontend vitest run \
  src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts \
  src/views/admin/__tests__/GroupsView.spec.ts \
  src/api/__tests__/admin.groups.systemCustom.spec.ts
pnpm --dir frontend run typecheck:tests
pnpm --dir frontend run typecheck
pnpm --dir frontend run lint:check
pnpm --dir frontend run test:run
pnpm --dir frontend run build
```

Expected: all commands exit `0`; full Vitest has no failed tests.

**Step 3: Commit**

```bash
git add frontend/src/components/admin/groups/SystemCustomGroupDialog.vue \
  frontend/src/components/admin/groups/__tests__/SystemCustomGroupDialog.spec.ts
git commit -m "fix: separate custom group model scrolling"
```

### Task 6: Rebuild and smoke-test the local application container

**Files:**
- No source changes expected.

**Step 1: Build the app image from this worktree**

```bash
docker compose --env-file /Users/alien/Documents/sub2/deploy/.env \
  -f deploy/docker-compose.dev.yml build sub2api
```

**Step 2: Replace only the app container**

```bash
docker compose --env-file /Users/alien/Documents/sub2/deploy/.env \
  -f /Users/alien/Documents/sub2/deploy/docker-compose.dev.yml \
  up -d --no-deps --force-recreate sub2api
```

Confirm the PostgreSQL and Redis container IDs did not change.

**Step 3: Verify health and layout**

Check `/health`, container logs, and—when an authenticated local admin session is available—the two independent scroll regions and simultaneous multi-source sections in `/admin/groups`.
