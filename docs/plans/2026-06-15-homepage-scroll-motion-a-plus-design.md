# Homepage Scroll Motion A+ Design

## Background

The first scroll-motion pass preserved the homepage style but was too subtle to notice during normal browsing. The page is already using the latest code locally, so the next change should increase perceptibility rather than fix hot reload.

## Goal

Make the default homepage scroll-loading animation visibly smoother and more noticeable while keeping the existing refined SaaS/developer-platform style.

## Direction

Use a CSS-only A+ motion pass. Keep the layout, content, colors, and dependencies unchanged.

## Motion Changes

- Increase section/card entry travel from subtle to clearly visible.
- Add a tiny scale-in and blur-to-clear effect during entry.
- Increase stagger delay enough that repeated cards feel intentionally sequenced.
- Make section headings reveal before cards with a stronger offset.
- Make code and CTA panels settle a little more slowly than simple cards.
- Preserve reduced-motion behavior and keep mobile motion softer than desktop.

## Guardrails

- No scroll-jacking.
- No new JavaScript state or animation library.
- No looping section reveals.
- No layout shift or horizontal overflow.
- No changes to custom homepage content mode.

## Success Criteria

- Users can notice cards and section titles entering as they scroll.
- The page still feels calm, technical, and trustworthy.
- Desktop and mobile render without horizontal overflow.
- Existing homepage tests still pass.
