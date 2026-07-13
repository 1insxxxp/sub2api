# Affiliate Tier Identity Redesign

## Goal

Bring the affiliate tier identity into the product's existing blue/cyan operational-dashboard language. Replace the current game-rank metal badges and make each tier's affiliate page feel progressively more accomplished without changing the location or behavior of core actions.

## Product Names And Compatibility

Keep the persisted and API tier values unchanged so existing users, settings, migrations, and rebate calculations are unaffected. Map those values to the new display names everywhere the tier is visible:

| Internal value | Chinese display name | English display name |
| --- | --- | --- |
| `standard` | 原点级 | Origin |
| `bronze` | 脉冲级 | Pulse |
| `silver` | 星环级 | Orbit |
| `gold` | 极核级 | Core |

The user affiliate page and administrator affiliate/settings views must use the same names. An unknown value must fall back to the Origin presentation without changing backend data.

## Visual Direction

Use one coherent family of transparent blue/cyan geometric badges derived from the Passion API logo language: segmented geometry, white structural lines, circular nodes, and restrained soft glow. Avoid realistic metal, shields, crowns, armor, and large bronze/silver/gold color fields.

The badge structure grows with the tier:

- Origin: one core node and a single segmented ring.
- Pulse: a second ring and visible energy connections.
- Orbit: an orbital structure with multiple linked nodes.
- Core: a complete multilayer core and network structure.

All four badges remain primarily blue/cyan. Tier distinction comes from structural complexity, brightness, and small white or cool-blue highlights. The assets contain no text and must stay legible at the current summary and compact rule-list sizes.

## Tier-Aware Page Presentation

The page keeps a stable module order: statistics, tier identity and rules, invite code and link, balance transfer, and invitee records. Users must not need to relearn the page after an upgrade.

The tier identity area becomes the main visual differentiator:

- Origin uses a sparse pale-blue grid and a quiet single-node treatment.
- Pulse adds a double halo, slow pulse line, and a subtle cyan edge on the emphasized statistic.
- Orbit adds orbital paths, connected nodes, and brighter blue/cyan highlights.
- Core uses the complete network pattern and restrained flowing light without introducing a gold theme.

Background patterns stay inside the identity area rather than decorating the entire page. Mobile layouts simplify the pattern density and preserve the existing compact dashboard character. Dark mode receives equivalent contrast rather than a separate visual concept.

## Tier-Aware Information

Add a current-stage objective inside the tier identity area using data already returned by the affiliate detail API:

- Origin emphasizes the invite code/link and the first qualified invitee.
- Pulse emphasizes qualified invitees and progress toward Orbit.
- Orbit emphasizes cumulative rebate, qualified-invitee ratio, and progress toward Core.
- Core replaces remaining-progress language with highest-tier status, cumulative promotion results, and the effective rebate rate.

The four existing statistic cards stay in the same order. A tier-specific featured statistic may receive stronger border and background emphasis, but no statistic or operation is removed. Custom administrator rebate rates continue to be labeled separately from the automatic tier.

No new backend privileges, endpoints, or analytics fields are part of this redesign. Derived values such as qualified-invitee ratio must handle an invitee count of zero safely.

## Frontend Structure

Create a centralized tier presentation configuration that owns display keys, badge assets, theme identifiers, stage-objective selection, and featured-statistic selection. Extract the tier identity presentation from `AffiliateView.vue` into a focused component so tier-specific rendering does not spread through the rest of the page.

Keep business calculations such as effective rate resolution and promotion qualification on the backend. The frontend only selects presentation from the returned automatic tier and existing affiliate statistics.

## Motion And Accessibility

Motion is limited to the badge and identity background:

- Origin: very light core breathing.
- Pulse: slow pulse expansion.
- Orbit: subtle orbital movement.
- Core: restrained light flow through the complete structure.

All effects stop under `prefers-reduced-motion: reduce`. Tier name, rate, and status remain textual, so color and motion are never the only indicators. Badge images use localized tier names as alternative text; decorative pattern layers are hidden from assistive technology.

## Responsive Requirements

- Support 320px, 390px, and desktop widths without horizontal overflow.
- Simplify decorative lines and node count on narrow screens.
- Keep invite links, long localized labels, percentages, and stage objectives wrapping inside their containers.
- Do not let badge animation, featured states, or changing objective text resize the fixed identity layout unexpectedly.
- Preserve the existing mobile invitee-card treatment and desktop table behavior.

## Verification

- Component tests cover the four display-name mappings, theme selection, stage objectives, featured statistics, Core highest-tier state, custom-rate label, and unknown-tier fallback.
- Existing affiliate tests continue to cover rule highlighting, progress, invite links, and mobile invitee rendering.
- Run frontend unit tests, type checking, linting, and production build.
- Verify light and dark modes in a real browser at 320px, 390px, and 1440px.
- Capture screenshots and assert `scrollWidth <= clientWidth` at each viewport.
- Confirm reduced-motion disables badge and background animation.

## Non-Goals

- No changes to thresholds, rebate rates, qualification rules, or permanent-tier behavior.
- No new tier-only permissions or tools.
- No database or API schema migration.
- No full-page marketing layout or game-style rank presentation.
