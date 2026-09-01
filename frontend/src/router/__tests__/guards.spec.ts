import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { resolveCompletedSetupRedirectPath } from '@/router/setupRedirect'

// Mock 导航加载状态
vi.mock('@/composables/useNavigationLoading', () => {
  const mockStart = vi.fn()
  const mockEnd = vi.fn()
  return {
    useNavigationLoadingState: () => ({
      startNavigation: mockStart,
      endNavigation: mockEnd,
      isLoading: { value: false },
    }),
    useNavigationLoading: () => ({
      startNavigation: mockStart,
      endNavigation: mockEnd,
      isLoading: { value: false },
    }),
  }
})

// Mock 路由预加载
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

// Mock API 相关模块
vi.mock('@/api', () => ({
  authAPI: {
    getCurrentUser: vi.fn().mockResolvedValue({ data: {} }),
    logout: vi.fn(),
  },
  isTotp2FARequired: () => false,
}))

vi.mock('@/api/admin/system', () => ({
  checkUpdates: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn(),
}))


// 用于测试的 auth 状态
interface MockAuthState {
  isAuthenticated: boolean
  isAdmin: boolean
  canAccessManagerPage?: boolean
  canAccessAdminWorkbench?: boolean
  isSimpleMode: boolean
  backendModeEnabled: boolean
  hasPendingAuthSession: boolean
  setupNeedsSetup?: boolean
}

/**
 * 将 router/index.ts 中 beforeEach 守卫的核心逻辑提取为可测试的函数
 */
function simulateGuard(
  toPath: string,
  toMeta: Record<string, any>,
  authState: MockAuthState
): string | null {
  const requiresAuth = toMeta.requiresAuth !== false
  const requiresAdmin = toMeta.requiresAdmin === true
  const requiresManager = toMeta.requiresManager === true
  const requiresAdminWorkbench = toMeta.requiresAdminWorkbench === true
  const canAccessAdminWorkbench =
    authState.canAccessAdminWorkbench ?? authState.canAccessManagerPage ?? authState.isAdmin
  const canAccessManagerPage = authState.canAccessManagerPage ?? canAccessAdminWorkbench

  if (toPath === '/setup' && authState.setupNeedsSetup === false) {
    return resolveCompletedSetupRedirectPath(
      authState.isAuthenticated,
      authState.isAdmin,
      authState.canAccessAdminWorkbench
    )
  }

  // 不需要认证的路由
  if (!requiresAuth) {
    if (
      authState.isAuthenticated &&
      (toPath === '/login' || toPath === '/register')
    ) {
      if (authState.backendModeEnabled && !canAccessAdminWorkbench) {
        return null
      }
      return authState.isAdmin
        ? '/admin/dashboard'
        : canAccessAdminWorkbench
          ? '/admin/workbench'
          : '/dashboard'
    }
    if (authState.backendModeEnabled && !authState.isAuthenticated) {
      const allowed = ['/login', '/key-usage', '/setup', '/payment/result']
      const callbackPaths = [
        '/auth/callback',
        '/auth/linuxdo/callback',
        '/auth/oidc/callback',
        '/auth/wechat/callback',
        '/auth/wechat/payment/callback',
      ]
      const pendingAuthPaths = ['/register', '/email-verify']
      const isAllowed =
        allowed.some((path) => toPath === path || toPath.startsWith(path)) ||
        callbackPaths.includes(toPath) ||
        (authState.hasPendingAuthSession && pendingAuthPaths.includes(toPath))
      if (!isAllowed) {
        return '/login'
      }
    }
    return null // 允许通过
  }

  // 需要认证但未登录
  if (!authState.isAuthenticated) {
    return '/login'
  }

  // 需要管理员但不是管理员
  if (requiresAdmin && !authState.isAdmin) {
    return canAccessAdminWorkbench ? '/admin/workbench' : '/dashboard'
  }

  if (requiresAdminWorkbench && !canAccessAdminWorkbench) {
    return '/dashboard'
  }

  if (requiresManager && !canAccessManagerPage) {
    return '/dashboard'
  }

  // 简易模式限制
  if (authState.isSimpleMode) {
    const restrictedPaths = [
      '/admin/groups',
      '/admin/subscriptions',
      '/admin/redeem',
      '/subscriptions',
      '/redeem',
    ]
    if (restrictedPaths.some((path) => toPath.startsWith(path))) {
      return authState.isAdmin ? '/admin/dashboard' : '/dashboard'
    }
  }

  // Backend mode: sub-admins can access both the workbench and legacy manager page.
  if (authState.backendModeEnabled) {
    if (authState.isAuthenticated && authState.isAdmin) {
      return null
    }
    if (authState.isAuthenticated && canAccessAdminWorkbench) {
      return requiresAdminWorkbench ||
        requiresManager ||
        toPath.startsWith('/admin/workbench') ||
        toPath.startsWith('/manager')
        ? null
        : '/admin/workbench'
    }
    const allowed = ['/login', '/key-usage', '/setup', '/payment/result']
    const callbackPaths = [
      '/auth/callback',
      '/auth/linuxdo/callback',
      '/auth/oidc/callback',
      '/auth/wechat/callback',
      '/auth/wechat/payment/callback',
    ]
    const pendingAuthPaths = ['/register', '/email-verify']
    const isAllowed =
      allowed.some((path) => toPath === path || toPath.startsWith(path)) ||
      callbackPaths.includes(toPath) ||
      (authState.hasPendingAuthSession && pendingAuthPaths.includes(toPath))
    if (!isAllowed) {
      return '/login'
    }
  }

  return null // 允许通过
}

