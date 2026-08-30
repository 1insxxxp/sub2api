# Mobile Header Flat Actions Design

## Goal

Reduce the visual weight of the mobile header by replacing the repeated outlined icon boxes with a calm, flat action toolbar.

## Design

- Keep menu, announcements, model plaza, and check-in as stable 36px icon targets.
- Remove their default borders, gradients, and raised shadows on mobile.
- Use the active theme color for icons and a subtle theme-tinted surface only on hover, press, or keyboard focus.
- Keep the subscription control as the only compact status pill because it communicates both an icon and a count.
- Keep the user avatar frameless and preserve the existing desktop header.
- Preserve one-line layout at 320px and retain visible keyboard focus feedback.

## Verification

- Update the focused `AppHeader` tests to assert the flat default state and subtle interactive state.
- Keep the existing 320px toolbar budget test green.
- Run the focused component suite, frontend typecheck, targeted lint, and verify both local services remain healthy.
