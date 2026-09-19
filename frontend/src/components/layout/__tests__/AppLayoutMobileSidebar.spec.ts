import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AppLayout from '../AppLayout.vue'

const appStore = reactive({
  mobileOpen: false,
  sidebarCollapsed: false,
  setMobileOpen(open: boolean) { this.mobileOpen = open }
})
const route = reactive({ path: '/keys' })

vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('@/stores', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { role: 'user' } }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ setReplayCallback: vi.fn() }) }))
vi.mock('@/composables/useOnboardingTour', () => ({ useOnboardingTour: () => ({ replayTour: vi.fn() }) }))
vi.mock('../AppSidebar.vue', () => ({ default: { template: '<aside />' } }))
vi.mock('../AppHeader.vue', () => ({ default: { template: '<header><button>Header action</button></header>' } }))

let wrapper: VueWrapper | undefined
let media: MediaQueryList
let listeners: Set<(event: MediaQueryListEvent) => void>

async function setMobile(matches: boolean) {
  Object.defineProperty(media, 'matches', { configurable: true, value: matches })
  listeners.forEach(listener => listener({ matches } as MediaQueryListEvent))
  await flushPromises()
}

async function mountLayout() {
  wrapper = mount(AppLayout, { slots: { default: '<button>Main action</button>' } })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  appStore.mobileOpen = false
  route.path = '/keys'
  listeners = new Set()
  media = {
    matches: true,
    media: '(max-width: 1023px)',
    addEventListener: (_event: string, listener: (event: MediaQueryListEvent) => void) => listeners.add(listener),
    removeEventListener: (_event: string, listener: (event: MediaQueryListEvent) => void) => listeners.delete(listener)
  } as unknown as MediaQueryList
  vi.stubGlobal('matchMedia', vi.fn(() => media))
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
  document.documentElement.classList.remove('mobile-sidebar-open')
  vi.unstubAllGlobals()
})

describe('AppLayout mobile sidebar isolation', () => {
  it('blocks header and main interaction and locks page scrolling until the drawer closes', async () => {
    const view = await mountLayout()
    appStore.mobileOpen = true
    await flushPromises()
    expect(view.get('.app-shell-main').attributes('inert')).toBeDefined()
    expect(document.documentElement.classList.contains('mobile-sidebar-open')).toBe(true)

    appStore.mobileOpen = false
    await flushPromises()
    expect(view.get('.app-shell-main').attributes('inert')).toBeUndefined()
    expect(document.documentElement.classList.contains('mobile-sidebar-open')).toBe(false)
  })

  it('releases the background when switching to desktop and does not reopen on returning to mobile', async () => {
    const view = await mountLayout()
    appStore.mobileOpen = true
    await flushPromises()
    await setMobile(false)
    expect(appStore.mobileOpen).toBe(false)
    expect(view.get('.app-shell-main').attributes('inert')).toBeUndefined()
    expect(document.documentElement.classList.contains('mobile-sidebar-open')).toBe(false)
    await setMobile(true)
    expect(view.get('.app-shell-main').attributes('inert')).toBeUndefined()
  })

  it('cleans up the drawer state and its lock when navigating away', async () => {
    const view = await mountLayout()
    appStore.mobileOpen = true
    await flushPromises()
    view.unmount()
    wrapper = undefined
    expect(appStore.mobileOpen).toBe(false)
    expect(document.documentElement.classList.contains('mobile-sidebar-open')).toBe(false)
  })

  it('does not remove another dialog scroll lock', async () => {
    document.body.classList.add('modal-open')
    try {
      await mountLayout()
      appStore.mobileOpen = true
      await flushPromises()
      appStore.mobileOpen = false
      await flushPromises()
      expect(document.body.classList.contains('modal-open')).toBe(true)
    } finally {
      document.body.classList.remove('modal-open')
    }
  })
})
