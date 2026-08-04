import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AccountActionMenu from '../AccountActionMenu.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'test-account',
    platform: 'openai',
    type: 'oauth',
    proxy_id: null,
    concurrency: 3,
    priority: 50,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

const position = { top: 100, left: 100 }

// AccountActionMenu uses <Teleport to="body">; content is rendered in document.body, not in wrapper.
const getBodyText = () => document.body.textContent ?? ''
const getBodyButtons = () => Array.from(document.body.querySelectorAll('button'))

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('AccountActionMenu — spark shadow 按钮可见性', () => {

  it('按真实高度夹紧到短视口并允许滚动', async () => {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function () {
      if ((this as HTMLElement).classList.contains('action-menu-content')) {
        return {
          top: 300,
          right: 308,
          bottom: 1000,
          left: 100,
          width: 208,
          height: 700,
          x: 100,
          y: 300,
          toJSON: () => ({})
        }
      }
      return {
        top: 0,
        right: 0,
        bottom: 0,
        left: 0,
        width: 0,
        height: 0,
        x: 0,
        y: 0,
        toJSON: () => ({})
      }
    })
    vi.stubGlobal('innerWidth', 390)
    vi.stubGlobal('innerHeight', 360)

    const wrapper = mount(AccountActionMenu, {
      props: {
        show: true,
        account: makeAccount({ platform: 'openai', type: 'oauth' }),
        position: { top: 300, left: 100 }
      },
      attachTo: document.body
    })

    await flushPromises()
    const menu = document.body.querySelector<HTMLElement>('.action-menu-content')
    expect(menu?.style.top).toBe('8px')
    expect(menu?.style.maxHeight).toBe('344px')
    expect(menu?.style.overflowY).toBe('auto')
    wrapper.unmount()
  })

  it('普通账号显示「复制账号」按钮', () => {
    const account = makeAccount({ platform: 'anthropic', type: 'apikey', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).toContain('admin.accounts.duplicateAccount')
    wrapper.unmount()
  })

  it('影子账号隐藏「复制账号」按钮', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: 42 })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).not.toContain('admin.accounts.duplicateAccount')
    wrapper.unmount()
  })

  it.each(['oauth', 'setup-token'] as const)('%s 账号隐藏「复制账号」按钮，避免共享可轮换令牌', (type) => {
    const account = makeAccount({ platform: 'openai', type, parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).not.toContain('admin.accounts.duplicateAccount')
    wrapper.unmount()
  })

  it('点击「复制账号」触发 duplicate 事件并携带 account', async () => {
    const account = makeAccount({ platform: 'anthropic', type: 'apikey', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })

    const duplicateBtn = getBodyButtons().find(b => b.textContent?.includes('admin.accounts.duplicateAccount'))
    expect(duplicateBtn).toBeDefined()

    duplicateBtn!.click()
    await wrapper.vm.$nextTick()

    const emitted = wrapper.emitted('duplicate')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toMatchObject({ id: account.id, name: account.name })
    wrapper.unmount()
  })

  it('OpenAI OAuth 母账号（无 parent_account_id）显示「创建 spark 影子」按钮', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).toContain('admin.accounts.createSparkShadow')
    wrapper.unmount()
  })

  it('影子账号（parent_account_id 非 null）隐藏「创建 spark 影子」按钮', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: 42 })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).not.toContain('admin.accounts.createSparkShadow')
    wrapper.unmount()
  })

  it('非 OpenAI 账号隐藏「创建 spark 影子」按钮', () => {
    const account = makeAccount({ platform: 'antigravity', type: 'oauth', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).not.toContain('admin.accounts.createSparkShadow')
    wrapper.unmount()
  })

  it('影子账号隐藏凭据/隐私类操作(重授权/刷新token/隐私)— 外审 G4', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: 42 })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    const body = getBodyText()
    expect(body).not.toContain('admin.accounts.reAuthorize')
    expect(body).not.toContain('admin.accounts.refreshToken')
    expect(body).not.toContain('admin.accounts.setPrivacy')
    wrapper.unmount()
  })

  it('普通 OpenAI OAuth 母账号仍显示凭据/隐私类操作', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    const body = getBodyText()
    expect(body).toContain('admin.accounts.reAuthorize')
    expect(body).toContain('admin.accounts.setPrivacy')
    wrapper.unmount()
  })

  it('点击按钮触发 create-spark-shadow 事件并携带 account', async () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })

    // Content is teleported to body — find button by text there
    const sparkBtn = getBodyButtons().find(b => b.textContent?.includes('admin.accounts.createSparkShadow'))
    expect(sparkBtn).toBeDefined()

    sparkBtn!.click()
    await wrapper.vm.$nextTick()

    const emitted = wrapper.emitted('create-spark-shadow')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toMatchObject({ id: account.id, platform: 'openai' })

    wrapper.unmount()
  })
})
