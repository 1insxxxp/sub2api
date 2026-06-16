# Theme Color Unification Design

## Goal

Unify the authenticated product interface around one clear visual language: primary blue, slate neutrals, and small cyan accents. Keep semantic colors only where they communicate status or risk.

## Visual Rules

- Primary and interactive brand surfaces use blue.
- Structural text, borders, backgrounds, cards, and navigation use slate.
- Cyan appears as a small technical accent for emphasis, not as a competing theme.
- Green stays for success, active, healthy, and normal states.
- Amber stays for warning, reward, check-in, and attention states.
- Red and rose stay for error, danger, disabled, and destructive states.
- Purple, violet, indigo, teal, and decorative emerald should be removed from high-visibility UI unless the color carries a specific semantic meaning.

## Scope

The first pass focuses on visible authenticated surfaces:

- User dashboard stat cards and quick actions.
- Admin dashboard stat cards and chart palettes.
- Key usage page summary cards.
- Shared badge and button utilities where decorative colors leak into many pages.
- Small high-visibility accents in admin pages that are clearly decorative.

Lower-risk semantic colors remain unchanged. Operational screens can keep green, amber, and red for health thresholds, warnings, SLA, quota, and errors.

## Interaction And Responsiveness

This is a visual-only change. Existing routing, API calls, auth, check-in behavior, tables, filters, and local development services remain untouched. The current light and dark modes must continue to work, and mobile layouts must not gain horizontal overflow.

## Success Criteria

- The app reads as one product family across the homepage, app shell, dashboards, and common content pages.
- Dashboard cards no longer look like unrelated color blocks.
- Charts use a blue/slate/cyan-led palette, with semantic amber/red only for warning or error series.
- Dark mode remains calm and legible.
- Current local hot reload reflects the changes without restarting services.
