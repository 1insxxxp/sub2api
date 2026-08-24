# Mobile Header Toolbar Design

## Goal

Rebuild the authenticated mobile header as a compact, orderly toolbar that keeps the model plaza visible, removes the language switcher on small screens, and matches the existing blue/cyan product theme.

## Design

The desktop header keeps its current full labels, language switcher, balance pill, and profile identity. Below the `sm` breakpoint, the header becomes a single non-wrapping action row:

- The menu button remains on the left.
- Announcement, model plaza, subscription status, check-in, and profile remain accessible on the right.
- The locale switcher is hidden on mobile and remains available from `sm` upward.
- Menu, model plaza, check-in, and profile use a common 36px control size, 8px radius, blue/cyan border, pale theme background, and clear focus ring.
- The check-in label is hidden on mobile so its control cannot push the model plaza out of view. Its accessible label and tooltip remain intact.
- The action row never wraps; fixed-size actions do not shrink, while desktop-only content is removed from the mobile width budget.

## Responsive Behavior

At 320px and wider, the toolbar reserves only the minimum width needed by the menu and compact action controls. At `sm` and above, existing text labels and the language switcher return. Desktop page titles and balance presentation remain unchanged.

## Accessibility

Every icon-only control keeps an `aria-label` and title where appropriate. Focus rings use the existing blue theme. Hiding visible labels on mobile does not remove their accessible names.

## Verification

Component tests will assert that the locale switcher is mobile-hidden, that model plaza remains visible, that check-in becomes icon-only until `sm`, and that mobile controls share the compact theme class. The frontend build and focused component tests must pass.
