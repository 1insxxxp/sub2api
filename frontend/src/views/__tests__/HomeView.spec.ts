import { readFileSync } from 'node:fs'
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
    expect(wrapper.findAll('.home-section-reveal').length).toBeGreaterThanOrEqual(6)
    expect(wrapper.find('.home-code-panel.home-scroll-reveal').exists()).toBe(true)
    expect(wrapper.find('.home-cta-panel.home-scroll-reveal').exists()).toBe(true)
  })

  it('keeps scroll motion perceptible enough for the default homepage', () => {
    const source = readFileSync('src/views/HomeView.vue', 'utf-8')

    expect(source).toContain('--motion-distance: 30px')
    expect(source).toContain('--motion-section-distance: 24px')
    expect(source).toContain('--motion-scale: 0.985')
    expect(source).toContain('--motion-blur: 6px')
    expect(source).toContain('calc(90ms + (var(--motion-index) * 68ms))')
  })

  it('keeps the final CTA panel crisp while it scrolls into view', () => {
    const source = readFileSync('src/views/HomeView.vue', 'utf-8')

    expect(source).toContain('.home-cta-panel.home-scroll-reveal')
    expect(source).toContain('animation-name: home-panel-rise')
    expect(source).toContain('@keyframes home-panel-rise')
  })

  it('uses the blue-violet technology palette as the primary theme', () => {
    const source = readFileSync('tailwind.config.js', 'utf-8')

    expect(source).toContain('主色调 - Electric Blue/Violet 科技蓝紫系')
    expect(source).toContain("500: '#3b82f6'")
    expect(source).toContain("600: '#2563eb'")
    expect(source).toContain("700: '#1d4ed8'")
    expect(source).toContain("'gradient-primary': 'linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)'")
    expect(source).toContain('rgba(59, 130, 246')
    expect(source).toContain('rgba(139, 92, 246')
    expect(source).toContain('rgba(6, 182, 212')
    expect(source).not.toContain('#14b8a6')
    expect(source).not.toContain('rgba(20, 184, 166')
  })

  it('adapts homepage brand accents for both light and dark themes', () => {
    const source = readFileSync('src/views/HomeView.vue', 'utf-8')

    expect(source).toContain('rgba(239,246,255,0.92)')
    expect(source).toContain('rgba(30,41,59,0.48)')
    expect(source).toContain('border-primary-200 bg-primary-50')
    expect(source).toContain('dark:border-primary-500/30 dark:bg-primary-500/10')
    expect(source).toContain('text-cyan-300')
    expect(source).toContain('rgba(59, 130, 246, 0.24)')
    expect(source).toContain('rgba(139, 92, 246, 0.2)')
    expect(source).not.toContain('rgba(20, 184, 166, 0.22)')
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
