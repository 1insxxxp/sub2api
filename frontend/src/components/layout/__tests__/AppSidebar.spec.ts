import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar shared shell structure', () => {
  it('keeps the sidebar frame hooks used by the unified admin shell', () => {
    expect(componentSource).toContain('class="sidebar-nav scrollbar-hide sidebar-nav-shell"')
    expect(componentSource).toContain('class="sidebar-footer-shell')
    expect(styleSource).toContain('.sidebar-nav-shell')
    expect(styleSource).toContain('.sidebar-footer-shell')
  })
})

describe('AppSidebar collapse motion', () => {
  it('does not transition every link property while the active image studio item collapses', () => {
    const sidebarLinkBlockMatch = styleSource.match(/\.sidebar-link\s*\{[\s\S]*?\n {2}\}/)

    expect(sidebarLinkBlockMatch).not.toBeNull()
    expect(sidebarLinkBlockMatch?.[0]).not.toContain('transition-all')
    expect(sidebarLinkBlockMatch?.[0]).toContain('transition-property:')
  })
})

describe('AppSidebar image studio entry', () => {
  it('keeps the image studio entry visible without waiting on an async feature flag', () => {
    expect(componentSource).not.toContain('imageStudioAPI')
    expect(componentSource).not.toContain('refreshImageStudioFlag')
    expect(componentSource).not.toContain('flagImageStudio')
    expect(componentSource).toMatch(
      /\{ path: '\/images', label: t\('nav\.imageStudio'\), icon: SparklesIcon, hideInSimpleMode: true \}/,
    )
  })
})

describe('AppSidebar admin entries', () => {
  it('keeps the check-in management route visible from the admin sidebar', () => {
    expect(componentSource).toMatch(
      /\{ path: '\/admin\/checkins', label: t\('nav\.checkins'\), icon: [A-Za-z]+Icon, hideInSimpleMode: true \}/,
    )
  })
})
