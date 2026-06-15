# Modern Tech Theme Design

## Goal

Refresh the product theme from teal/cyan to a calmer blue-violet technology palette while preserving the clean homepage style and making both light and dark modes feel intentional.

## Direction

Use electric blue as the primary product color, violet as the premium highlight, and cyan as a restrained technical accent. This keeps the application suitable for a SaaS/API gateway while making the homepage feel more modern than the current teal-heavy palette.

## Palette

- Primary blue: `#3b82f6`, `#2563eb`, `#1d4ed8`
- Highlight violet: `#8b5cf6`, used mainly in gradients and glows
- Technical cyan: `#06b6d4`, used sparingly for status and motion details
- Light surfaces: white, slate-tinted gray, and soft blue backgrounds
- Dark surfaces: `#020617`, `#0f172a`, and blue-black panels

## Light Mode

Light mode should stay crisp and operational. Primary actions use blue gradients, focus rings use blue transparency, section eyebrows use the primary palette, and the hero background receives a subtle blue tint instead of the current mint wash.

## Dark Mode

Dark mode should feel designed rather than inverted. Deep slate backgrounds remain, but active UI states, focus rings, code panels, and homepage highlights should use blue/violet/cyan accents with enough contrast against `#020617`.

## Scope

- Update Tailwind `primary` tokens and theme gradients.
- Update hardcoded teal values in shared styles, onboarding controls, charts, and key usage visuals.
- Refresh homepage-specific emerald/teal accents where they define the primary brand mood.
- Keep success states green where they represent actual status rather than brand identity.
- Add tests that guard the new palette and light/dark homepage adaptation.
