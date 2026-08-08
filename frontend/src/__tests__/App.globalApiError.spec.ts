import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import App from '@/App.vue'

const mockRoute = {
  path: '/dashboard',
  fullPath: '/dashboard',
  meta: {},
}

const mockRouter = {
  afterEach: vi.fn(),
  replace: vi.fn(),
}

const showError = vi.fn()
const fetchPublicSettings = vi.fn().mockResolvedValue(undefined)
const fetchAnnouncements = vi.fn().mockResolvedValue(undefined)
const fetchActiveSubscriptions = vi.fn().mockResolvedValue(undefined)
const fetchComplianceStatus = vi.fn().mockResolvedValue(undefined)
const startPolling = vi.fn()
const clearSubscriptions = vi.fn()
const resetAnnouncements = vi.fn()
const resetCompliance = vi.fn()
const requireAcknowledgement = vi.fn()

vi.mock('vue-router', () => ({
  RouterView: { template: '<main data-test="router-view" />' },
  useRouter: () => mockRouter,
  useRoute: () => mockRoute,
}))

vi.mock('@/api/setup', () => ({
  getSetupStatus: vi.fn().mockResolvedValue({ needs_setup: false }),
}))

vi.mock('@/utils/siteIcons', () => ({
  applySiteIcons: vi.fn(),
}))

vi.mock('@/router/title', () => ({
  resolveRouteDocumentTitle: vi.fn(() => 'Sub2API'),
}))

vi.mock('@/components/common/Toast.vue', () => ({
  default: { template: '<div data-test="toast-host" />' },
}))

vi.mock('@/components/common/NavigationProgress.vue', () => ({
  default: { template: '<div data-test="navigation-progress" />' },
}))

vi.mock('@/components/common/AppDialogHost.vue', () => ({
  default: { template: '<div data-test="dialog-host" />' },
}))

vi.mock('@/components/admin/AdminComplianceDialog.vue', () => ({
  default: { template: '<div data-test="admin-compliance-dialog" />' },
}))

vi.mock('@/components/common/AnnouncementPopup.vue', () => ({
  default: { template: '<div data-test="announcement-popup" />' },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: null,
    siteLogo: '',
    siteName: 'Sub2API',
    showError,
    fetchPublicSettings,
  }),
  useAuthStore: () => ({
    isAuthenticated: false,
    isAdmin: false,
  }),
  useSubscriptionStore: () => ({
    fetchActiveSubscriptions,
    startPolling,
    clear: clearSubscriptions,
  }),
  useAnnouncementStore: () => ({
    fetchAnnouncements,
    reset: resetAnnouncements,
  }),
  useAdminComplianceStore: () => ({
    fetchStatus: fetchComplianceStatus,
    reset: resetCompliance,
    requireAcknowledgement,
  }),
  useAdminSettingsStore: () => ({
    customMenuItems: [],
  }),
}))

describe('App global API error handling', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    document.title = ''
  })

  it('将公共 HTTP 错误事件转成全局错误提示', () => {
    const wrapper = mount(App, {
      global: {
        plugins: [createI18n({
          legacy: false,
          locale: 'zh',
          messages: {
            zh: { common: { unknownError: '发生未知错误' } },
          },
        })],
      },
    })

    window.dispatchEvent(new CustomEvent('sub2api-api-error', {
      detail: { message: '保存失败' },
    }))

    expect(showError).toHaveBeenCalledWith('保存失败')

    wrapper.unmount()
  })

  it('卸载后移除公共 HTTP 错误事件监听', () => {
    const wrapper = mount(App, {
      global: {
        plugins: [createI18n({
          legacy: false,
          locale: 'zh',
          messages: {
            zh: { common: { unknownError: '发生未知错误' } },
          },
        })],
      },
    })

    wrapper.unmount()
    showError.mockClear()

    window.dispatchEvent(new CustomEvent('sub2api-api-error', {
      detail: { message: '不应提示' },
    }))

    expect(showError).not.toHaveBeenCalled()
  })
})
