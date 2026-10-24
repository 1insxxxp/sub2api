import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AppSidebar from '../AppSidebar.vue'

const authState = reactive({
  user: { id: 72, role: 'user', balance_redeem_code_enabled: false as boolean | undefined },
  isAdmin: false,
  isSubAdmin: false,
  canAccessAdminWorkbench: false,
  isSimpleMode: false,
})
const appState = reactive({
  sidebarCollapsed: false, mobileOpen: false, sidebarScrollTop: 0,
  backendModeEnabled: false, publicSettingsLoaded: true,
  siteName: 'Passion', siteLogo: '', siteVersion: '0.2.18',
  cachedPublicSettings: { custom_menu_items: [], user_sidebar_order: [] },
  setMobileOpen: vi.fn(),
})
const adminState = reactive({ customMenuItems: [], opsMonitoringEnabled: true, paymentEnabled: true })

vi.mock('@/stores', () => ({
  useAuthStore: () => authState,
  useAppStore: () => appState,
  useAdminSettingsStore: () => adminState,
  useOnboardingStore: () => ({ isCurrentStep: () => false }),
}))
vi.mock('@/utils/featureFlags', async () => ({
  ...await vi.importActual<typeof import('@/utils/featureFlags')>('@/utils/featureFlags'),
  makeSidebarFlag: () => () => true,
}))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: ref(false), refreshBatchImageAccess: vi.fn() }),
}))
vi.mock('@vueuse/core', () => ({ useMediaQuery: () => ref(false) }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/dashboard' }),
  useRouter: () => ({ push: vi.fn() }),
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

let wrapper: VueWrapper | undefined
const mountSidebar = () => {
  wrapper = mount(AppSidebar, {
    global: { stubs: {
      RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
      VersionBadge: true, Icon: true,
    } },
  })
  return wrapper
}

beforeEach(() => {
  authState.user.balance_redeem_code_enabled = false
  authState.isSimpleMode = false
  appState.backendModeEnabled = false
  localStorage.clear()
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined })

describe('ordinary user balance-transfer navigation', () => {
  it('only advertises the independent tool after explicit authorization', async () => {
    const view = mountSidebar()
    expect(view.find('a[href="/balance-transfer"]').exists()).toBe(false)
    authState.user.balance_redeem_code_enabled = true
    await flushPromises()
    expect(view.find('a[href="/balance-transfer"]').exists()).toBe(true)
    expect(view.find('a[href="/admin/workbench"]').exists()).toBe(false)
    expect(view.get('.sidebar-logo').attributes('href')).toBe('/dashboard')
  })

  it('removes the creation entry when permission is revoked', async () => {
    authState.user.balance_redeem_code_enabled = true
    const view = mountSidebar()
    expect(view.find('a[href="/balance-transfer"]').exists()).toBe(true)
    authState.user.balance_redeem_code_enabled = false
    await flushPromises()
    expect(view.find('a[href="/balance-transfer"]').exists()).toBe(false)
  })

  it('does not advertise creation when an older user response omits the permission', () => {
    authState.user.balance_redeem_code_enabled = undefined
    expect(mountSidebar().find('a[href="/balance-transfer"]').exists()).toBe(false)
  })

  it('respects simple-mode restrictions', () => {
    authState.user.balance_redeem_code_enabled = true
    authState.isSimpleMode = true
    expect(mountSidebar().find('a[href="/balance-transfer"]').exists()).toBe(false)
  })

  it('does not grant ordinary-user navigation in backend mode', () => {
    authState.user.balance_redeem_code_enabled = true
    appState.backendModeEnabled = true
    expect(mountSidebar().find('a[href="/balance-transfer"]').exists()).toBe(false)
  })
})
