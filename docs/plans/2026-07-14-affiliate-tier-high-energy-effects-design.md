# Affiliate Tier High-Energy Effects

## Goal

Raise the affiliate badge motion from restrained ambient light to an immediately visible high-energy state while preserving the distinct tier silhouettes, blue/cyan product language, mobile stability, and reduced-motion behavior.

## Direction

Use layered CSS energy fields rather than particles, animated image assets, canvas, or a runtime motion dependency. Each badge combines a stronger near glow, a wider outer aura, and its existing tier-specific beam or track. Peak light should be close to twice the restrained version, with primary cycles shortened to roughly 1.8-3.6 seconds.

Avoid rapid white flashes. No effect may flash three or more times per second. Text, rates, and progress remain visually dominant enough to read without interference.

## Tier Effects

- Origin: a strong core breath, recurring expanding energy ring, and bright arc sweep from the open edge.
- Pulse: a high-intensity center burst with twin horizontal wing expansion and a short cyan afterglow.
- Orbit: two tilted light tracks moving in opposite directions, with a bright orbital point and visible trail.
- Core: sequential six-direction illumination, central convergence, and an outward hexagonal shockwave.

## Activation

The current large badge continuously plays the full high-energy treatment. The current compact badge plays a clipped, simplified treatment. Other compact badges remain static on touch input and activate on desktop hover only.

Large effects may extend beyond the image while remaining inside the affiliate identity card. Compact effects remain clipped to the fixed 36px badge footprint.

## Accessibility And Performance

- Continue to disable repeated motion under `prefers-reduced-motion: reduce`.
- Reduced-motion mode retains a brighter static aura without beams, rotation, expansion, or convergence.
- Animate opacity, transform, and filter only; do not animate layout properties.
- Decorative layers remain pointer-inert and hidden from assistive technology.
- Do not add keyboard tab stops solely to trigger decorative effects.

## Verification

- Tests lock the high-energy duration ranges, stronger opacity peaks, outer aura layer, touch-only suppression, compact clipping, and reduced-motion fallback.
- Run affiliate component regressions, type checking, linting, and production build.
- Verify desktop and 390px mobile layouts in a real browser.
- Sample computed opacity or transform at multiple points to prove the animation visibly changes.
- Confirm all badge assets load, compact effects do not overlap text, and `scrollWidth <= clientWidth`.

## Non-Goals

- No changes to affiliate tier names, rules, thresholds, or rebate calculations.
- No particles, sound, canvas, animated bitmap assets, or whole-page effects.
- No persistent animation for non-current compact badges on touch devices.
