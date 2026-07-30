# Mobile Group Selector Design

## Problem

On narrow screens, group names and rate badges compete for the same horizontal row. Long Chinese names or unbroken text are truncated, while hand-built group dropdowns can extend beyond the viewport. This makes similarly named production groups difficult to distinguish.

## Scope

- Make group names fully readable on mobile without truncation.
- Keep rate and peak-rate badges visible without squeezing the name.
- Keep dropdown surfaces within the mobile viewport.
- Apply the behavior through the shared group option component so all existing consumers benefit.
- Preserve the current compact desktop layout.

## Design

`GroupOptionItem` remains a horizontal name/description and rate layout on desktop. Below the small breakpoint, it switches to a stacked layout. The group badge occupies the full available width, its name wraps anywhere, and the rate area moves beneath the name. Descriptions retain the existing three-line clamp so a large group list remains scannable.

The group badge gains an opt-in wrapping mode rather than changing every compact badge in tables. `GroupOptionItem` enables that mode. Selected badges in the mobile API-key card also enable wrapping so the current selection does not collide with the change-group affordance.

The custom API-key group dropdown uses a responsive width of the viewport minus 16 pixels, capped at its existing desktop width. Its left edge is clamped to an 8-pixel viewport gutter. The admin user API-key dropdown receives the same positioning constraints.

## Accessibility And Interaction

- Existing buttons, keyboard behavior, selection state, and checkmarks remain unchanged.
- Wrapping changes only presentation; names remain plain visible text.
- The list retains vertical scrolling and does not introduce horizontal scrolling.

## Testing

- Component tests verify the wrapping mode and responsive classes for long group names.
- View tests verify custom dropdown width constraints and selected badge wrapping.
- Run focused Vitest tests, frontend type checking, and production build.
- Verify the API-key group selector at a mobile viewport in the local browser, including horizontal overflow and long-name visibility.

