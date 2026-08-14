# API Key Group Selector Width Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Increase the desktop API key group selector to 480px while retaining its mobile bottom-sheet layout and viewport containment.

**Architecture:** Keep the existing selector and positioning flow in `KeysView.vue`. Change the desktop Tailwind width and the single positioning estimate together so rendering and edge clamping use one matching size; verify both through the existing source-level layout specification.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest

---

### Task 1: Widen the desktop selector

**Files:**
- Modify: `frontend/src/views/user/KeysView.vue`
- Test: `frontend/src/views/user/__tests__/KeysViewLayout.spec.ts`

**Step 1: Write the failing test**

Add assertions to the group-selector layout test:

```ts
expect(keysViewSource).toContain("'w-[480px] slide-in-from-top-2'")
expect(keysViewSource).toContain('const dropdownEstWidth = Math.min(480,')
expect(keysViewSource).not.toContain("'w-[380px] slide-in-from-top-2'")
```

**Step 2: Run the test to verify it fails**

Run:

```bash
cd frontend
pnpm exec vitest run src/views/user/__tests__/KeysViewLayout.spec.ts
```

Expected: FAIL because `KeysView.vue` still uses `380px` for the desktop class and positioning estimate.

**Step 3: Implement the minimal change**

In `KeysView.vue`:

```vue
: 'w-[480px] slide-in-from-top-2'
```

and:

```ts
const dropdownEstWidth = Math.min(480, window.innerWidth - dropdownViewportPadding * 2)
```

Do not change the mobile bottom-sheet branch.

**Step 4: Run focused and static verification**

Run:

```bash
cd frontend
pnpm exec vitest run src/views/user/__tests__/KeysViewLayout.spec.ts src/views/user/__tests__/KeysView.spec.ts
pnpm exec eslint src/views/user/KeysView.vue src/views/user/__tests__/KeysViewLayout.spec.ts
pnpm exec vue-tsc --noEmit
```

Expected: all tests and static checks pass.

**Step 5: Commit**

```bash
git add frontend/src/views/user/KeysView.vue frontend/src/views/user/__tests__/KeysViewLayout.spec.ts
git commit -m "fix: widen API key group selector"
```

