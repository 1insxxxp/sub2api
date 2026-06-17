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
    expect(wrapper.text()).toContain('OpenAI-compatible calls')
    expect(wrapper.text()).not.toContain('Keep OpenAI-compatible calls while managing account pools')
    expect(wrapper.text()).toContain('Routing and billing board')
    expect(wrapper.text()).toContain('Accounts, routing, and billing in one workspace')
    expect(wrapper.text()).toContain('No client rewrite, just change the endpoint')
    expect(wrapper.text()).toContain('Verify calls first, then turn on operations')
    expect(wrapper.text()).toContain('Start with one business request')
  })

  it('renders motion hooks for the default homepage experience', async () => {
    const wrapper = await mountHome()

    expect(wrapper.find('.home-motion-root').exists()).toBe(true)
    expect(wrapper.find('.home-site-header').classes()).toContain('fixed')
    expect(wrapper.find('.home-site-header-spacer').exists()).toBe(true)
    expect(wrapper.find('.home-hero-overline').exists()).toBe(true)
    expect(wrapper.findAll('.home-proof-chip')).toHaveLength(3)
    expect(wrapper.findAll('.home-proof-icon')).toHaveLength(3)
    expect(wrapper.find('.home-routing-panel').exists()).toBe(true)
    expect(wrapper.find('.home-panel-icon').exists()).toBe(true)
    expect(wrapper.findAll('.home-channel-icon').length).toBeGreaterThanOrEqual(4)
    expect(wrapper.findAll('.home-metric-icon').length).toBeGreaterThanOrEqual(4)
    expect(wrapper.findAll('.home-metric-row').length).toBeGreaterThanOrEqual(4)
    expect(wrapper.findAll('.home-status-pulse').length).toBeGreaterThan(0)
    expect(wrapper.findAll('.home-status-dot').length).toBeGreaterThanOrEqual(5)
    expect(wrapper.findAll('.home-motion-card').length).toBeGreaterThanOrEqual(6)
    expect(wrapper.findAll('.home-section-reveal').length).toBeGreaterThanOrEqual(6)
    expect(wrapper.find('.home-code-panel.home-scroll-reveal').exists()).toBe(true)
    expect(wrapper.find('.home-cta-panel.home-scroll-reveal').exists()).toBe(true)
  })

  it('uses a cohesive glass navigation shell for the homepage header', async () => {
    const wrapper = await mountHome()

    expect(wrapper.find('.home-nav-shell').exists()).toBe(true)
    expect(wrapper.find('.home-brand-link').exists()).toBe(true)
    expect(wrapper.find('.home-brand-mark').exists()).toBe(true)
    expect(wrapper.find('.home-nav-rail').exists()).toBe(true)
    expect(wrapper.findAll('.home-nav-link')).toHaveLength(3)
    expect(wrapper.find('.home-header-actions').exists()).toBe(true)
    expect(wrapper.find('.home-icon-control').exists()).toBe(true)
    expect(wrapper.find('.home-dashboard-cta').exists()).toBe(true)

    const source = readFileSync('src/views/HomeView.vue', 'utf-8')
    expect(source).toContain('home-nav-shell')
    expect(source).toContain('home-header-actions')
    expect(source).toContain('home-dashboard-cta')
    expect(source).toContain('h-11')
  })

  it('does not prepend the authenticated user initial to the header dashboard CTA', async () => {
    authStoreState.isAuthenticated = true
    authStoreState.user = { email: 'admin@example.com' }

    const wrapper = await mountHome()
    const dashboardLinks = wrapper.findAll('a[href="/dashboard"]')

    expect(dashboardLinks.length).toBeGreaterThan(0)
    expect(dashboardLinks[0].text()).toBe('Enter dashboard')
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

    expect(source).toContain('rgba(219,234,254,0.92)')
    expect(source).toContain('rgba(30,64,175,0.26)')
    expect(source).toContain('home-hero-overline')
    expect(source).toContain('home-proof-chip')
    expect(source).toContain('home-panel-icon')
    expect(source).toContain('home-metric-row')
    expect(source).toContain('--status-pulse-color')
    expect(source).toContain('bg-[linear-gradient(90deg,#2563eb,#06b6d4)]')
    expect(source).toContain('text-cyan-300')
    expect(source).toContain('rgba(59, 130, 246, 0.24)')
    expect(source).toContain('rgba(14, 116, 144, 0.18)')
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
