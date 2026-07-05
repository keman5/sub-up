import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SubscriptionsView from '../SubscriptionsView.vue'
import { useAppDialog } from '@/composables/useAppDialog'

const mocks = vi.hoisted(() => ({
  assign: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    subscriptions: {
      list: vi.fn().mockResolvedValue({ items: [], total: 0, pages: 0 }),
      assign: mocks.assign,
      bulkAssign: vi.fn(),
      getProgress: vi.fn()
    },
    groups: {
      getAll: vi.fn().mockResolvedValue([
        {
          id: 7,
          name: '接力套餐',
          description: '',
          platform: 'openai',
          subscription_type: 'subscription',
          status: 'active',
          rate_multiplier: 1
        }
      ])
    },
    users: {
      list: vi.fn().mockResolvedValue({ items: [], total: 0, pages: 0 })
    },
    usage: {
      searchUsers: vi.fn().mockResolvedValue([])
    }
  }
}))

describe('SubscriptionsView duplicate assignment confirmation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mocks.assign.mockReset()
  })

  it('retries duplicate confirmation with the original group id even if the form is reset', async () => {
    const i18n = createI18n({
      legacy: false,
      locale: 'zh',
      messages: {
        zh: {}
      },
      missing: (_locale, key) => key
    })
    const wrapper = mount(SubscriptionsView, {
      global: {
        plugins: [i18n],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot /><slot name="table" /><slot name="pagination" /></div>' },
          DataTable: { template: '<div><slot name="empty" /></div>' },
          Pagination: true,
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          ConfirmDialog: true,
          EmptyState: true,
          Select: true,
          SearchSuggestInput: true,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
          RouterLink: true
        }
      }
    })
    await flushPromises()

    const vm = wrapper.vm as any
    vm.selectedAssignUsers = [{ id: 42, email: 'user@example.com', deleted: false }]
    vm.assignForm.group_id = 7
    vm.assignForm.validity_days = 30
    mocks.assign
      .mockRejectedValueOnce({
        reason: 'SUBSCRIPTION_DUPLICATE_CONFIRMATION_REQUIRED',
        response: { data: { reason: 'SUBSCRIPTION_DUPLICATE_CONFIRMATION_REQUIRED' } }
      })
      .mockResolvedValueOnce({ id: 99 })

    const submitPromise = vm.handleAssignSubscription()
    await flushPromises()

    vm.assignForm.group_id = null
    vm.assignForm.validity_days = 1
    useAppDialog().settleCurrent(true)
    await submitPromise
    await flushPromises()

    expect(mocks.assign).toHaveBeenNthCalledWith(
      2,
      {
        user_id: 42,
        group_id: 7,
        validity_days: 30,
        confirm_duplicate: true
      },
      { skipGlobalErrorToast: true }
    )
  })
})
