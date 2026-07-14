# Affiliate Tier Light Effects

## Goal

Add restrained blue/cyan light animation to the affiliate tier badges while preserving the operational dashboard style, the distinct badge silhouettes, and mobile performance.

## Motion Model

Use CSS-only decorative layers around the existing transparent WebP assets. Do not replace the assets with animated images, add a runtime animation library, or use canvas.

The current large badge plays a slow, continuous tier-specific effect. In the compact tier rule grid, only the current tier plays a persistent low-intensity effect; the other tiers animate on hover or keyboard focus. Mobile devices therefore keep a single continuously animated compact badge.

## Tier Effects

- Origin: a quiet core breath and a short glint around the open arc.
- Pulse: a horizontal energy expansion from the center into both wings.
- Orbit: a soft point of light moving around the tilted orbital silhouette.
- Core: sequential illumination around the hexagonal matrix followed by a restrained central convergence.

Each effect reinforces the tier's existing silhouette. Effects use the established blue/cyan palette and remain subordinate to tier names, rebate rates, and progress information.

## Component Structure

Wrap each badge image in a decorative effect container carrying its normalized tier theme. The wrapper owns glow, beam, and orbit layers; the image remains the visible semantic asset and keeps its fixed dimensions.

The large badge effect remains inside the existing fixed badge area. Compact rule effects remain inside the existing 36px badge footprint so they cannot resize rule rows or cause horizontal overflow. Decorative layers are `aria-hidden` and ignore pointer events.

## Accessibility And Performance

- Preserve text labels so animation is never the only tier indicator.
- Stop transforms and repeated animation under `prefers-reduced-motion: reduce` while retaining a static, low-opacity glow.
- Keep animation CSS-only and limit animated properties to `opacity`, `transform`, and `filter` where practical.
- Support keyboard focus with `focus-within` wherever hover starts an effect.
- Avoid rapid flashes and keep every cycle slower than two seconds.

## Responsive Behavior

The large badge runs continuously at desktop and mobile sizes. Compact non-current badges require hover or focus and therefore remain static on touch-only use. No effect may alter the established 72px/92px large badge or 36px compact badge dimensions.

## Verification

- Component tests verify effect wrappers expose the normalized tier theme and distinguish current compact badges.
- Existing affiliate component tests continue to pass.
- Type checking, linting, and production build pass.
- Browser checks cover desktop and 390px mobile layouts, image loading, overflow, and overlap.
- Browser inspection confirms the reduced-motion media query disables repeated animations.

## Non-Goals

- No changes to tier names, thresholds, rebate calculations, or API data.
- No sound, particles outside the badge footprint, or full-page animation.
- No animated image or canvas asset pipeline.
