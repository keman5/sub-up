import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

// Mock authAPI
const mockLogin = vi.fn()
const mockLogin2FA = vi.fn()
const mockLogout = vi.fn()
const mockGetCurrentUser = vi.fn()
const mockRegister = vi.fn()
const mockRefreshToken = vi.fn()

vi.mock('@/api', () => ({
  authAPI: {
    login: (...args: any[]) => mockLogin(...args),
    login2FA: (...args: any[]) => mockLogin2FA(...args),
    logout: (...args: any[]) => mockLogout(...args),
    getCurrentUser: (...args: any[]) => mockGetCurrentUser(...args),
    register: (...args: any[]) => mockRegister(...args),
    refreshToken: (...args: any[]) => mockRefreshToken(...args),
  },
  isTotp2FARequired: (response: any) => response?.requires_2fa === true,
}))

const fakeUser = {
  id: 1,
  username: 'testuser',
  email: 'test@example.com',
  role: 'user' as const,
  balance: 100,
  concurrency: 5,
  status: 'active' as const,
  allowed_groups: null,
  created_at: '2024-01-01',
  updated_at: '2024-01-01',
}

const fakeAdminUser = {
  ...fakeUser,
  id: 2,
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin' as const,
}

const fakeSuperAdminUser = {
  ...fakeUser,
  id: 3,
  username: 'super-admin',
  email: 'super-admin@example.com',
  role: 'super_admin' as const,
}

