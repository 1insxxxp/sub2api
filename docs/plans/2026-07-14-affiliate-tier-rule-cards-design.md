# Affiliate Tier Rule Cards Design

## Goal

Replace the shared-border tier rule grid with four complete tier cards. Every
tier must read as a first-class level while the current, unlocked, and locked
states remain immediately distinguishable.

## Scope

- Keep the existing four tier definitions, labels, rates, thresholds, badges,
  and automatic-level calculation.
- Change only the tier rules area inside `AffiliateTierIdentity.vue`.
- Do not change affiliate accounting, qualification, or API contracts.

## Card Structure

Render the tiers as a two-column grid on larger screens and one column on
mobile. Each tier is an independent card with its own complete border and
stable minimum height. A card contains:

- tier badge and tier-specific effect;
- localized tier name;
- rebate percentage;
- qualified-invitee threshold;
- a status label: unlocked, current, or locked;
- remaining qualified invitees for locked levels when the value is available.

## State Rules

Tier order follows the order returned in `detail.tiers`.

- Tiers before the automatic level are `unlocked`.
- The automatic level is `current`.
- Tiers after the automatic level are `locked`.
- If the runtime level is unknown, the existing normalization behavior treats
  Origin as current.

The current card receives the strongest border and ambient effect. Unlocked
cards retain normal contrast and a subdued completion mark. Locked cards keep
a complete visible frame but reduce badge intensity and text contrast.

## Visual Language

All cards use the project's restrained white, gray, blue, and cyan surfaces.
Tier identity comes from frame details rather than unrelated color families:

- Origin: circular corner trace;
- Pulse: short waveform accent;
- Orbit: orbital corner arcs;
- Core: hexagonal/reactor trace with the strongest cyan glow.

Effects remain contained within each card. The Core tier may be more energetic,
but it must not overlap adjacent cards or make other tiers look unfinished.

## Responsive And Accessibility

- Use one column below the `sm` breakpoint and two columns from `sm` upward.
- Do not truncate the status, rate, or threshold on narrow screens.
- Expose the visual status through `data-tier-state` and localized visible text.
- Decorative frame elements are hidden from assistive technology.
- Disable frame animation under `prefers-reduced-motion: reduce`.

## Verification

- Component tests cover four complete cards and all three states.
- Tests verify the current-level transition for each automatic level.
- Existing badge-effect, localization, numeric-sanitization, and reduced-motion
  tests continue to pass.
- Run frontend typecheck, lint, focused tests, and production build.
- Verify desktop and mobile rendering in the local browser.
