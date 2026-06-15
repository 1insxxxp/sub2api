# Homepage Scroll Motion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a subtle premium scroll-loading motion pass to the default home page without changing the layout or custom home content behavior.

**Architecture:** Keep the implementation CSS-first inside `HomeView.vue`. Add semantic motion classes to existing homepage sections, refine the existing reveal keyframes and stagger delays, and extend the current unit test so custom-home bypass and default motion hooks remain protected.

**Tech Stack:** Vue 3, Vite, Tailwind utility classes, scoped CSS, Vitest, Vue Test Utils.

---

### Task 1: Protect The Motion Hooks

**Files:**
- Modify: `frontend/src/views/__tests__/HomeView.spec.ts`

**Step 1: Read the existing test**

Run:

```bash
sed -n '100,160p' frontend/src/views/__tests__/HomeView.spec.ts
```

Expected: find the existing `renders motion hooks for the default homepage experience` test.

**Step 2: Extend the test expectations**

Add checks for the new section-level and panel-level hooks:

```ts
expect(wrapper.findAll('.home-section-reveal').length).toBeGreaterThanOrEqual(6)
expect(wrapper.find('.home-code-panel.home-scroll-reveal').exists()).toBe(true)
expect(wrapper.find('.home-cta-panel.home-scroll-reveal').exists()).toBe(true)
```

Keep the existing assertions for `.home-motion-root`, `.home-grid-bg`, `.home-reveal`, `.home-scroll-reveal`, and `.home-motion-card`.

**Step 3: Run the focused test**

Run:

```bash
pnpm --dir frontend run test -- HomeView
```

Expected: fail before implementation because `.home-section-reveal` is not present yet.

### Task 2: Add Section Reveal Classes

**Files:**
- Modify: `frontend/src/views/HomeView.vue`

**Step 1: Add section reveal hooks**

Add `home-section-reveal` and small delay classes to the non-hero section copy blocks:

```vue
<p class="home-section-reveal home-section-reveal-1 ...">
<h2 class="home-section-reveal home-section-reveal-2 ...">
<p class="home-section-reveal home-section-reveal-3 ...">
```

Apply this to:

- Capabilities section eyebrow, title, subtitle.
- Integration section eyebrow, title, subtitle.
- Workflow section eyebrow, title, subtitle.
- Final CTA title and subtitle.

**Step 2: Run the focused test**

Run:

```bash
pnpm --dir frontend run test -- HomeView
```

Expected: the motion-hook test passes.

### Task 3: Refine Scroll Motion CSS

**Files:**
- Modify: `frontend/src/views/HomeView.vue`

**Step 1: Tune shared motion variables**

Use calm defaults:

```css
.home-motion-root {
  --motion-distance: 16px;
  --motion-duration: 760ms;
  --motion-ease: cubic-bezier(0.16, 1, 0.3, 1);
  --motion-section-distance: 14px;
}
```

**Step 2: Add section reveal styling**

```css
.home-section-reveal {
  animation: home-section-rise 720ms var(--motion-ease) both;
  animation-timeline: view();
  animation-range: entry 10% cover 28%;
}

.home-section-reveal-1 {
  animation-delay: 0ms;
}

.home-section-reveal-2 {
  animation-delay: 55ms;
}

.home-section-reveal-3 {
  animation-delay: 110ms;
}
```

**Step 3: Refine existing scroll reveal**

Keep `--motion-index`, but reduce intensity:

```css
.home-scroll-reveal {
  animation: home-rise-in 720ms var(--motion-ease) both;
  animation-delay: calc(60ms + (var(--motion-index) * 42ms));
  animation-timeline: view();
  animation-range: entry 9% cover 26%;
}
```

**Step 4: Add the new keyframe**

```css
@keyframes home-section-rise {
  from {
    opacity: 0;
    transform: translateY(var(--motion-section-distance));
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}
```

**Step 5: Tune mobile motion**

Add a small-screen override:

```css
@media (max-width: 640px) {
  .home-motion-root {
    --motion-distance: 12px;
    --motion-section-distance: 10px;
  }

  .home-scroll-reveal,
  .home-section-reveal {
    animation-delay: 40ms;
  }
}
```

The existing `prefers-reduced-motion` block should remain at the bottom and continue to override animations.

### Task 4: Verify The Homepage

**Files:**
- Verify: `frontend/src/views/HomeView.vue`
- Verify: `frontend/src/views/__tests__/HomeView.spec.ts`

**Step 1: Run focused test**

Run:

```bash
pnpm --dir frontend run test -- HomeView
```

Expected: pass.

**Step 2: Run typecheck**

Run:

```bash
pnpm --dir frontend run typecheck
```

Expected: pass.

**Step 3: Run frontend build**

Run:

```bash
pnpm --dir frontend run build
```

Expected: pass.

**Step 4: Check the running local page**

Open:

```text
http://127.0.0.1:3000/
```

Expected: homepage keeps the same visual style, with calmer section-level reveal and no horizontal overflow on mobile.

### Task 5: Commit Implementation

**Files:**
- Modify: `frontend/src/views/HomeView.vue`
- Modify: `frontend/src/views/__tests__/HomeView.spec.ts`

**Step 1: Inspect diff**

Run:

```bash
git diff -- frontend/src/views/HomeView.vue frontend/src/views/__tests__/HomeView.spec.ts
```

Expected: only homepage motion hooks, CSS, and related tests changed.

**Step 2: Commit**

Run:

```bash
git add frontend/src/views/HomeView.vue frontend/src/views/__tests__/HomeView.spec.ts
git commit -m "feat: refine homepage scroll motion"
```

Expected: implementation commit created on `codex/homepage-scroll-motion`.
