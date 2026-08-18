# Channel Monitor Mobile Cards Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make every Channel Monitor V2 matrix row readable as a three-band card on screens up to 640px while preserving the desktop matrix.

**Architecture:** Keep one `alignedRows` render path. Add stable classes and mobile labels to the existing dimension/summary/pulse nodes, wrap the pulse track in its own scroll shell, and use a mobile-only two-column card grid with explicit placement. Desktop selectors retain the existing six-column matrix.

**Tech Stack:** Vue 3 SFC, TypeScript, scoped CSS, Tailwind utility classes, Vitest, Vue Test Utils.

---

### Task 1: Lock the mobile card contract with failing tests

**Files:**
- Modify: `frontend/src/features/channel-monitor-v2/__tests__/RelayPulseMatrix.spec.ts`
- Modify: `frontend/src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts`

**Step 1: Write the failing component test**

Add a test that mounts a row with a long group name and asserts:

```ts
expect(wrapper.find('.dimension-label').text()).toContain(longGroupName)
expect(wrapper.findAll('.summary-value').map((node) => node.attributes('data-mobile-label')))
  .toEqual(['成功率', '首 Token', '每秒 Token', '缓存率'])
expect(wrapper.find('.mobile-pulse-axis').text()).toContain('08/01')
expect(wrapper.find('.pulse-track-shell').exists()).toBe(true)
```

**Step 2: Write the failing structure test**

Require mobile card hooks and explicit overflow ownership:

```ts
expect(src).toContain('.dimension-label')
expect(src).toContain('.mobile-pulse-axis')
expect(src).toContain('.pulse-track-shell')
expect(src).toContain('overflow-wrap: anywhere')
expect(src).toContain('grid-column: 1 / -1')
```

**Step 3: Run tests to verify RED**

Run:

```bash
cd frontend
pnpm exec vitest run \
  src/features/channel-monitor-v2/__tests__/RelayPulseMatrix.spec.ts \
  src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts
```

Expected: FAIL because the mobile card hooks and labels do not exist.

### Task 2: Implement the responsive card layout

**Files:**
- Modify: `frontend/src/features/channel-monitor-v2/RelayPulseMatrix.vue`

**Step 1: Add stable semantic hooks**

- Add `dimension-label` to the row label.
- Add `:data-mobile-label` to each existing summary cell using current i18n keys.
- Wrap `.pulse-track` in `.pulse-track-shell`.
- Add a mobile-only `.mobile-pulse-axis` with `axisStart` and `axisEnd`.

The existing numeric formatters remain the sole source of metric values.

**Step 2: Replace the broken mobile grid**

Within `@media (max-width: 640px)`:

```css
.matrix-header { display: none; }
.matrix-table { min-width: 0 !important; }
.matrix-row {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
}
.dimension-cell,
.mobile-pulse-axis,
.pulse-track-shell { grid-column: 1 / -1; }
.dimension-label {
  white-space: normal;
  overflow-wrap: anywhere;
}
.pulse-track-shell { overflow-x: auto; }
```

Style each body row as a bordered card and each summary metric as a compact label/value pair. Remove the old four-column mobile override and the unused `left` declarations.

**Step 3: Run focused tests to verify GREEN**

Run the Task 1 Vitest command.

Expected: both files PASS.

**Step 4: Commit the implementation**

```bash
git add frontend/src/features/channel-monitor-v2/RelayPulseMatrix.vue \
  frontend/src/features/channel-monitor-v2/__tests__/RelayPulseMatrix.spec.ts \
  frontend/src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts
git commit -m "fix: adapt channel monitor matrix for mobile"
```

### Task 3: Verify desktop behavior and production quality

**Files:**
- No production file changes expected.

**Step 1: Run the full feature suite**

```bash
cd frontend
pnpm exec vitest run src/features/channel-monitor-v2
```

Expected: all Channel Monitor V2 tests PASS.

**Step 2: Run static gates**

```bash
pnpm run typecheck:tests
pnpm run typecheck
pnpm run lint:check
pnpm run build
```

Expected: all commands exit 0.

**Step 3: Run mobile visual verification**

Build/restart the local app from the feature branch, open `/channel-status` (or the authenticated equivalent) at approximately 390px width, and verify:

- full platform/group/model name is readable;
- metrics form a stable two-column grid;
- the pulse track owns horizontal overflow only when zoomed;
- the page itself has no horizontal overflow;
- desktop width still shows the original table.

**Step 4: Inspect final diff**

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only expected commits/files.
