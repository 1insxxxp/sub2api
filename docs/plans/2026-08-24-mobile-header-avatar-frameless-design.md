# Mobile Header Frameless Avatar Design

## Goal

Remove the decorative frame around the mobile header avatar while preserving the avatar menu interaction, accessible focus feedback, and existing toolbar layout.

## Design

- Add a dedicated class to the user-menu trigger and avatar container.
- At widths up to 639px, keep the trigger at the shared 36px mobile control size but make its border, background, and shadow transparent.
- Remove the avatar container ring and shadow on mobile so the uploaded or default avatar is shown directly.
- Preserve the existing desktop appearance and dropdown behavior.
- Retain a visible `focus-visible` outline around the trigger for keyboard accessibility.

## Verification

- Add a focused `AppHeader` test that asserts the dedicated mobile avatar rules remove the decorative surfaces without changing the shared control size.
- Run the focused component test, frontend typecheck, and targeted lint.
- Inspect the local header at a 320px mobile viewport and confirm the toolbar remains on one line.
