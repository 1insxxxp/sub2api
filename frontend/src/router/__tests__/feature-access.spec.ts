import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const routerSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'),
  'utf8',
)

type NavigationGuard = (
  to: Record<string, any>,
  from: Record<string, any>,
  next: ReturnType<typeof vi.fn>
) => Promise<void>

const routerHarness = vi.hoisted(() => ({
  guard: null as NavigationGuard | null,
}))

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  canAccessAdminWorkbench: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const adminComplianceStore = vi.hoisted(() => ({
  initialized: false,
  fetchStatus: vi.fn(),
  requireAcknowledgement: vi.fn(),
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  publicSettingsLoaded: false,
  cachedPublicSettings: null as null | {
    payment_enabled?: boolean
    risk_control_enabled?: boolean
    image_studio_enabled?: boolean
    lottery_enabled?: boolean
    custom_menu_items?: []
  },
  fetchPublicSettings: vi.fn(),
}))

vi.mock('vue-router', () => ({
  createWebHistory: vi.fn(() => ({})),
  createRouter: vi.fn(() => ({
    beforeEach: vi.fn((guard: NavigationGuard) => {
      routerHarness.guard = guard
    }),
    afterEach: vi.fn(),
    onError: vi.fn(),
  })),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => adminComplianceStore,
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function runGuard(meta: Record<string, unknown>, path: string) {
  if (!routerHarness.guard) {
    throw new Error('router guard was not registered')
  }

  const next = vi.fn()
  const navigation = routerHarness.guard(
    {
      path,
      fullPath: path,
      name: 'FeatureRoute',
      params: {},
      meta: { requiresAuth: true, ...meta },
    },
    {},
    next
  )
  return { navigation, next }
}

describe('feature route guard', () => {
  beforeAll(async () => {
    await import('@/router')
  })

  beforeEach(() => {
    authStore.isAuthenticated = true
    authStore.isAdmin = false
    authStore.canAccessAdminWorkbench = false
    authStore.isSimpleMode = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockReset()
    adminComplianceStore.initialized = false
    adminComplianceStore.fetchStatus.mockReset()
    adminComplianceStore.requireAcknowledgement.mockReset()
  })

  it('marks the image studio route as feature protected', () => {
    expect(routerSource).toMatch(
      /path: '\/images',[\s\S]*?requiresImageStudio: true[\s\S]*?titleKey: 'imageStudio\.title'/,
    )
  })

  it('marks the lottery route as feature protected', () => {
    expect(routerSource).toMatch(
      /path: '\/lottery',[\s\S]*?requiresLottery: true[\s\S]*?titleKey: 'lottery\.title'/,
    )
  })

  it('marks the admin workbench route as workbench protected', () => {
    expect(routerSource).toMatch(
      /path: '\/admin\/workbench',[\s\S]*?requiresAdminWorkbench: true[\s\S]*?titleKey: 'adminWorkbench\.title'/,
    )
  })

  it('allows a sub admin to open the admin workbench route', async () => {
    authStore.canAccessAdminWorkbench = true

    const { navigation, next } = runGuard({ requiresAdminWorkbench: true }, '/admin/workbench')
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it('redirects a sub admin away from full admin routes', async () => {
    authStore.canAccessAdminWorkbench = true

    const { navigation, next } = runGuard({ requiresAdmin: true }, '/admin/users')
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith('/admin/workbench')
  })

  it('waits for the first public-settings request before deciding payment access', async () => {
    const deferred = createDeferred<{ payment_enabled: boolean }>()
    appStore.fetchPublicSettings.mockImplementation(async () => {
      const settings = await deferred.promise
      appStore.cachedPublicSettings = settings
      appStore.publicSettingsLoaded = true
      return settings
    })

    const { navigation, next } = runGuard({ requiresPayment: true }, '/purchase')

    await vi.waitFor(() => expect(appStore.fetchPublicSettings).toHaveBeenCalledTimes(1))
    expect(next).not.toHaveBeenCalled()

    deferred.resolve({ payment_enabled: true })
    await navigation
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    ['payment', { requiresPayment: true }, '/purchase'],
    ['risk control', { requiresRiskControl: true }, '/admin/risk-control'],
    ['image studio', { requiresImageStudio: true }, '/images'],
    ['lottery', { requiresLottery: true }, '/lottery'],
  ])('does not treat a failed %s settings load as explicitly disabled', async (_name, meta, path) => {
    authStore.isAdmin = meta.requiresRiskControl === true
    appStore.fetchPublicSettings.mockResolvedValue(null)

    const { navigation, next } = runGuard(meta, path)
    await navigation

    expect(appStore.publicSettingsLoaded).toBe(false)
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    ['payment', { requiresPayment: true }, { payment_enabled: false }, '/dashboard'],
    [
      'risk control',
      { requiresRiskControl: true },
      { risk_control_enabled: false },
      '/admin/settings',
    ],
    ['image studio', { requiresImageStudio: true }, { image_studio_enabled: false }, '/dashboard'],
    ['lottery', { requiresLottery: true }, { lottery_enabled: false }, '/dashboard'],
  ])('redirects when loaded settings explicitly disable %s', async (_name, meta, settings, target) => {
    authStore.isAdmin = meta.requiresRiskControl === true
    appStore.cachedPublicSettings = settings
    appStore.publicSettingsLoaded = true

    const { navigation, next } = runGuard(meta, '/feature')
    await navigation

    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith(target)
  })

  it('does not request admin compliance status for sub-admin workbench access', async () => {
    authStore.isAdmin = false
    authStore.canAccessAdminWorkbench = true

    const { navigation, next } = runGuard({ requiresAdminWorkbench: true }, '/admin/workbench')
    await navigation

    expect(adminComplianceStore.fetchStatus).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })
})
