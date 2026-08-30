# Admin Workbench Mobile Layout Design

## Scope

Make all three `/admin/workbench` tabs usable from 320px through 430px wide screens without horizontal page scrolling:

- balance transfer and generated redeem codes
- commission settings, calendar, and day details
- affiliate leaderboard

Desktop behavior and API contracts remain unchanged.

## Layout Direction

Use one mobile-first responsive system instead of separate mobile components. Existing data and actions remain in the same Vue components, while the templates gain stable mobile dimensions and breakpoint-specific composition.

### Page Shell and Tabs

- Reduce page padding and vertical spacing below the `sm` breakpoint.
- Present the title and balance as a compact stacked header.
- Render the three tabs as an equal-width grid on mobile. Each tab has a stable height, icon, and wrapping label so no horizontal tab scrolling is required.
- Preserve the current horizontal desktop tab treatment at `sm` and above.

### Balance Transfer

- Keep inputs in one column on narrow screens and use two columns from `sm` upward.
- Stack the generated-result heading and copy action on very narrow screens.
- Keep the generated-result panel height bounded and scroll codes inside it.
- Arrange list controls as a full-width mobile grid and keep each code row vertically structured: selection and identity first, metadata second, actions last.
- Long codes and notes may wrap; monetary values and status labels remain atomic.

### Commission Calendar

- Keep month totals as two stable metric cells when space permits and stack only below 360px.
- Use a compact seven-column month grid on mobile. Cells show the day and a data marker; full values remain in the month summary and selected-day dialog.
- Open day details in a centered, viewport-bounded dialog on mobile. Group amounts use two columns when possible and one column at very narrow widths.
- Long request IDs, emails, API key names, and model names wrap inside their cards without expanding the viewport.

### Affiliate Leaderboard

- Keep the desktop table at `md` and above.
- Use mobile cards below `md`, with rank, avatar, and user identity in one row.
- Use a two-column metric grid, allowing the cumulative amount to span the full row so labels do not collide.
- Email and username wrap within the card; numeric values do not wrap.

## Accessibility and Interaction

- Preserve tab roles, labels, and selected states.
- Keep touch targets at least 40px high.
- Preserve keyboard focus styles and dialog escape/backdrop close behavior.
- Use semantic buttons and labels; responsive changes must not hide required actions or data.

## Verification

- Add source/component tests that lock the mobile grid, bounded result panel, compact calendar, centered dialog, and leaderboard metric layout.
- Run the focused Vitest suite, `pnpm typecheck`, and `pnpm build`.
- Inspect screenshots at 375x812 and 1440x900, checking for horizontal overflow, clipped actions, wrapped labels, and dialog containment.

