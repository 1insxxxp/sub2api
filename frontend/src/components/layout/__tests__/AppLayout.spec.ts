import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppLayout workspace sizing', () => {
  it('keeps module content fluid inside the available workspace', () => {
    expect(componentSource).toContain('class="app-shell-content"')
    expect(componentSource).toContain('min-width: 0;')
    expect(componentSource).toContain('max-width: none;')
    expect(componentSource).toContain('--app-content-padding-y: clamp(')
    expect(componentSource).toContain('--app-content-padding-x: clamp(')
    expect(componentSource).toContain('padding: var(--app-content-padding-y) var(--app-content-padding-x);')
    expect(componentSource).not.toContain('max-w-[1600px]')
    expect(componentSource).not.toContain('mx-auto w-full max-w')
  })

  it('does not animate every main-shell property during sidebar toggles', () => {
    const mainShellClassMatch = componentSource.match(/class="app-shell-main[^"]*"/)
    expect(mainShellClassMatch?.[0]).not.toContain('transition-all')
    expect(componentSource).toContain('transition: margin-left 240ms ease-out;')
    expect(componentSource).toContain("'app-shell-main-image-studio': route.path === '/images'")
    expect(componentSource).toContain('.app-shell-main-image-studio')
    expect(componentSource).toContain('transition: none;')
    expect(componentSource).toContain('.app-shell-main-image-studio .app-shell-content')
    expect(componentSource).toContain('.app-shell-image-studio .sidebar-label')
  })

  it('exposes the available image studio width as a named layout container', () => {
    const imageStudioContentRule =
      componentSource.match(/\.app-shell-main-image-studio \.app-shell-content\s*\{([^}]*)\}/)?.[1] ?? ''

    expect(imageStudioContentRule).toMatch(/container:\s*image-studio-workspace \/ inline-size/)
  })
})
