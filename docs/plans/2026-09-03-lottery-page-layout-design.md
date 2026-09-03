# Lottery Page Layout Design

## Approved direction

Use the approved A direction: a compact, airy lottery page that keeps the existing blue/cyan visual language while giving the prize details and draw history a clearer vertical rhythm.

## Visual structure

- Keep the hero and slot-machine draw panel as the primary focus.
- Turn the prize details area into a contained, lightly tinted panel so its heading, cards, and empty state read as one section.
- Use a three-column prize grid from the large-tablet/desktop breakpoint onward. Smaller screens fall back to two columns, then one column on phones.
- Reduce prize-card padding and remove the artificial description height that creates excess whitespace. Keep the card footer aligned with flex layout so values and inventory stay visually consistent.
- Separate the prize panel from the history panel with a deliberate section gap instead of allowing their borders to visually touch.

## Responsive behavior

- At `lg` and above, three equal prize cards fill the available content width.
- At `sm` through `lg`, two cards remain readable without horizontal scrolling.
- At phone widths, cards stack to one column and the draw action remains full width.
- Long prize names and descriptions continue to wrap without breaking the grid.

## Interaction and accessibility

- Preserve existing draw, result, history, and disabled-prize behavior; this is a presentation-only change.
- Keep existing semantic headings and button labels.
- Retain visible hover/focus affordances and dark-mode color variants.

## Scope

Modify `frontend/src/views/user/LotteryView.vue` and update its focused view test only if a stable layout hook is needed. No API or data-model changes are required.
