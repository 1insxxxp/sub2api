import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HomeView from '@/views/HomeView.vue'

const appStoreState = reactive({
  cachedPublicSettings: null as null | {
    site_name?: string
    site_logo?: string
    site_subtitle?: string
    doc_url?: string
    home_content?: string
  },
  siteName: 'PassionAPI',
  siteLogo: '',
  docUrl: 'https://docs.example.com',
  publicSettingsLoaded: true,
  fetchPublicSettings: vi.fn(),
})

const authStoreState = reactive({
  isAuthenticated: false,
  isAdmin: false,
  user: null as null | { email: string },
  checkAuth: vi.fn(),
})

vi.mock('@/stores', () => ({
  useAppStore: () => appStoreState,
  useAuthStore: () => authStoreState,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'home.hero.title': 'A unified gateway for reliable multi-model API access',
    'home.hero.subtitle':
      'Route OpenAI-compatible requests across multiple providers with wallet billing, monitoring, and risk controls.',
    'home.hero.primaryCta': 'Start using',
    'home.hero.secondaryCta': 'Read docs',
    'home.hero.statusBadge': 'Gateway online',
    'home.hero.panelTitle': 'Live routing console',
    'home.trust.multiModel.title': 'Multi-model access',
    'home.sections.capabilitiesTitle': 'Everything needed to operate an API gateway',
    'home.integration.title': 'OpenAI-compatible by design',
    'home.workflow.title': 'Launch in three steps',
    'home.cta.title': 'Ready to route production traffic?',
    'home.footer.tagline': 'Reliable AI API gateway for teams and developers.',
    'home.login': 'Login',
    'home.docs': 'Docs',
    'home.viewDocs': 'View Documentation',
    'home.switchToLight': 'Switch to Light Mode',
    'home.switchToDark': 'Switch to Dark Mode',
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const mountHome = async () => {
  const wrapper = mount(HomeView, {
    global: {
      stubs: {
        Icon: {
          props: ['name'],
          template: '<span class="icon-stub" :data-icon="name" />',
        },
        LocaleSwitcher: {
          template: '<button type="button">Locale</button>',
        },
        RouterLink: {
          props: ['to'],
          template: '<a :href="typeof to === \'string\' ? to : \'#\'"><slot /></a>',
        },
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('HomeView default homepage', () => {
  beforeEach(() => {
    appStoreState.cachedPublicSettings = null
    appStoreState.siteName = 'PassionAPI'
    appStoreState.siteLogo = ''
    appStoreState.docUrl = 'https://docs.example.com'
    appStoreState.publicSettingsLoaded = true
    appStoreState.fetchPublicSettings.mockReset()

    authStoreState.isAuthenticated = false
    authStoreState.isAdmin = false
    authStoreState.user = null
    authStoreState.checkAuth.mockReset()

    localStorage.clear()
    document.documentElement.classList.remove('dark')
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation(() => ({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    })
  })

  it('renders the redesigned acquisition homepage copy', async () => {
    const wrapper = await mountHome()

    expect(wrapper.text()).toContain('A unified gateway for reliable multi-model API access')
    expect(wrapper.text()).toContain('Live routing console')
    expect(wrapper.text()).toContain('Everything needed to operate an API gateway')
    expect(wrapper.text()).toContain('OpenAI-compatible by design')
    expect(wrapper.text()).toContain('Launch in three steps')
    expect(wrapper.text()).toContain('Ready to route production traffic?')
  })

  it('renders motion hooks for the default homepage experience', async () => {
    const wrapper = await mountHome()

    expect(wrapper.find('.home-motion-root').exists()).toBe(true)
    expect(wrapper.find('.home-routing-panel').exists()).toBe(true)
    expect(wrapper.findAll('.home-status-pulse').length).toBeGreaterThan(0)
    expect(wrapper.findAll('.home-motion-card').length).toBeGreaterThanOrEqual(6)
  })

  it('keeps configured custom home content as a full-page override', async () => {
    appStoreState.cachedPublicSettings = {
      site_name: 'PassionAPI',
      site_logo: '',
      site_subtitle: '',
      doc_url: '',
      home_content: '<main><h1>Custom landing page</h1></main>',
    }

    const wrapper = await mountHome()

    expect(wrapper.html()).toContain('Custom landing page')
    expect(wrapper.text()).not.toContain('A unified gateway for reliable multi-model API access')
    expect(wrapper.find('.home-motion-root').exists()).toBe(false)
  })
})
