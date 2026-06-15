# Homepage Redesign Design

## Background

The default home page currently works as a simple product entry page, but its visual style is closer to a template landing page than a polished SaaS gateway site. The next UI iteration should start from the home page, then carry the resulting style into user and admin pages.

This redesign targets the default home page only. Existing custom home content support remains unchanged: when `homeContent` is configured, the iframe or HTML mode still takes priority.

## Goals

- Present the site as a professional AI API gateway for external acquisition.
- Keep the home page efficient for existing users who want to log in or enter the dashboard.
- Support Chinese and English through the existing i18n system.
- Preserve light mode, dark mode, and the existing theme toggle.
- Preserve responsive behavior on mobile, tablet, and desktop.
- Avoid adding a pricing or plans section in this phase.
- Minimize risk by changing the default home page and locale copy first, without touching backend logic.

## Non-Goals

- No pricing table or plan comparison in this phase.
- No changes to authentication, billing, routing, channel, or admin logic.
- No redesign of login, user dashboard, or admin pages yet.
- No removal of custom home content behavior.

## Audience

The page serves two audiences:

- New visitors evaluating whether the service is reliable enough for API traffic.
- Existing users returning to log in, view documentation, or enter the dashboard.

## Recommended Direction

Use a professional overseas SaaS and developer-platform style:

- Light background with restrained gray surfaces.
- Dark product panel as the hero visual focus.
- Teal/green as a brand accent, not a full-page dominant gradient.
- Fewer decorative glow blobs and oversized rounded cards.
- Clear hierarchy, compact product proof, and practical calls to action.

The page should feel closer to a reliable developer platform than a generic marketing page.

## Page Structure

### 1. Header

The header keeps the existing site settings and utilities:

- Site logo and site name from current public settings.
- Language switcher.
- Theme toggle.
- Documentation link when configured.
- Login button for guests.
- Dashboard button for authenticated users.

On mobile, secondary links collapse or hide so the primary action remains visible.

### 2. Hero

The hero positions the product clearly:

- Chinese headline: `稳定接入多模型 API 的统一网关`
- English headline: `A unified gateway for reliable multi-model API access`
- Supporting copy explains OpenAI-compatible access, multi-channel routing, balance billing, monitoring, and risk controls.
- Primary CTA:
  - Guest: start using or log in.
  - Authenticated user: enter dashboard.
- Secondary CTA: documentation.

The right side becomes a product-style routing panel instead of a generic terminal window. It should show realistic gateway concepts:

- Model providers or channels with status chips.
- Request route result.
- Latency and success state.
- Balance billing event.
- Small request log rows.

On mobile, this panel stacks below the copy and simplifies into compact cards.

### 3. Trust Bar

A short row of capability proof:

- Multi-model access.
- Smart channel routing.
- Real-time billing.
- Monitoring and risk control.

These are qualitative product capabilities, not inflated public metrics.

### 4. Capability Sections

Six concise capability blocks:

- Unified API access.
- Account pool management.
- Channel health monitoring.
- Wallet and balance billing.
- User and API key management.
- Risk control center.

Each block uses a clear icon, title, and one short sentence.

### 5. Developer Integration

Show an OpenAI-compatible request example and migration points:

- Keep client code mostly unchanged.
- Replace Base URL.
- Use the platform API key.
- Monitor requests and cost from the dashboard.

This section should support both Chinese and English copy.

### 6. Usage Flow

Show three steps:

1. Register or log in.
2. Create an API key and prepare balance.
3. Replace Base URL and start calling.

### 7. Final CTA And Footer

Close with:

- Start using or enter dashboard.
- View documentation.
- Login.
- Legal/document links when available.

## Responsiveness

The home page must remain comfortable across these viewports:

- Mobile: 360px and up.
- Tablet: 768px and up.
- Desktop: 1280px and up.

Mobile rules:

- No horizontal overflow.
- Hero content appears before the visual panel.
- Navigation keeps the primary CTA visible.
- Feature grids collapse to one column.
- Long English text wraps cleanly.

## Theme Support

The page must support both light and dark themes with the existing `dark` class strategy.

Light mode:

- White and gray backgrounds.
- Dark text.
- Teal accents.
- Subtle borders.

Dark mode:

- Deep neutral background.
- Muted borders.
- Teal accents with lower saturation.
- Product panel remains legible without excessive glow.

## i18n

All new visible text should live in locale files:

- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Do not hard-code Chinese or English copy in `HomeView.vue`.

## Implementation Scope

Initial implementation should change:

- `frontend/src/views/HomeView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Optional small global style changes are allowed only if they are clearly reusable and do not disturb existing content pages.

## Testing And Verification

Run:

- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run build`

Manual verification:

- Guest home page in light mode.
- Guest home page in dark mode.
- Authenticated home page CTA state.
- Chinese locale.
- English locale.
- Mobile width around 360px.
- Desktop width around 1280px.
- Existing custom home content mode still bypasses the default page.

## Rollout Plan

This is a frontend-only change. It should be reviewed locally before deployment. Production deployment should follow the existing non-interrupting blue/green approach used for previous releases.
