# Affiliate Core Reactor

## Goal

Make Core unmistakably more powerful than Origin, Pulse, and Orbit. Preserve the blue/cyan operational product style while giving the highest affiliate tier an exclusive reactor-like structure and motion hierarchy.

## Visual Concept

Core becomes a compact six-direction energy reactor. Three hexagonal structures rotate at different rates, six perimeter nodes ignite in sequence, energy converges into the center, and a restrained blue-white arc precedes an outward hexagonal shockwave.

This is not a brightness-only change. Structural complexity, counter-rotation, node sequencing, and a clear charge-release rhythm establish Core as the final tier.

## Large Core Badge

When the current automatic tier is Core, the large badge receives the complete reactor treatment:

- Three nested hexagonal energy structures with opposing rotation.
- A six-node ignition sequence that visually feeds the center.
- A central charge peak approximately every three seconds.
- A brief blue-white electric arc during peak charge.
- An outward hexagonal shockwave after convergence.

The cycle avoids rapid flashing and keeps tier text and rebate rate readable. Effects may extend beyond the image but remain inside the identity card and behind adjacent text.

## Compact Core Preview

The compact Core badge in the rule grid receives an exclusive low-frequency idle state even when Core is not the current tier. Its six-node perimeter performs a slow brightness chase so the highest tier is recognizable at rest.

Desktop hover triggers a stronger convergence and release. Touch input keeps only the simplified node cycle and does not run the large shockwave. Compact layers remain clipped to 36px with an iOS-compatible overflow fallback.

## Accessibility And Performance

- All new layers are decorative, pointer-inert, and `aria-hidden`.
- `prefers-reduced-motion: reduce` stops rotation, node sequencing, arcs, convergence, and shockwaves.
- Reduced-motion mode retains three static hexagonal structures with moderate brightness.
- Animate only opacity, transform, and filter.
- Keep every cycle slower than 1.5 seconds and avoid repeated high-contrast flashes.
- Non-Core tiers do not render visible reactor effects.

## Verification

- Component tests verify the reactor and arc layers, Core-only CSS selectors, compact idle contract, touch simplification, and reduced-motion coverage.
- Existing affiliate tier tests remain green.
- Run type checking, linting, and production build.
- Verify the compact Core preview in desktop and 390px browser layouts, including clipping, asset loading, and horizontal overflow.
- Inspect computed animation names for Core layers and confirm no console errors.

## Non-Goals

- No changes to affiliate rules, rates, thresholds, labels, or backend data.
- No particle system, canvas, sound, bitmap regeneration, or new animation dependency.
- No additional full-page decoration outside the tier identity card.
