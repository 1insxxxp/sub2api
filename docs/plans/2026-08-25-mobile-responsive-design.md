# Mobile Responsive Design

## Goal

Improve mobile usability across the shared application shell and the most frequently used pages without changing the existing desktop information architecture.

## Scope

The first pass covers:

- Shared app header, sidebar drawer, page containers, toast notifications, and modal positioning.
- High-frequency pages: admin workbench, accounts, API keys, subscriptions, redeem, usage history, and channel status.
- A second pass for groups, channels, system settings, and model plaza after the shared fixes are verified.

The current uncommitted commission calendar changes remain out of scope and must not be included in this work.

## Design

### Shared shell

At mobile widths, the sidebar behaves as an overlay drawer and the content area uses a stable viewport-safe width with no horizontal page overflow. The top navigation keeps only high-value actions visible, moves secondary actions into a compact menu when necessary, and preserves the model plaza entry where it is available.

### Dialogs and overlays

Dialogs are centered on mobile rather than bottom sheets unless the interaction is explicitly a short action sheet. Dialog bodies have a bounded height and internal scrolling, while the title and action footer remain usable. Toasts are constrained to the viewport and wrap long messages without pushing layout width.

### Tables and dense data

Desktop tables remain unchanged. On mobile, dense tables use existing card or responsive-row patterns where available; otherwise they receive a contained horizontal scroll area instead of widening the page. Primary actions stay visible near the first content block, and long names use truncation or controlled wrapping.

### Cards, grids, and calendars

Fixed-width cards and multi-column grids collapse to one column at mobile widths. Calendar cells use stable grid tracks and compact numeric summaries so amounts do not overflow or create unpredictable row heights.

## Verification

Use the local application at 375px, 390px, 768px, and a desktop viewport. Verify:

- No document-level horizontal overflow.
- Dialog content, controls, and footer actions remain reachable.
- Navigation actions do not overlap or disappear unexpectedly.
- Tables and dense records can be read and operated without clipped controls.
- Existing desktop layout and current user changes remain intact.