const fakeAuthResponse = {
  access_token: 'test-token-123',
  refresh_token: 'refresh-token-456',
  expires_in: 3600,
  token_type: 'Bearer',
  user: { ...fakeUser },
}

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.useFakeTimers()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // --- login ---

  describe('login', () => {
    it('成功登录后设置 token 和 user', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.token).toBe('test-token-123')
      expect(store.user).toEqual(fakeUser)
      expect(store.isAuthenticated).toBe(true)
      expect(localStorage.getItem('auth_token')).toBe('test-token-123')
      expect(localStorage.getItem('auth_user')).toBe(JSON.stringify(fakeUser))
    })

    it('短有效期 token 登录后不主动定时刷新，避免后台 refresh 循环', async () => {
      mockLogin.mockResolvedValue({
        ...fakeAuthResponse,
        expires_in: 60,
      })
      mockRefreshToken.mockResolvedValue({
        access_token: 'refreshed-token',
        refresh_token: 'refreshed-refresh-token',
        expires_in: 60,
      })
      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })
      await Promise.resolve()
      await Promise.resolve()

      expect(mockRefreshToken).not.toHaveBeenCalled()

      await vi.advanceTimersByTimeAsync(29_000)
      expect(mockRefreshToken).not.toHaveBeenCalled()

      await vi.advanceTimersByTimeAsync(5 * 60_000)
      expect(mockRefreshToken).not.toHaveBeenCalled()
      expect(mockGetCurrentUser).not.toHaveBeenCalled()
    })

    it('长有效期 token 登录后不会因 setTimeout 上限立即 refresh', async () => {
      mockLogin.mockResolvedValue({
        ...fakeAuthResponse,
        expires_in: 30 * 24 * 60 * 60,
      })
      mockRefreshToken.mockResolvedValue({
        access_token: 'refreshed-token',
        refresh_token: 'refreshed-refresh-token',
        expires_in: 30 * 24 * 60 * 60,
      })
      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })
      await Promise.resolve()
      await Promise.resolve()

      expect(mockRefreshToken).not.toHaveBeenCalled()

      await vi.advanceTimersByTimeAsync(60_000)

      expect(mockRefreshToken).not.toHaveBeenCalled()
    })

    it('外部刷新成功后重排 token 刷新定时器，避免成功后立即再次刷新', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      window.dispatchEvent(new CustomEvent('auth-token-refreshed', {
        detail: {
          access_token: 'interceptor-token',
          refresh_token: 'interceptor-refresh-token',
          expires_in: 7200,
        },
      }))

      await vi.advanceTimersByTimeAsync(3480_000)

      expect(mockRefreshToken).not.toHaveBeenCalled()
      expect(store.token).toBe('interceptor-token')
      expect(localStorage.getItem('refresh_token')).toBe('interceptor-refresh-token')
    })

    it('外部刷新返回短有效期 token 后不启动后台 refresh 循环', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      window.dispatchEvent(new CustomEvent('auth-token-refreshed', {
        detail: {
          access_token: 'short-interceptor-token',
          refresh_token: 'short-interceptor-refresh-token',
          expires_in: 60,
        },
      }))

      await vi.advanceTimersByTimeAsync(5 * 60_000)

      expect(mockRefreshToken).not.toHaveBeenCalled()
      expect(mockGetCurrentUser).not.toHaveBeenCalled()
      expect(store.token).toBe('short-interceptor-token')
      expect(localStorage.getItem('refresh_token')).toBe('short-interceptor-refresh-token')
    })

    it('登录失败时清除状态并抛出错误', async () => {
      mockLogin.mockRejectedValue(new Error('Invalid credentials'))
      const store = useAuthStore()

      await expect(store.login({ email: 'test@example.com', password: 'wrong' })).rejects.toThrow(
        'Invalid credentials'
      )

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('需要 2FA 时返回响应但不设置认证状态', async () => {
      const twoFAResponse = { requires_2fa: true, temp_token: 'temp-123' }
      mockLogin.mockResolvedValue(twoFAResponse)
      const store = useAuthStore()

      const result = await store.login({ email: 'test@example.com', password: '123456' })

      expect(result).toEqual(twoFAResponse)
      expect(store.token).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })
  })

  // --- login2FA ---

  describe('login2FA', () => {
    it('2FA 验证成功后设置认证状态', async () => {
      mockLogin2FA.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      const user = await store.login2FA('temp-123', '654321')

      expect(store.token).toBe('test-token-123')
      expect(store.user).toEqual(fakeUser)
      expect(user).toEqual(fakeUser)
      expect(mockLogin2FA).toHaveBeenCalledWith({
        temp_token: 'temp-123',
        totp_code: '654321',
      })
    })

    it('2FA 验证失败时清除状态并抛出错误', async () => {
      mockLogin2FA.mockRejectedValue(new Error('Invalid TOTP'))
      const store = useAuthStore()

      await expect(store.login2FA('temp-123', '000000')).rejects.toThrow('Invalid TOTP')
      expect(store.token).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })
  })

  // --- logout ---

  describe('logout', () => {
    it('注销后清除所有状态和 localStorage', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      mockLogout.mockResolvedValue(undefined)
      const store = useAuthStore()

      // 先登录
      await store.login({ email: 'test@example.com', password: '123456' })
      expect(store.isAuthenticated).toBe(true)

      // 注销
      await store.logout()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(localStorage.getItem('auth_user')).toBeNull()
      expect(localStorage.getItem('refresh_token')).toBeNull()
      expect(localStorage.getItem('token_expires_at')).toBeNull()
    })
  })

  // --- checkAuth ---

  describe('checkAuth', () => {
    it('从 localStorage 恢复持久化状态', () => {
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeUser))

      // Mock refreshUser (getCurrentUser) 防止后台刷新报错
      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })

      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBe('saved-token')
      expect(store.user).toEqual(fakeUser)
      expect(store.isAuthenticated).toBe(true)
    })

    it('localStorage 无数据时保持未认证状态', () => {
      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('localStorage 中用户数据损坏时清除状态', () => {
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', 'invalid-json{{{')

      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(localStorage.getItem('auth_token')).toBeNull()
    })

    it('恢复 refresh token 和过期时间', () => {
      const futureTs = String(Date.now() + 3600_000)
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeUser))
      localStorage.setItem('refresh_token', 'saved-refresh')
      localStorage.setItem('token_expires_at', futureTs)

      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })

      const store = useAuthStore()
      store.checkAuth()

      expect(store.isAuthenticated).toBe(true)
    })

    it('恢复已过期 token 时不后台请求用户信息或立即 refresh', async () => {
      localStorage.setItem('auth_token', 'expired-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeUser))
      localStorage.setItem('refresh_token', 'saved-refresh')
      localStorage.setItem('token_expires_at', String(Date.now() - 1000))

      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })
      mockRefreshToken.mockResolvedValue({
        access_token: 'refreshed-token',
        refresh_token: 'refreshed-refresh-token',
        expires_in: 60,
      })

      const store = useAuthStore()
      store.checkAuth()
      await Promise.resolve()
      await Promise.resolve()

      expect(store.isAuthenticated).toBe(true)
      expect(mockGetCurrentUser).not.toHaveBeenCalled()
      expect(mockRefreshToken).not.toHaveBeenCalled()
    })

    it('恢复已过期 token 时保留本地登录态并等待真实请求刷新', async () => {
      localStorage.setItem('auth_token', 'expired-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeUser))
      localStorage.setItem('refresh_token', 'stale-refresh')
      localStorage.setItem('token_expires_at', String(Date.now() - 1000))

      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })
      mockRefreshToken.mockRejectedValue({ status: 401, code: 'INVALID_REFRESH_TOKEN' })

      const store = useAuthStore()
      store.checkAuth()
      await Promise.resolve()
      await Promise.resolve()

      expect(mockGetCurrentUser).not.toHaveBeenCalled()
      expect(mockRefreshToken).not.toHaveBeenCalled()
      expect(store.isAuthenticated).toBe(true)
      expect(localStorage.getItem('auth_token')).toBe('expired-token')
      expect(localStorage.getItem('refresh_token')).toBe('stale-refresh')
    })

    it('恢复持久化 pending auth session', () => {
      localStorage.setItem(
        'pending_auth_session',
        JSON.stringify({
          token: 'pending-token',
          token_field: 'pending_auth_token',
          provider: 'wechat',
          redirect: '/profile',
        })
      )

      const store = useAuthStore()
      store.checkAuth()

      expect(store.hasPendingAuthSession).toBe(true)
      expect(store.pendingAuthSession).toEqual({
        token: 'pending-token',
        token_field: 'pending_auth_token',
        provider: 'wechat',
        redirect: '/profile',
      })
    })
  })

  // --- setToken ---

  describe('setToken', () => {
    it('短有效期 OAuth token 设置后不立即 refresh，避免回调登录后 refresh 循环', async () => {
      localStorage.setItem('refresh_token', 'oauth-refresh-token')
      localStorage.setItem('token_expires_at', String(Date.now() + 60_000))
      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })
      mockRefreshToken.mockResolvedValue({
        access_token: 'refreshed-token',
        refresh_token: 'refreshed-refresh-token',
        expires_in: 60,
      })

      const store = useAuthStore()
      await store.setToken('oauth-access-token')
      await Promise.resolve()
      await Promise.resolve()

      expect(mockGetCurrentUser).toHaveBeenCalledTimes(1)
      expect(mockRefreshToken).not.toHaveBeenCalled()

      await vi.advanceTimersByTimeAsync(5 * 60_000)

      expect(mockRefreshToken).not.toHaveBeenCalled()
      expect(store.token).toBe('oauth-access-token')
    })
  })

  describe('pending auth session', () => {
    it('persists and clears pending auth session state', () => {
      const store = useAuthStore()

      store.setPendingAuthSession({
        token: 'pending-token',
        token_field: 'pending_auth_token',
        provider: 'wechat',
        redirect: '/profile',
      })

      expect(store.hasPendingAuthSession).toBe(true)
      expect(JSON.parse(localStorage.getItem('pending_auth_session') || 'null')).toEqual({
        token: 'pending-token',
        token_field: 'pending_auth_token',
        provider: 'wechat',
        redirect: '/profile',
      })

      store.clearPendingAuthSession()

      expect(store.hasPendingAuthSession).toBe(false)
      expect(localStorage.getItem('pending_auth_session')).toBeNull()
    })

    it('restores a persisted pending oauth session without requiring a token value', () => {
      const firstStore = useAuthStore()

      firstStore.setPendingAuthSession({
        token: '',
        token_field: 'pending_oauth_token',
        provider: 'oidc',
        redirect: '/welcome',
        adoption_required: true,
        suggested_display_name: 'OIDC Nick'
      })

      setActivePinia(createPinia())
      const restoredStore = useAuthStore()
      restoredStore.checkAuth()

      expect(restoredStore.isAuthenticated).toBe(false)
      expect(restoredStore.hasPendingAuthSession).toBe(true)
      expect(restoredStore.pendingAuthSession).toEqual({
        token: '',
        token_field: 'pending_oauth_token',
        provider: 'oidc',
        redirect: '/welcome',
        adoption_required: true,
        suggested_display_name: 'OIDC Nick',
        suggested_avatar_url: undefined
      })
    })

    it('preserves pending auth session when registration fails', async () => {
      const store = useAuthStore()
      store.setPendingAuthSession({
        token: 'pending-token',
        token_field: 'pending_auth_token',
        provider: 'oidc',
        redirect: '/register',
      })
      mockRegister.mockRejectedValue(new Error('Register failed'))

      await expect(
        store.register({ email: 'user@example.com', password: 'secret-123' })
      ).rejects.toThrow('Register failed')

      expect(store.hasPendingAuthSession).toBe(true)
      expect(store.pendingAuthSession).toEqual({
        token: 'pending-token',
        token_field: 'pending_auth_token',
        provider: 'oidc',
        redirect: '/register',
      })
    })
  })

  // --- isAdmin ---

  describe('isAdmin', () => {
    it('管理员用户返回 true', async () => {
      const adminResponse = { ...fakeAuthResponse, user: { ...fakeAdminUser } }
      mockLogin.mockResolvedValue(adminResponse)
      const store = useAuthStore()

      await store.login({ email: 'admin@example.com', password: '123456' })

      expect(store.isAdmin).toBe(true)
    })

    it('超级管理员用户返回 true', async () => {
      const superAdminResponse = { ...fakeAuthResponse, user: { ...fakeSuperAdminUser } }
      mockLogin.mockResolvedValue(superAdminResponse)
      const store = useAuthStore()

      await store.login({ email: 'super-admin@example.com', password: '123456' })

      expect(store.isAdmin).toBe(true)
    })

    it('普通用户返回 false', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.isAdmin).toBe(false)
    })

    it('未登录时返回 false', () => {
      const store = useAuthStore()
      expect(store.isAdmin).toBe(false)
    })
  })

  // --- refreshUser ---

  describe('refreshUser', () => {
    it('刷新用户数据并更新 localStorage', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()
      await store.login({ email: 'test@example.com', password: '123456' })

      const updatedUser = { ...fakeUser, username: 'updated-name' }
      mockGetCurrentUser.mockResolvedValue({ data: updatedUser })

      const result = await store.refreshUser()

      expect(result).toEqual(updatedUser)
      expect(store.user).toEqual(updatedUser)
      expect(JSON.parse(localStorage.getItem('auth_user')!)).toEqual(updatedUser)
    })

    it('未认证时抛出错误', async () => {
      const store = useAuthStore()
      await expect(store.refreshUser()).rejects.toThrow('Not authenticated')
    })
  })

  // --- isSimpleMode ---

  describe('isSimpleMode', () => {
    it('run_mode 为 simple 时返回 true', async () => {
      const simpleResponse = {
        ...fakeAuthResponse,
        user: { ...fakeUser, run_mode: 'simple' as const },
      }
      mockLogin.mockResolvedValue(simpleResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.isSimpleMode).toBe(true)
    })

    it('默认为 standard 模式', () => {
      const store = useAuthStore()
      expect(store.isSimpleMode).toBe(false)
    })
  })
})
