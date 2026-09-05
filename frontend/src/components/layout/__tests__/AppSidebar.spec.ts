import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')
const zhCommonPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../i18n/locales/zh/common.ts')
const zhCommonSource = readFileSync(zhCommonPath, 'utf8')
const enCommonPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../i18n/locales/en/common.ts')
const enCommonSource = readFileSync(enCommonPath, 'utf8')

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

describe('AppSidebar collapsible groups', () => {
  it('lets the user collapse a group even while a child route is active', () => {
    // The expand state must come from the user's override first, falling back
    // to the active-route heuristic only when the user has not clicked yet.
    expect(componentSource).toContain('const groupExpandOverrides = ref<Map<string, boolean>>(new Map())')
    expect(componentSource).not.toContain('expandedGroups.value.has(item.path) || isGroupActive(item)')
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
  it('uses the shared image studio feature flag', () => {
    expect(componentSource).not.toContain('imageStudioAPI')
    expect(componentSource).not.toContain('refreshImageStudioFlag')
    expect(componentSource).toContain('const flagImageStudio = makeSidebarFlag(FeatureFlags.imageStudio)')
    expect(componentSource).toMatch(
      /\{ path: '\/images', label: t\('nav\.imageStudio'\), icon: SparklesIcon, hideInSimpleMode: true, featureFlag: flagImageStudio \}/,
    )
  })
})

describe('AppSidebar admin entries', () => {
  it('exposes the admin workbench entry from the admin sidebar', () => {
    expect(componentSource).toMatch(
      /\{ path: '\/admin\/workbench', label: t\('nav\.adminWorkbench'\), icon: [A-Za-z]+Icon \}/,
    )
  })

  it('keeps the check-in management route visible from the admin sidebar', () => {
    expect(componentSource).toMatch(
      /\{ path: '\/admin\/checkins', label: t\('nav\.checkins'\), icon: [A-Za-z]+Icon, hideInSimpleMode: true \}/,
    )
  })

  it('does not expose the legacy manager page beside the workbench', () => {
    const buildSelfNav = componentSource.match(/function buildSelfNavItems[\s\S]*?return items\n}/)?.[0] ?? ''
    const adminNav = componentSource.match(/const adminNavItems = computed[\s\S]*?return finalizeNav\(baseItems\)\n}/)?.[0] ?? ''

    expect(buildSelfNav).not.toContain("path: '/manager'")
    expect(adminNav).not.toContain("path: '/manager'")
  })

  it('distinguishes management destinations from the matching personal pages', () => {
    expect(componentSource).toContain("path: '/admin/usage', label: t('nav.adminUsage')")
    expect(componentSource).toContain("path: '/admin/lottery', label: t('nav.lotteryManagement')")
    expect(componentSource).toContain("path: '/admin/affiliates',\n      label: t('nav.affiliateManagement')")
    expect(componentSource).toContain("path: '/admin/orders', label: t('nav.orderList')")

    expect(zhCommonSource).toContain("adminUsage: '全站用量'")
    expect(zhCommonSource).toContain("lotteryManagement: '抽奖管理'")
    expect(zhCommonSource).toContain("affiliateManagement: '邀请管理'")
    expect(zhCommonSource).toContain("orderList: '订单列表'")
    expect(enCommonSource).toContain("adminUsage: 'Global Usage'")
    expect(enCommonSource).toContain("lotteryManagement: 'Lottery Management'")
    expect(enCommonSource).toContain("affiliateManagement: 'Affiliate Management'")
    expect(enCommonSource).toContain("orderList: 'Order List'")
  })
})

describe('AppSidebar sub-admin entries', () => {
  it('uses admin workbench access instead of full admin access for the regular view', () => {
    expect(componentSource).toContain("v-else-if=\"!appStore.backendModeEnabled || authStore.canAccessAdminWorkbench\"")
    expect(componentSource).toContain('const canAccessAdminWorkbench = computed(() => authStore.canAccessAdminWorkbench)')
  })

  it('keeps the workbench entry out of the full-admin personal menu', () => {
    const buildSelfNav = componentSource.match(/function buildSelfNavItems[\s\S]*?return items\n}/)?.[0] ?? ''

    expect(buildSelfNav).toContain('authStore.isSubAdmin')
    expect(buildSelfNav).toContain('appStore.backendModeEnabled && authStore.isSubAdmin')
    expect(buildSelfNav).toContain('if (authStore.isSubAdmin)')
    expect(buildSelfNav).not.toContain('appStore.backendModeEnabled && canAccessAdminWorkbench.value')
    expect(buildSelfNav).not.toContain('if (canAccessAdminWorkbench.value)')
  })
})

describe('AppSidebar custom menu external links', () => {
  it('renders selected custom menus as safe new-tab links', () => {
    expect(componentSource).toContain('item.externalUrl')
    expect(componentSource).toContain('target="_blank"')
    expect(componentSource).toContain('rel="noopener noreferrer"')
    expect(componentSource).toContain('resolveCustomMenuNavigation')
  })
})
