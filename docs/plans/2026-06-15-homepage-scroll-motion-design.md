# Homepage Scroll Motion Design

## Background

The current default home page already has a polished SaaS/developer-platform style with subtle reveal animation, routing-panel sheen, status pulses, and card hover motion. The next pass should preserve that visual direction and only refine how content appears while scrolling.

This change applies only to the default home page. Custom home content remains untouched: if `homeContent` is configured, the iframe or custom HTML mode still bypasses the default page.

## Goal

Improve the default home page's scroll-loading feel with understated, premium motion that supports reading instead of competing with it.

## Selected Direction

Use a CSS-first motion refinement.

Reasons:

- The current implementation already has motion classes in `HomeView.vue`.
- CSS keeps the change small and avoids extra runtime state.
- The desired motion level is light, so IntersectionObserver or a shared composable would be more machinery than the feature needs.
- Existing `prefers-reduced-motion` behavior can continue to protect users who disable animation.

## Motion Principles

- Keep movement short and calm.
- Use opacity, small vertical offset, and tiny scale changes only where they add polish.
- Avoid large parallax, scroll-jacking, cursor-following effects, or looping section animations.
- Stagger repeated cards by a small delay so sections feel intentional.
- Make code and routing panels feel active through subtle sheen and highlight timing, not heavy motion.
- Reduce movement further on mobile.

## Proposed Changes

### Section Reveal

Add a lightweight section-heading reveal style for titles, subtitles, and eyebrow labels in the non-hero sections. These elements should rise slightly and fade in before the related cards.

### Card Stagger

Refine `home-scroll-reveal` so repeated trust, capability, integration, workflow, and CTA items enter with a calmer distance and stagger. Existing `--motion-index` values should remain the source of per-card delay.

### Product Panels

Keep the routing panel and code panel sheen, but tune timing so it feels like a quiet surface highlight. The animation should remain decorative and should not draw attention away from the CTA or section copy.

### CTA Focus

When the final CTA panel enters the viewport, add one subtle focus moment through fade, lift, and shadow/color settling. It should not pulse or repeat.

### Mobile And Reduced Motion

Mobile should use shorter travel distance and reduced delay. The existing `prefers-reduced-motion` block should continue to collapse animations and hover transforms.

## Implementation Scope

Modify:

- `frontend/src/views/HomeView.vue`
- `frontend/src/views/__tests__/HomeView.spec.ts`

No locale copy changes are needed.

## Testing

Automated checks:

- `pnpm --dir frontend run test -- HomeView`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run build`

Manual checks:

- Guest default home page in light mode.
- Guest default home page in dark mode.
- Mobile viewport around 360px wide.
- Desktop viewport around 1280px wide.
- Custom home content still bypasses default homepage motion.

## Non-Goals

- No homepage layout redesign.
- No new JavaScript motion library.
- No scroll-jacking or page-progress animation.
- No changes to authentication, API, admin, check-in, billing, or deployment logic.
