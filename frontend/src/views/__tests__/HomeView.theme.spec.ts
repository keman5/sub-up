import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount, RouterLinkStub } from '@vue/test-utils'
import { nextTick } from 'vue'
import HomeView from '../HomeView.vue'
import ThemeSwitcher from '@/components/common/ThemeSwitcher.vue'
import { initThemeClass, useTheme } from '@/composables/useTheme'

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Test', siteLogo: '', docUrl: '', publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: { isAuthenticated: false, isAdmin: false, user: null, checkAuth: vi.fn() },
}))
vi.mock('@/stores', () => ({ useAppStore: () => appStore, useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

enableAutoUnmount(afterEach)
let systemChanged: ((event: MediaQueryListEvent) => void) | undefined
const media = {
  matches: false,
  addEventListener: (_: string, listener: (event: MediaQueryListEvent) => void) => { systemChanged = listener },
}
const stubs = { RouterLink: RouterLinkStub, LocaleSwitcher: true, Icon: true }

describe('HomeView shared theme contract', () => {
  beforeEach(() => {
    localStorage.clear()
    media.matches = false
    vi.spyOn(window, 'matchMedia').mockReturnValue(media as unknown as MediaQueryList)
    initThemeClass()
  })
  afterEach(() => { vi.restoreAllMocks() })

  it.each([false, true])('shares all three preferences with the console switcher (compact=%s)', async (compact) => {
    appStore.cachedPublicSettings = { compact_home_enabled: compact }
    const shared = useTheme()
    const home = mount(HomeView, { global: { stubs } })
    const select = async (value: string) => {
      await home.get('[data-testid="theme-switcher-trigger"]').trigger('click')
      for (const option of ['system', 'light', 'dark']) {
        expect(home.find(`[data-testid="theme-option-${option}"]`).exists()).toBe(true)
      }
      await home.get(`[data-testid="theme-option-${value}"]`).trigger('click')
      expect(shared.preference.value).toBe(value)
      expect(localStorage.getItem('theme')).toBe(value)
    }

    await select('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    await select('light')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    systemChanged?.({ matches: true } as MediaQueryListEvent)
    await nextTick()
    expect(shared.effectiveTheme.value).toBe('light')
    await select('system')
    expect(shared.effectiveTheme.value).toBe('dark')
    systemChanged?.({ matches: false } as MediaQueryListEvent)
    await nextTick()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    // Simulate leaving home and mounting the same switcher used by AppHeader.
    home.unmount()
    const consoleSwitcher = mount(ThemeSwitcher, { global: { stubs: { Icon: true } } })
    expect(consoleSwitcher.get('[data-testid="theme-switcher-trigger"]').attributes('title')).toBe('common.theme.system')
    await consoleSwitcher.get('[data-testid="theme-switcher-trigger"]').trigger('click')
    await consoleSwitcher.get('[data-testid="theme-option-dark"]').trigger('click')
    expect(shared.preference.value).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })
})
