# Available Channels Top Tabs Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the desktop left channel rail with a compact horizontal channel tab strip so the toolbar and model grid use the full catalog width.

**Architecture:** Keep the existing channel selection state, mobile picker, model projection, and detail rendering intact. Reshape only `AvailableChannelCatalog.vue`: use a single-column flow, render the desktop listbox as an overflow-safe horizontal tab strip, and adapt loading/empty states and keyboard navigation to the horizontal layout.

**Tech Stack:** Vue 3 Composition API, TypeScript, Tailwind CSS, Vue Test Utils, Vitest

---

### Task 1: Lock the desktop layout contract with failing tests

**Files:**
- Modify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Step 1: Replace the left-rail assertions**

Update the layout test to require:

```ts
const tabsShell = wrapper.get('[data-testid="channel-tabs-shell"]')
const tabs = wrapper.get('[data-testid="channel-navigation"]')

expect(layout.classes()).not.toContain('xl:grid-cols-[240px_minmax(0,1fr)]')
expect(wrapper.find('[data-testid="channel-navigation-shell"]').exists()).toBe(false)
expect(tabsShell.classes()).toEqual(expect.arrayContaining(['hidden', 'xl:block']))
expect(tabs.classes()).toEqual(expect.arrayContaining(['flex', 'overflow-x-auto']))
expect(wrapper.get('[data-testid="channel-nav-item"]').classes()).toContain('shrink-0')
expect(toolbar.classes()).not.toContain('xl:col-start-2')
expect(detail.classes()).not.toContain('xl:col-start-2')
```

Retain checks for the navigation title, channel count, toolbar slot, single detail region, and mobile picker.

**Step 2: Change the keyboard test to horizontal keys**

Require `ArrowRight`, `ArrowLeft`, `Home`, and `End` to update `aria-selected` and move focus. Remove the vertical-arrow expectation from this horizontal navigation contract.

**Step 3: Update loading, empty, refresh, and breakpoint assertions**

Require a `catalog-loading-tabs` skeleton instead of a loading rail; require the empty layout and populated layout not to contain the old 240px grid; require refresh mode to keep the top tabs mounted without row-start classes; and preserve the existing `xl` breakpoint consistency checks for group/price surfaces.

**Step 4: Run the targeted test and confirm failure**

Run:

```bash
npm --prefix frontend run test:run -- src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: FAIL because the component still renders `channel-navigation-shell`, the 240px grid, vertical overflow, and ArrowUp/ArrowDown navigation.

**Step 5: Commit the test contract with the implementation in Task 2**

Do not commit a deliberately failing intermediate state.

### Task 2: Implement the top channel tabs

**Files:**
- Modify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Test: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Step 1: Convert loading and empty states to one column**

Replace the loading rail with a compact desktop-only horizontal skeleton row and keep the full-width detail skeleton below it. Remove the empty state's two-column grid and left `aside`; when the toolbar slot exists, render a compact empty top-tabs shell followed by the full-width toolbar and empty message.

**Step 2: Convert the populated catalog to one column**

Use `relative min-w-0 space-y-4` for the main section. Remove all `xl:col-*`, `xl:row-*`, `xl:col-span-*`, sticky positioning, container-height calculations, and vertical rail overflow classes from navigation, toolbar, warnings, and detail.

**Step 3: Render the desktop listbox as horizontal tabs**

Render `data-testid="channel-tabs-shell"` as a desktop-only full-width bordered surface. Inside it, keep a compact heading/count row and render `data-testid="channel-navigation"` with:

```html
class="flex min-w-0 gap-2 overflow-x-auto px-3 pb-3 ..."
```

Each `channel-nav-item` is a `shrink-0` compact option with a bounded width, one concise channel-name line, platform badges, and group/model counts. Keep `role="listbox"`, `role="option"`, `aria-selected`, roving `tabindex`, `aria-controls`, reduced-motion handling, and the active primary-color treatment.

**Step 4: Adapt keyboard navigation**

Map `ArrowRight` to the next channel and `ArrowLeft` to the previous channel while retaining `Home` and `End`. Continue focusing the newly selected option after `nextTick`.

**Step 5: Run the targeted test and confirm success**

Run:

```bash
npm --prefix frontend run test:run -- src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
```

Expected: PASS.

**Step 6: Commit the UI change**

```bash
git add frontend/src/components/channels/AvailableChannelCatalog.vue frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts
git commit -m "fix: replace available channel rail with top tabs"
```

### Task 3: Verify frontend quality gates

**Files:**
- Verify: `frontend/src/components/channels/AvailableChannelCatalog.vue`
- Verify: `frontend/src/components/channels/__tests__/AvailableChannelCatalog.spec.ts`

**Step 1: Run lint without mutations**

```bash
npm --prefix frontend run lint:check
```

Expected: exit code 0.

**Step 2: Run TypeScript checks**

```bash
npm --prefix frontend run typecheck
```

Expected: exit code 0.

**Step 3: Run the full frontend test suite**

```bash
npm --prefix frontend run test:run
```

Expected: all tests pass.

**Step 4: Produce the frontend build**

```bash
npm --prefix frontend run build
```

Expected: Vue TypeScript compilation and Vite production build both succeed.

### Task 4: Rebuild and smoke-test the local Docker app

**Files:**
- Verify: `deploy/docker-compose.dev.yml`

**Step 1: Rebuild only the application image**

From `deploy/`, run the same local environment and port overrides already used by the active `sub2api-dev` stack:

```bash
docker compose -f docker-compose.dev.yml build sub2api
```

Expected: a new local `sub2api` image is built successfully.

**Step 2: Recreate only the application container**

```bash
docker compose -f docker-compose.dev.yml up -d --no-deps --force-recreate sub2api
```

Expected: PostgreSQL and Redis remain running and only `sub2api-dev` is recreated.

**Step 3: Verify service health and the served frontend asset**

```bash
docker inspect --format '{{.State.Health.Status}}' sub2api-dev
curl -fsS http://127.0.0.1:18080/health
```

Expected: container reports `healthy` and health endpoint returns `{"status":"ok"}`. Confirm the served JavaScript chunk contains `channel-tabs-shell` and no old `channel-navigation-shell` marker.

**Step 4: Inspect repository status**

```bash
git status --short
```

Expected: only the pre-existing untracked root `package.json` and `package-lock.json` remain; they are not staged or modified.
