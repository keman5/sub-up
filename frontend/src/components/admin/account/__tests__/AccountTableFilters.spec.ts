import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountTableFilters from '../AccountTableFilters.vue'

const messages: Record<string, string> = {
  'admin.accounts.searchAccounts': 'Search accounts',
  'common.noOptionsFound': 'No options found',
  'admin.accounts.allPlatforms': 'All Platforms',
  'admin.accounts.allTypes': 'All Types',
  'admin.accounts.oauthType': 'OAuth',
  'admin.accounts.setupToken': 'Setup Token',
  'admin.accounts.apiKey': 'API Key',
  'admin.accounts.allStatus': 'All Status',
  'admin.accounts.status.active': 'Active',
  'admin.accounts.status.inactive': 'Inactive',
  'admin.accounts.status.error': 'Error',
  'admin.accounts.status.rateLimited': 'Rate Limited',
  'admin.accounts.status.tempUnschedulable': 'Temp Unschedulable',
  'admin.accounts.status.unschedulable': 'Unschedulable',
  'admin.accounts.allPrivacyModes': 'All Privacy Modes',
  'admin.accounts.privacyUnset': 'Unset',
  'admin.accounts.allGroups': 'All Groups',
  'admin.accounts.ungroupedGroup': 'Ungrouped',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const mockAccountsList = vi.fn()

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: (...args: any[]) => mockAccountsList(...args),
    },
  },
}))

function mountFilters() {
  return mount(AccountTableFilters, {
    props: {
      searchQuery: '',
      filters: {
        platform: '',
        type: '',
        status: '',
        privacy_mode: '',
        group: '',
      },
      groups: [],
    },
    global: {
      stubs: {
        Select: true,
        Teleport: true,
      },
    },
  })
}

describe('AccountTableFilters', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockAccountsList.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads default account suggestions on focus when keyword is empty', async () => {
    mockAccountsList.mockResolvedValue({
      items: [{ id: 101, name: 'focus-account' }],
    })

    const wrapper = mountFilters()
    const input = wrapper.get('input[type="text"]')

    await input.trigger('focus')
    vi.advanceTimersByTime(300)
    await flushPromises()

    expect(mockAccountsList).toHaveBeenCalledWith(1, 20, { search: '' })
    expect(wrapper.text()).toContain('focus-account')
    expect(wrapper.emitted('change')).toBeFalsy()
  })

  it('closes the dropdown when focus leaves the component', async () => {
    mockAccountsList.mockResolvedValue({
      items: [{ id: 101, name: 'focus-account' }],
    })

    const wrapper = mountFilters()
    const input = wrapper.get('input[type="text"]')

    await input.trigger('focus')
    vi.advanceTimersByTime(300)
    await flushPromises()
    expect(wrapper.text()).toContain('focus-account')

    await input.trigger('blur')
    await flushPromises()

    expect(wrapper.text()).not.toContain('focus-account')
  })

  it('reuses loaded suggestions on refocus without requesting again', async () => {
    mockAccountsList.mockResolvedValue({
      items: [{ id: 101, name: 'focus-account' }],
    })

    const wrapper = mountFilters()
    const input = wrapper.get('input[type="text"]')

    await input.trigger('focus')
    vi.advanceTimersByTime(300)
    await flushPromises()

    await input.trigger('blur')
    await flushPromises()

    await input.trigger('focus')
    vi.advanceTimersByTime(300)
    await flushPromises()

    expect(mockAccountsList).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('focus-account')
  })

  it('keeps the same suggestion nodes when a repeated request returns the same accounts', async () => {
    mockAccountsList.mockResolvedValue({
      items: [{ id: 101, name: 'focus-account' }],
    })

    const wrapper = mountFilters()
    const input = wrapper.get('input[type="text"]')

    await input.trigger('focus')
    vi.advanceTimersByTime(300)
    await flushPromises()

    const beforeHtml = wrapper.findAll('[data-test="search-suggest-option"]').map((node) => node.element)

    await wrapper.findComponent({ name: 'SearchSuggestInput' }).vm.$emit('search', 'focus')
    vi.advanceTimersByTime(300)
    await flushPromises()

    const afterHtml = wrapper.findAll('[data-test="search-suggest-option"]').map((node) => node.element)

    expect(mockAccountsList).toHaveBeenCalledTimes(2)
    expect(afterHtml).toHaveLength(1)
    expect(afterHtml[0]).toBe(beforeHtml[0])
  })
})
