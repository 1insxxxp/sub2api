# Homepage tech stage redesign

## Direction

Rebuild the default landing page around a dark, high-contrast technology stage with blue/cyan accents. Keep the existing theme toggle, authentication links, docs/model plaza entries, endpoint normalization, compact mode, and custom `home_content` branch unchanged.

## Implementation

- Replace the centered single-column hero with a responsive two-column hero: localized value proposition and a decorative API gateway console.
- Retain the orbit visual as a stronger animated signal with layered grid, glow, scanline, and status pulse effects.
- Reframe capability content as a bento-style card row and a connected three-step workflow rail.
- Preserve reduced-motion behavior and keep all decorative motion in CSS transforms/opacity.
- Verify with the HomeView unit suites, frontend typecheck/build, and a source-level motion/accessibility check.
