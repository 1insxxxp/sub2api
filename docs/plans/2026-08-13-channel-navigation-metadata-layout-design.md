# Channel Navigation Metadata Layout Design

## Problem

The desktop channel rail currently places platform badges and group/model counts in one wrapping flex row. At the fixed 240px rail width, short combinations stay on one line while longer combinations wrap, so neighboring navigation items use inconsistent metadata layouts.

## Decision

Keep the existing 240px rail and visual language, but give every navigation item a fixed three-level information hierarchy:

1. Channel name.
2. Platform badges.
3. Group and model counts.

The badge and count rows remain independently wrap-safe, but they never share a line. This makes count placement predictable without truncating channel names or platform labels.

## Accessibility and responsive behavior

The existing listbox, option roles, keyboard navigation, focus styles, and mobile channel picker remain unchanged. The change applies only to the desktop rail's internal metadata layout.

## Verification

Add a component regression test that asserts platform badges and counts render in separate block rows and that the old shared `justify-between` wrapping row is absent. Run the focused catalog tests, frontend lint, typecheck, and production build.
