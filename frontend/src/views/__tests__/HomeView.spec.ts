import { readFileSync } from 'node:fs'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HomeView from '@/views/HomeView.vue'

const appStoreState = reactive({
  cachedPublicSettings: null as null | {
    site_name?: string
    site_logo?: string
    site_logo_light?: string
    site_logo_dark?: string
    site_subtitle?: string
    doc_url?: string
    home_content?: string
  },
  siteName: 'PassionAPI',
  siteLogo: '',
  effectiveSiteLogo: '',
  docUrl: 'https://docs.example.com',
  publicSettingsLoaded: true,
  fetchPublicSettings: vi.fn(),
  syncThemeFromDocument: vi.fn(),
  setTheme: vi.fn(),
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

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStoreState,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'home.hero.titleLead': 'Passion API',
    'home.hero.titleAccent': 'One-stop API relay service',
    'home.hero.eyebrow': 'PASSION API GATEWAY',
    'home.hero.proof.compatible': 'OpenAI-compatible calls',
    'home.hero.proof.routing': 'Account pools and failover',
    'home.hero.proof.billing': 'Wallet billing and usage traces',
    'home.hero.primaryCta': 'Start using',
    'home.hero.dashboardCta': 'Enter dashboard',
    'home.hero.secondaryCta': 'Read docs',
    'home.hero.statusBadge': 'Live now',
    'home.hero.panelTitle': 'Routing and billing board',
    'home.trust.multiModel.title': 'Multi-model access',
    'home.sections.capabilitiesTitle': 'Accounts, routing, and billing in one workspace',
    'home.integration.title': 'No client rewrite, just change the endpoint',
    'home.workflow.title': 'Verify calls first, then turn on operations',
    'home.cta.title': 'Ready to Get Started?',
    'home.cta.subtitle': 'Start with one business request, then turn on billing, routing, and risk controls as you grow.',
    'home.capabilities.unifiedApi.title': 'Keep existing clients',
    'home.capabilities.accountPool.title': 'Managed account pools',
    'home.capabilities.wallet.title': 'Wallet and ledgers',
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
    appStoreState.effectiveSiteLogo = ''
    appStoreState.docUrl = 'https://docs.example.com'
    appStoreState.publicSettingsLoaded = true
    appStoreState.fetchPublicSettings.mockReset()
    appStoreState.syncThemeFromDocument.mockReset()
    appStoreState.setTheme.mockReset()

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

    expect(wrapper.text()).toContain('Passion API')
    expect(wrapper.text()).toContain('One-stop API relay service')
    expect(wrapper.text()).toContain('PASSION API GATEWAY')
    expect(wrapper.text()).not.toContain('Routing and billing board')
    expect(wrapper.text()).not.toContain('182 ms')
    expect(wrapper.text()).not.toContain('99.9%')
    expect(wrapper.text()).toContain('Keep existing clients')
    expect(wrapper.text()).toContain('Managed account pools')
    expect(wrapper.text()).toContain('Wallet and ledgers')
    expect(wrapper.find('.home-minimal-hero').exists()).toBe(true)
    expect(wrapper.find('.home-minimal-connection').exists()).toBe(true)
  })

  it('renders motion hooks for the default homepage experience', async () => {
    const wrapper = await mountHome()

    expect(wrapper.find('.home-minimal-root').exists()).toBe(true)
    expect(wrapper.find('.home-minimal-header').classes()).toContain('fixed')
    expect(wrapper.find('.home-minimal-logo').exists()).toBe(true)
    expect(wrapper.find('.home-minimal-orb').exists()).toBe(true)
    expect(wrapper.findAll('.home-minimal-reveal').length).toBeGreaterThanOrEqual(6)
  })

  it('uses a transparent navigation shell for the homepage header', async () => {
    const wrapper = await mountHome()

    expect(wrapper.find('.home-minimal-logo').exists()).toBe(true)
    expect(wrapper.findAll('nav a[href^="#"]')).toHaveLength(0)

    const source = readFileSync('src/views/HomeView.vue', 'utf-8')
    const styleSource = source
    expect(source).toContain('home-minimal-root')
    expect(source).toContain('home-minimal-header')
    expect(source).toContain('home-minimal-hero')
    expect(source).toContain('home-minimal-orb')
    expect(source).toContain('home-minimal-primary')
    expect(source).toContain('home-minimal-reveal')
    expect(styleSource).toContain('.home-minimal-root')
    expect(styleSource).toContain('.home-minimal-header')
    expect(styleSource).toContain('prefers-reduced-motion: reduce')
  })

  it('keeps the default homepage controls contained on narrow screens', async () => {
    const wrapper = await mountHome()
    const source = readFileSync('src/views/HomeView.vue', 'utf-8')

    expect(wrapper.find('.home-minimal-root').classes()).toContain('overflow-x-hidden')
    expect(wrapper.find('.home-minimal-logo').classes()).toContain('shrink-0')
    expect(wrapper.find('.home-minimal-primary').classes()).toContain('w-full')
    expect(source).toContain('min-h-12 w-full')
    expect(source).toContain('@media (max-width: 640px)')
    expect(source).toContain('home-minimal-header')
  })

  it('does not prepend the authenticated user initial to the header dashboard CTA', async () => {
    authStoreState.isAuthenticated = true
    authStoreState.user = { email: 'admin@example.com' }

    const wrapper = await mountHome()
    const dashboardLinks = wrapper.findAll('a[href="/dashboard"]')

    expect(dashboardLinks.length).toBeGreaterThan(0)
    expect(dashboardLinks[0].text()).toBe('Enter dashboard')
  })

  it('keeps entrance motion perceptible enough for the default homepage', () => {
    const source = readFileSync('src/views/HomeView.vue', 'utf-8')

    expect(source).toContain('--home-motion-ease')
    expect(source).toContain('home-minimal-rise')
    expect(source).toContain('calc(80ms + (var(--motion-index) * 90ms))')
    expect(source).toContain('@keyframes home-orb-float')
    expect(source).toContain('@keyframes home-grid-pan')
    expect(source).toContain('home-grid-pan 22s linear infinite')
    expect(source).toContain('@keyframes home-orb-spin')
    expect(source).toContain('home-orb-track')
  })

  it('uses the blue-slate-cyan technology palette as the primary theme', () => {
    const source = readFileSync('tailwind.config.js', 'utf-8')

    expect(source).toContain('主色调 - Electric Blue + Slate/Cyan 科技蓝灰系')
    expect(source).toContain("500: '#3b82f6'")
    expect(source).toContain("600: '#2563eb'")
    expect(source).toContain("700: '#1d4ed8'")
    expect(source).toContain("'gradient-primary': 'linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)'")
    expect(source).toContain('rgba(59, 130, 246')
    expect(source).toContain('rgba(14, 116, 144')
    expect(source).toContain('rgba(6, 182, 212')
    expect(source).not.toContain('rgba(139, 92, 246')
    expect(source).not.toContain('#14b8a6')
    expect(source).not.toContain('rgba(20, 184, 166')
  })

  it('adapts homepage brand accents for both light and dark themes', () => {
    const source = readFileSync('src/views/HomeView.vue', 'utf-8')

    expect(source).toContain('home-minimal-grid')
    expect(source).toContain('from-blue-700 via-blue-600 to-cyan-500')
    expect(source).toContain('dark:from-blue-300')
    expect(source).toContain('home-minimal-logo')
    expect(source).not.toContain('rgba(139, 92, 246')
    expect(source).not.toContain('rgba(20, 184, 166, 0.22)')
  })

  it('keeps the reinforced brand theme hooks for sharper UI surfaces', () => {
    const source = readFileSync('src/style.css', 'utf-8')

    expect(source).toContain('.brand-surface')
    expect(source).toContain('.brand-rail')
    expect(source).toContain('.theme-crisp')
    expect(source).toContain('.home-site-header-scrolled')
    expect(source).toContain('rgba(255, 255, 255, 0.78)')
    expect(source).toContain('rgba(2, 6, 23, 0.72)')
    expect(source).toContain('0 12px 30px rgba(37, 99, 235, 0.28)')
    expect(source).toContain('linear-gradient(90deg, var(--brand-600), var(--brand-500), var(--brand-cyan))')
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
    expect(wrapper.text()).not.toContain('One-stop API relay service')
    expect(wrapper.find('.home-motion-root').exists()).toBe(false)
  })
})
