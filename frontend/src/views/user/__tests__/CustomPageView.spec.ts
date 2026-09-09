import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CustomPageView from '../CustomPageView.vue'

const { appStore } = vi.hoisted(() => ({
  appStore: {
    publicSettingsLoaded: true,
    cachedPublicSettings: { custom_menu_items: [{ id: 'docs', url: 'https://example.com/docs' }] },
  },
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: 'docs' } }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }))
vi.mock('@/stores', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: false, user: { id: 7 }, token: 'test-token' }) }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [] }) }))
vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => `/api/v1${path}` }))

const wrappers: ReturnType<typeof mount>[] = []

function mountPage() {
  const wrapper = mount(CustomPageView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('custom page embedded content', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings.custom_menu_items = [{ id: 'docs', url: 'https://example.com/docs' }]
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.unstubAllGlobals()
  })

  it('renders embedded content without the removed open-in-new-window control', () => {
    const wrapper = mountPage()
    expect(wrapper.find('.custom-open-fab').exists()).toBe(false)
    expect(wrapper.get('iframe').attributes('src')).toContain('user_id=7')
    expect(wrapper.get('iframe').attributes('src')).toContain('token=test-token')
  })

  it('keeps Markdown pages separate from the embedded-page controls', async () => {
    appStore.cachedPublicSettings.custom_menu_items = [{ id: 'docs', url: 'md:guide' }]
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, text: async () => '# Guide' }))
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.find('.custom-open-fab').exists()).toBe(false)
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('.markdown-page-content h1').text()).toBe('Guide')
  })
})
