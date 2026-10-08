# Home Screen Standalone Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Open Passion API from a newly added phone Home Screen icon in a standalone window, keeping all same-origin application routes in scope.

**Architecture:** Add installation metadata to the existing HTML entry point and a same-origin static Web App Manifest. Reuse the existing scalable brand mark with a 512px PNG fallback and Apple touch icon. Vite copies these public assets into the embedded frontend; existing backend static serving and branding injection remain intact.

**Tech Stack:** HTML metadata, Web App Manifest JSON, Vite/Vue, existing Go embedded frontend.

---

## Approved design

- Manifest `id`, `start_url`, and `scope` are `/`; `display` is `standalone`. Root scope covers `/home`, `/login`, `/keys`, and `/admin/accounts` on the installation origin.
- Name and short name are `Passion API`. Existing SVG is declared as `sizes: any`, `purpose: any`; the existing 512x512 PNG provides a raster fallback.
- HTML links the manifest and PNG Apple touch icon, with both standard and Apple standalone capability metadata. The Apple status bar uses its default safe-area behavior; the viewport and page layout remain unchanged.
- Installation metadata does not introduce service-worker caching or alter application routing, authentication, charging, or permissions. External domains and intentionally opened external links remain browser-managed.
- Existing installed shortcuts may retain their installation mode. Device acceptance uses a newly added icon with “Open as Web App” enabled where that option is available.

## Task 1: Reproduce the missing installation contract

**Files:**
- Inspect: `frontend/index.html`
- Inspect: `frontend/public/logo-passion-api-mark.svg`
- Inspect: `frontend/public/logo-passion-api-mark.png`
- Inspect: `backend/internal/web/embed_on.go`

Run a one-off HTML/manifest asset probe before changing files. Expect failure because the current HTML has no manifest link or standalone capability declaration. Confirm the same declarations are absent from both deployed entrypoints. This configuration change uses artifact validation rather than permanent tests that duplicate constant metadata.

## Task 2: Add the installation metadata

**Files:**
- Modify: `frontend/index.html`
- Create: `frontend/public/manifest.json`

Add the same-origin manifest link, Apple touch icon, `mobile-web-app-capable=yes`, `apple-mobile-web-app-capable=yes`, `apple-mobile-web-app-title=Passion API`, and default Apple status-bar style. Declare the approved root-scoped standalone manifest and existing SVG/PNG resources with their truthful types and sizes. Keep existing favicon, title, viewport, and application entry point.

## Task 3: Verify built assets and embedded serving

- Run the one-off probe against source HTML and the production build. Parse JSON, validate root scope for user/admin/authentication routes, and verify each icon file/type/dimension.
- Run `pnpm run build` in `frontend`; confirm the manifest and both referenced icons are copied to `backend/internal/web/dist`.
- Run relevant branding/navigation tests and `pnpm run lint:check`.
- Run `go test -tags=embed ./internal/web` with bounded local build concurrency; confirm existing settings injection, static serving, SPA fallback, and caching checks pass.
- Serve built assets locally and verify manifest JSON MIME and icon response MIME/body, including requests initiated from `/admin/accounts`.
- Independently review the final diff; use a new iPhone Home Screen installation for final device acceptance after deployment. A local build does not prove that the phone has adopted the new installation mode.

## References

- https://support.apple.com/zh-cn/guide/iphone/iphea86e5236/ios
- https://webkit.org/blog/16993/news-from-wwdc25-web-technology-coming-this-fall-in-safari-26-beta/
- https://web.dev/learn/pwa/web-app-manifest
- https://web.dev/articles/add-manifest
