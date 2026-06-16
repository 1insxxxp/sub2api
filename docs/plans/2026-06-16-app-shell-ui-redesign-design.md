# App Shell UI Redesign Design

## Background

The home page now has a polished blue-violet developer-platform direction, but the authenticated app still uses older shell, card, and table styling. The next UI pass should make the logged-in product feel like the same product as the refreshed home page without rewriting every feature page at once.

## Goals

- Establish a consistent visual foundation for user and admin pages.
- Preserve existing routes, behavior, permissions, feature flags, and data loading.
- Keep light mode, dark mode, and mobile compatibility.
- Improve density, scanability, and perceived quality for operational workflows.
- Keep changes local to shared layout and reusable style primitives first.

## Non-Goals

- No backend changes.
- No pricing, billing, auth, routing, or feature-flag behavior changes.
- No full per-page redesign in this phase.
- No production deployment as part of this local UI work.

## Recommended Direction

Use a refined SaaS operations style: clean blue-slate surfaces, subtle depth, crisp active states, compact controls, and restrained motion. The UI should feel reliable for an API gateway and admin console, not like a marketing page inside the app.

## Scope

### Global App Shell

- `frontend/src/components/layout/AppLayout.vue`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/components/layout/AppHeader.vue`
- `frontend/src/components/layout/TablePageLayout.vue`
- `frontend/src/style.css`

The shell should introduce:

- A calmer page background in light and dark modes.
- A more premium sidebar with clearer active states and grouped navigation.
- A sticky header with better contrast, compact utilities, and mobile-safe spacing.
- More consistent page padding and responsive constraints.
- Refined table containers for admin-heavy screens.

### Shared Primitives

Global component classes should be tuned instead of rewriting each view:

- `.card`, `.glass-card`, `.stat-card`
- `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost`
- `.input`
- `.table`, `.table-container`, table rows and headers
- `.badge`
- `.dropdown`
- `.tabs`
- `.empty-state`, `.skeleton`

This lets existing user and admin pages inherit the new style with minimal churn.

## Visual Rules

- Keep cards at a practical radius and avoid nested card-heavy layouts.
- Use blue as the main product accent, with violet and cyan only as highlights.
- Keep success, warning, and danger colors semantic.
- Preserve readable contrast in both themes.
- Avoid large decorative blobs inside app pages.
- Use motion only for state changes, hover feedback, dropdowns, and page-level polish.
- Ensure long Chinese and English labels truncate or wrap cleanly.

## Responsiveness

Desktop should prioritize dense scanning: compact side navigation, readable tables, and stable header actions. Mobile should keep the sidebar overlay behavior, avoid horizontal overflow, and keep user actions accessible from the header/dropdown.

## Testing And Verification

Automated checks:

- Existing layout tests under `frontend/src/components/layout/__tests__`.
- Targeted tests for pages that depend heavily on table layout if snapshots or structure assertions exist.
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run build`

Manual checks on local hot-reload server:

- `http://127.0.0.1:18080/home`
- User dashboard, keys, usage, profile.
- Admin dashboard, users, accounts, channel monitor, checkins, settings.
- Light and dark modes.
- Mobile viewport around 360px.
- Desktop viewport around 1280px.

## Rollout

This phase is frontend-only and should be reviewed locally. Production deployment should remain a separate blue/green or otherwise non-interrupting step.
