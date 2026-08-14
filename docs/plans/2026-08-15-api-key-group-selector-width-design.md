# API Key Group Selector Width Design

## Goal

Make the API key group selector easier to scan on desktop without changing its content, behavior, or mobile presentation.

## Design

- Increase the desktop floating selector width from `380px` to `480px`.
- Use the same `480px` value in the viewport-clamping calculation so the fixed popup cannot overflow the right edge.
- Keep the existing breakpoint: viewports below `768px` continue to use the full-width bottom sheet with eight-pixel outer spacing.
- Preserve the current search field, option rendering, maximum list height, keyboard behavior, colors, and dark mode.

## Verification

- Add a source-level regression assertion covering the desktop `480px` class and the matching `480px` positioning constant.
- Run the KeysView layout tests, ESLint for the changed Vue/test files, and the frontend type checker.

