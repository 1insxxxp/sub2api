# Affiliate Core Compact Visibility Design

## Goal

Make the 36px Core tier preview immediately distinguishable in a static screenshot while preserving the existing two-column tier grid and mobile layout.

## Design

- Keep every compact badge at 36px so rows remain aligned.
- Give the compact Core badge a persistent blue-white hexagonal perimeter and a brighter six-step ignition node.
- Add a low-frequency cyan energy border to the Core rule cell. The cell effect remains visible while idle and intensifies on desktop hover.
- Keep the full reactor, electric arc, convergence, and shockwave burst for the current Core badge and desktop hover.
- On touch input, retain only the low-frequency node and cell-border motion.
- Under reduced motion, show a static perimeter and cell border with no animation.

## Constraints

- No particles, canvas, dependencies, or layout size changes.
- No horizontal overflow or clipping outside the tier grid.
- Effects remain decorative and hidden from assistive technology.
- Other tier cells must not inherit Core-specific styling.

## Verification

- Component tests cover the Core rule-cell hook, idle cell animation, hover animation, touch fallback, and reduced-motion fallback.
- Browser verification checks desktop and 390px layouts, computed animations, 36px badge dimensions, and horizontal overflow.
- A static screenshot must show a visible Core perimeter and cell treatment without relying on animation timing.
