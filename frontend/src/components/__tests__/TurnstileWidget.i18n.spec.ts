import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import TurnstileWidget from '../TurnstileWidget.vue'
import en from '@/i18n/locales/en/common'
import zh from '@/i18n/locales/zh/common'

enableAutoUnmount(afterEach)
const scriptSelector = 'script[src*="challenges.cloudflare.com/turnstile"]'
afterEach(() => {
  delete window.turnstile
  delete window.onTurnstileLoad
  document.querySelectorAll(scriptSelector).forEach(script => script.remove())
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('Turnstile script failure localization', () => {
  it.each([
    ['en', 'Verification failed to load', 'Click to reload the page'],
    ['zh', '验证码加载失败', '点击刷新页面'],
  ])('renders the %s failure and reloads only on user action', async (locale, title, description) => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const reload = vi.fn()
    vi.stubGlobal('location', { ...window.location, reload })
    const wrapper = mount(TurnstileWidget, {
      props: { siteKey: 'site-key' },
      global: { plugins: [createI18n({ legacy: false, locale, fallbackLocale: 'en', messages: { en, zh } })] },
    })
    expect(wrapper.find('[role="status"]').exists()).toBe(true)
    document.querySelector(scriptSelector)!.dispatchEvent(new Event('error'))
    await flushPromises()
    expect(wrapper.find('[role="status"]').exists()).toBe(false)
    expect(wrapper.emitted('error')).toEqual([[]])
    const prompt = wrapper.get('[data-testid="turnstile-reload-prompt"]')
    expect(prompt.get('.turnstile-reload-title').text()).toBe(title)
    expect(prompt.get('.turnstile-reload-desc').text()).toBe(description)
    expect(reload).not.toHaveBeenCalled()
    await prompt.trigger('click')
    expect(reload).toHaveBeenCalledOnce()
  })
})
