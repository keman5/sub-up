import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import TroubleshootingAssistant from '@/components/common/TroubleshootingAssistant.vue'
import { troubleshootingAPI } from '@/api/troubleshooting'

vi.mock('@/api/troubleshooting', () => ({
  troubleshootingAPI: {
    analyze: vi.fn(),
    notifyAdmin: vi.fn(),
  },
}))

describe('TroubleshootingAssistant', () => {
  afterEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    document.body.innerHTML = ''
  })

  it('submits the pasted failure report and renders the diagnosis', async () => {
    vi.mocked(troubleshootingAPI.analyze).mockResolvedValue({
      answer: '可能原因\n上游账号池暂时不可用。\n\n是否需要联系管理员\n需要。',
      source: 'ai',
      needs_admin: true,
      ai_attempted: true,
      ai_available: true,
      ai_attempts: 1,
      limit: {
        short_window_remaining: 5,
        daily_remaining: 19,
      },
    })

    const wrapper = mount(TroubleshootingAssistant, { attachTo: document.body })

    await wrapper.get('button[aria-label="打开故障排查助手"]').trigger('click')
    await wrapper.get('textarea').setValue('POST /v1/responses 返回 503 Service Unavailable')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(troubleshootingAPI.analyze).toHaveBeenCalledWith('POST /v1/responses 返回 503 Service Unavailable')
    expect(wrapper.text()).toContain('可能原因')
    expect(wrapper.text()).toContain('上游账号池暂时不可用')
    expect(wrapper.text()).toContain('剩余 5/5分钟 · 19/今日')

    wrapper.unmount()
  })

  it('shows an assistant-style waiting message while analyzing', async () => {
    let resolveAnalyze: ((value: Awaited<ReturnType<typeof troubleshootingAPI.analyze>>) => void) | undefined
    vi.mocked(troubleshootingAPI.analyze).mockReturnValue(new Promise((resolve) => {
      resolveAnalyze = resolve
    }))

    const wrapper = mount(TroubleshootingAssistant, { attachTo: document.body })

    await wrapper.get('button[aria-label="打开故障排查助手"]').trigger('click')
    await wrapper.get('textarea').setValue('POST /v1/responses 返回 503 Service Unavailable')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.text()).toContain('正在分析错误原因')
    expect(wrapper.find('.assistant-message-loading').exists()).toBe(true)

    resolveAnalyze?.({
      answer: '可能原因\n上游账号池暂时不可用。',
      source: 'rules',
      needs_admin: true,
      ai_attempted: true,
      ai_available: false,
      ai_attempts: 1,
      limit: null,
    })
    await flushPromises()

    expect(wrapper.text()).not.toContain('正在分析错误原因')
    expect(wrapper.text()).toContain('上游账号池暂时不可用')

    wrapper.unmount()
  })

  it('persists dragged position', async () => {
    const wrapper = mount(TroubleshootingAssistant, { attachTo: document.body })

    await wrapper.get('button[aria-label="打开故障排查助手"]').trigger('click')
    await wrapper.get('.assistant-header').trigger('mousedown', { clientX: 100, clientY: 100 })
    window.dispatchEvent(new MouseEvent('mousemove', { clientX: 60, clientY: 70 }))
    window.dispatchEvent(new MouseEvent('mouseup'))

    const raw = localStorage.getItem('sub2api.troubleshootingAssistant.position')
    expect(raw).toBeTruthy()
    const saved = JSON.parse(raw || '{}') as { x: number; y: number }
    expect(saved.x).toBeGreaterThan(24)
    expect(saved.y).toBeGreaterThan(24)

    wrapper.unmount()
  })

  it('drags the closed floating button without opening it', async () => {
    const wrapper = mount(TroubleshootingAssistant, { attachTo: document.body })

    const button = wrapper.get('button[aria-label="打开故障排查助手"]')
    await button.trigger('mousedown', { clientX: 100, clientY: 100 })
    window.dispatchEvent(new MouseEvent('mousemove', { clientX: 50, clientY: 60 }))
    window.dispatchEvent(new MouseEvent('mouseup'))
    await button.trigger('click')

    const raw = localStorage.getItem('sub2api.troubleshootingAssistant.position')
    expect(raw).toBeTruthy()
    const saved = JSON.parse(raw || '{}') as { x: number; y: number }
    expect(saved.x).toBeGreaterThan(24)
    expect(saved.y).toBeGreaterThan(24)
    expect(wrapper.find('.assistant-panel').exists()).toBe(false)

    wrapper.unmount()
  })

  it('notifies admin for diagnoses that need admin handling', async () => {
    vi.mocked(troubleshootingAPI.analyze).mockResolvedValue({
      answer: '已确认账号池无可用账号，需要管理员处理。',
      source: 'rules',
      needs_admin: true,
      ai_attempted: false,
      ai_available: false,
      ai_attempts: 0,
      limit: null,
    })
    vi.mocked(troubleshootingAPI.notifyAdmin).mockResolvedValue({
      message: '已通知管理员，请等待 5 分钟后重试。',
    })

    const wrapper = mount(TroubleshootingAssistant, { attachTo: document.body })

    await wrapper.get('button[aria-label="打开故障排查助手"]').trigger('click')
    await wrapper.get('textarea').setValue('POST /v1/responses 返回 503 Service Unavailable')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    const notifyButton = wrapper.get('button[data-testid="troubleshooting-notify-admin"]')
    await notifyButton.trigger('click')
    await flushPromises()

    expect(troubleshootingAPI.notifyAdmin).toHaveBeenCalledWith({
      message: 'POST /v1/responses 返回 503 Service Unavailable',
      diagnosis: '已确认账号池无可用账号，需要管理员处理。',
    })
    expect(wrapper.text()).toContain('已通知管理员，请等待 5 分钟后重试。')

    wrapper.unmount()
  })
})
