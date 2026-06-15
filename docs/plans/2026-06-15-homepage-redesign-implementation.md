# Homepage Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the default home page into a bilingual acquisition landing page with a strong login/dashboard entry, while preserving custom home content, light/dark mode, and mobile responsiveness.

**Architecture:** Keep the change frontend-only. Replace the default `HomeView.vue` landing layout with a structured SaaS/developer-platform page that reads all visible text from existing i18n locale files. Reuse current stores, theme toggle, locale switcher, authentication state, site settings, and documentation URL logic.

**Tech Stack:** Vue 3, TypeScript, Vue Router, vue-i18n, Pinia stores, Tailwind CSS, existing icon system.

---

### Task 1: Add Bilingual Home Page Copy

**Files:**
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Step 1: Inspect existing `home` locale shape**

Run:

```bash
rg "home:" frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
```

Expected: find the current `home` object in both locale files.

**Step 2: Add the new locale keys**

Extend the existing `home` namespace instead of creating a new top-level namespace.

Add keys for:

- `nav.features`
- `nav.integration`
- `nav.workflow`
- `hero.eyebrow`
- `hero.title`
- `hero.subtitle`
- `hero.primaryCta`
- `hero.dashboardCta`
- `hero.secondaryCta`
- `hero.statusBadge`
- `hero.panelTitle`
- `hero.panelSubtitle`
- `hero.routeLabel`
- `hero.billingLabel`
- `hero.latencyLabel`
- `hero.successLabel`
- `hero.channels.openai`
- `hero.channels.gemini`
- `hero.channels.anthropic`
- `hero.channels.grok`
- `trust.multiModel.title`
- `trust.multiModel.desc`
- `trust.routing.title`
- `trust.routing.desc`
- `trust.billing.title`
- `trust.billing.desc`
- `trust.monitoring.title`
- `trust.monitoring.desc`
- `sections.capabilitiesEyebrow`
- `sections.capabilitiesTitle`
- `sections.capabilitiesSubtitle`
- `capabilities.unifiedApi.title`
- `capabilities.unifiedApi.desc`
- `capabilities.accountPool.title`
- `capabilities.accountPool.desc`
- `capabilities.monitoring.title`
- `capabilities.monitoring.desc`
- `capabilities.wallet.title`
- `capabilities.wallet.desc`
- `capabilities.keys.title`
- `capabilities.keys.desc`
- `capabilities.risk.title`
- `capabilities.risk.desc`
- `integration.eyebrow`
- `integration.title`
- `integration.subtitle`
- `integration.replaceBaseUrl`
- `integration.useApiKey`
- `integration.monitorCost`
- `workflow.eyebrow`
- `workflow.title`
- `workflow.subtitle`
- `workflow.step1.title`
- `workflow.step1.desc`
- `workflow.step2.title`
- `workflow.step2.desc`
- `workflow.step3.title`
- `workflow.step3.desc`
- `cta.title`
- `cta.subtitle`
- `footer.tagline`

Use concise Chinese and English copy. Do not add pricing copy.

**Step 3: Run typecheck**

Run:

```bash
pnpm --dir frontend run typecheck
```

Expected: PASS. If it fails because of locale syntax, fix the locale object.

**Step 4: Commit**

```bash
git add frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: add homepage redesign copy"
```

---

### Task 2: Rebuild The Default Home Page Layout

**Files:**
- Modify: `frontend/src/views/HomeView.vue`

**Step 1: Inspect current dependencies and computed state**

Run:

```bash
Get-Content -LiteralPath 'frontend/src/views/HomeView.vue' -TotalCount 420
```

Expected: identify existing `homeContent`, `isHomeContentUrl`, `siteName`, `siteLogo`, `siteSubtitle`, `docUrl`, `isAuthenticated`, `dashboardPath`, `toggleTheme`, and `LocaleSwitcher` usage.

**Step 2: Preserve custom home content branch**

Keep this behavior at the top of the template:

- If `homeContent` exists and is a URL, render the full-page iframe.
- If `homeContent` exists and is HTML, render it with `v-html`.
- Only show the redesigned landing page when `homeContent` is empty.

**Step 3: Replace the default page template**

Create these sections in order:

- Header with logo/site name, desktop anchor links, docs link, language switcher, theme toggle, and auth-aware CTA.
- Hero with eyebrow, title, subtitle, primary CTA, secondary docs CTA, and product routing panel.
- Trust bar with four compact capability items.
- Capabilities grid with six product capability blocks.
- Developer integration section with OpenAI-compatible request example and three migration bullets.
- Workflow section with three steps.
- Final CTA and footer.

Use existing `Icon` and `LocaleSwitcher` components where possible.

**Step 4: Build data arrays in `<script setup>`**

Add computed arrays for:

- `trustItems`
- `capabilityItems`
- `workflowSteps`
- `channelRows`
- `requestRows`
- `integrationPoints`

Each item should reference `t(...)` keys from Task 1.

**Step 5: Keep theme and auth logic intact**

Do not remove or rewrite:

- `toggleTheme`
- `isDark`
- `isAuthenticated`
- `dashboardPath`
- `userInitial`
- `siteName`
- `siteLogo`
- `docUrl`
- `homeContent`
- `isHomeContentUrl`

Expected: old functional behavior remains available.

**Step 6: Commit**

```bash
git add frontend/src/views/HomeView.vue
git commit -m "feat: redesign default homepage"
```

---

### Task 3: Verify Responsive And Theme Behavior Locally

**Files:**
- Modify only if verification finds layout defects:
  - `frontend/src/views/HomeView.vue`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/i18n/locales/en.ts`

**Step 1: Run static checks**

Run:

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

Expected: both PASS.

**Step 2: Start or reuse the local dev server**

If Docker/local app is already running on `http://127.0.0.1:18080`, use it. Otherwise start the existing frontend/dev stack according to the repo's current development setup.

Expected: home page is reachable locally.

**Step 3: Browser verification**

Use the in-app browser or Playwright to verify:

- Desktop light mode: `1280x800`.
- Desktop dark mode: `1280x800`.
- Mobile light mode: `390x844`.
- Mobile dark mode: `390x844`.
- Chinese locale.
- English locale.
- Guest CTA goes to login/register path as implemented.
- Authenticated CTA still goes to dashboard.
- No horizontal overflow on mobile.

**Step 4: Fix any visual defects**

If any text wraps badly, panels overlap, or mobile layout overflows:

- Adjust responsive grid classes.
- Reduce hero type size on mobile.
- Use `min-w-0`, `break-words`, and stable panel widths where needed.
- Keep dark mode contrast readable.

**Step 5: Final commit**

```bash
git add frontend/src/views/HomeView.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "fix: polish homepage responsive states"
```

Only make this commit if Step 4 changed files.

---

### Task 4: Final Verification And Handoff

**Files:**
- No planned file changes.

**Step 1: Confirm git state**

Run:

```bash
git status --short
```

Expected: no modified tracked files. Existing ignored or unrelated deployment artifacts may remain untracked and should not be committed.

**Step 2: Summarize verification evidence**

Collect:

- Typecheck result.
- Build result.
- Browser/mobile checks.
- Any known limitations.

**Step 3: Stop before production deployment**

Do not deploy this UI change unless the user explicitly asks for deployment after local review.

Expected handoff: local home page is ready for user review, and content pages remain unchanged.