describe('路由守卫逻辑', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  // --- 未认证用户 ---

  describe('未认证用户', () => {
    const authState: MockAuthState = {
      isAuthenticated: false,
      isAdmin: false,
      canAccessAdminWorkbench: false,
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问需要认证的页面重定向到 /login', () => {
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBe('/login')
    })

    it('访问管理页面重定向到 /login', () => {
      const redirect = simulateGuard('/admin/dashboard', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/login')
    })

    it('访问公开页面允许通过', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('访问 /home 公开页面允许通过', () => {
      const redirect = simulateGuard('/home', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })
  })

  // --- 已认证普通用户 ---

  describe('已认证普通用户', () => {
    const authState: MockAuthState = {
      isAuthenticated: true,
      isAdmin: false,
      canAccessAdminWorkbench: false,
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问 /login 重定向到 /dashboard', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('访问 /register 重定向到 /dashboard', () => {
      const redirect = simulateGuard('/register', { requiresAuth: false }, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('访问 /dashboard 允许通过', () => {
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBeNull()
    })

    it('访问管理页面被拒绝，重定向到 /dashboard', () => {
      const redirect = simulateGuard('/admin/dashboard', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('访问 /admin/users 被拒绝', () => {
      const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/dashboard')
    })
  })

  describe('已认证二级管理员', () => {
    const authState: MockAuthState = {
      isAuthenticated: true,
      isAdmin: false,
      canAccessAdminWorkbench: true,
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问 /admin/workbench 允许通过', () => {
      const redirect = simulateGuard('/admin/workbench', { requiresAdminWorkbench: true }, authState)
      expect(redirect).toBeNull()
    })

    it('访问完整后台被拒绝，重定向到管理员页面', () => {
      const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/admin/workbench')
    })

    it('访问 /login 重定向到 /admin/workbench', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/workbench')
    })
  })

  // --- 已认证管理员 ---

  describe('已认证管理员', () => {
    const authState: MockAuthState = {
      isAuthenticated: true,
      isAdmin: true,
      canAccessAdminWorkbench: true,
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问 /login 重定向到 /admin/dashboard', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/dashboard')
    })

    it('访问管理页面允许通过', () => {
      const redirect = simulateGuard('/admin/dashboard', { requiresAdmin: true }, authState)
      expect(redirect).toBeNull()
    })

    it('访问用户页面允许通过', () => {
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBeNull()
    })
  })

  describe('已认证二级管理员', () => {
    const authState: MockAuthState = {
      isAuthenticated: true,
      isAdmin: false,
      canAccessAdminWorkbench: true,
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问 /login 重定向到管理员页面', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/workbench')
    })

    it('访问管理员页面允许通过', () => {
      const redirect = simulateGuard('/admin/workbench', { requiresAdminWorkbench: true }, authState)
      expect(redirect).toBeNull()
    })

    it('访问完整后台被重定向回管理员页面', () => {
      const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/admin/workbench')
    })
  })

  // --- 简易模式 ---

  describe('简易模式受限路由', () => {
    it('普通用户简易模式访问 /subscriptions 重定向到 /dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/subscriptions', {}, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('普通用户简易模式访问 /redeem 重定向到 /dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/redeem', {}, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('管理员简易模式访问 /admin/groups 重定向到 /admin/dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        canAccessAdminWorkbench: true,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/admin/groups', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/admin/dashboard')
    })

    it('管理员简易模式访问 /admin/subscriptions 重定向', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        canAccessAdminWorkbench: true,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard(
        '/admin/subscriptions',
        { requiresAdmin: true },
        authState
      )
      expect(redirect).toBe('/admin/dashboard')
    })

    it('简易模式下非受限页面正常访问', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBeNull()
    })

    it('简易模式下 /keys 正常访问', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/keys', {}, authState)
      expect(redirect).toBeNull()
    })
  })

  describe('Backend Mode', () => {
    it('unauthenticated: /home redirects to /login', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/home', { requiresAuth: false }, authState)
      expect(redirect).toBe('/login')
    })

    it('unauthenticated: /login is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /key-usage is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/key-usage', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /setup is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/setup', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: initialized /setup redirects to /login', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
        setupNeedsSetup: false,
      }
      const redirect = simulateGuard('/setup', { requiresAuth: false }, authState)
      expect(redirect).toBe('/login')
    })

    it('admin: initialized /setup redirects to /admin/dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
        setupNeedsSetup: false,
      }
      const redirect = simulateGuard('/setup', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/dashboard')
    })

    it('admin: /admin/dashboard is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/admin/dashboard', { requiresAdmin: true }, authState)
      expect(redirect).toBeNull()
    })

    it('admin: /login redirects to /admin/dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/dashboard')
    })

    it('non-admin authenticated: /dashboard redirects to /login', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBe('/login')
    })

    it('sub-admin authenticated: /dashboard redirects to /admin/workbench', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBe('/admin/workbench')
    })

    it('sub-admin authenticated: /admin/workbench is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/admin/workbench', { requiresAdminWorkbench: true }, authState)
      expect(redirect).toBeNull()
    })

    it('non-admin authenticated: /login is allowed (no redirect loop)', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('non-admin authenticated: /key-usage is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/key-usage', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('secondary admin: /manager is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/manager', { requiresManager: true }, authState)
      expect(redirect).toBeNull()
    })

    it('secondary admin: /dashboard redirects to /admin/workbench', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBe('/admin/workbench')
    })

    it('secondary admin: /admin/users redirects to /admin/workbench', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/admin/workbench')
    })

    it('secondary admin: /login redirects to /admin/workbench', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        canAccessAdminWorkbench: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/workbench')
    })

    it('unauthenticated: callback routes are allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/auth/wechat/callback', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: WeChat payment callback route is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/auth/wechat/payment/callback', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /payment/result is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/payment/result', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /register is allowed when a pending auth session exists', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: true,
      }
      const redirect = simulateGuard('/register', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /email-verify is blocked without a pending auth session', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        canAccessAdminWorkbench: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/email-verify', { requiresAuth: false }, authState)
      expect(redirect).toBe('/login')
    })
  })
})
