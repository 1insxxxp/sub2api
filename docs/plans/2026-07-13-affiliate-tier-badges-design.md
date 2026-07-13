# Affiliate Tier Badge Design

## Goal

Give the affiliate promotion page a distinctive progression identity without turning it into a marketing landing page or reducing information density.

## Visual Direction

Use four transparent-background, futuristic metal badge assets:

- Standard / 普通: graphite compass emblem
- Bronze / 青铜: bronze shield emblem with restrained oxidized-green accents
- Silver / 白银: cool silver starburst emblem
- Gold / 黄金: gold crown emblem

The badges should share one silhouette family and lighting direction so they read as a coherent level system. They must remain legible at both 88px and 36px. Avoid text inside the image assets so localization remains in HTML.

## Page Composition

The current-level summary places an 88px badge beside the current tier, effective rebate rate, qualified invitee count, and progress. At mobile widths the badge reduces to 68px and the content wraps without horizontal overflow.

The four-tier rule band remains a compact two-column grid. Each tier uses a 36-44px badge beside its localized name, rate, and requirement. The current automatic tier receives a stronger border and restrained glow; other tiers remain visually quieter.

No cards are nested inside the existing summary card. Badge treatments are unframed elements inside the current layout.

## Motion

The current badge uses a subtle breathing glow. Gold may receive a very light periodic highlight sweep. There are no continuously rotating rings or particle systems.

All motion is disabled under `prefers-reduced-motion: reduce`, leaving a static highlight.

## Assets

Generate transparent raster badge assets, preferably WebP with PNG fallback if required by the generation output. Store them with the frontend's existing static assets and reference them through the build pipeline. Do not encode level names in the images.

## Responsive Requirements

- 320px and 390px: no viewport overflow; badge 68px; tier rules remain two columns.
- Desktop: current badge 88px; summary content remains aligned with existing cards.
- Long translated labels and percentages must not resize or shift the badge layout.

## Accessibility

- Badge images use localized tier names as alt text where meaningful.
- Visual state is duplicated in text; color and images are not the only level indicators.
- Decorative glow layers are hidden from assistive technology.
- Motion respects reduced-motion preferences.

## Verification

- Component tests verify asset mapping, current-tier highlighting, localized labels, and reduced-motion classes/contracts.
- Typecheck and production build must pass.
- Real-browser checks cover 320px, 390px, and desktop dimensions, including `scrollWidth <= clientWidth` and screenshots.
