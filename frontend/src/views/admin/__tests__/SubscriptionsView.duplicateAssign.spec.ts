import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent } from 'vue'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AppDialogHost from '@/components/common/AppDialogHost.vue'
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
    const dialog = useAppDialog()
    while (dialog.currentDialog.value) {
      dialog.settleCurrent(false)
    }
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

  it('shows duplicate confirmation above assign modal and clicking confirm retries without errors', async () => {
    const i18n = createI18n({
      legacy: false,
      locale: 'zh',
      messages: {
        zh: {
          common: {
            cancel: '取消',
            confirm: '确认'
          },
          admin: {
            subscriptions: {
              assignSubscription: '分配订阅',
              duplicateConfirmTitle: '覆盖已有订阅',
              duplicateConfirmMessage: '用户 {user} 已存在「{group}」订阅，是否继续？',
              duplicateConfirmAction: '确认覆盖',
              subscriptionAssigned: '分配成功',
              failedToAssign: '分配订阅失败'
            }
          }
        }
      },
      missing: (_locale, key) => key
    })

    const Host = defineComponent({
      components: { AppDialogHost, SubscriptionsView },
      template: '<div><SubscriptionsView /><AppDialogHost /></div>'
    })

    const wrapper = mount(Host, {
      global: {
        plugins: [i18n],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot /><slot name="table" /><slot name="pagination" /></div>' },
          DataTable: { template: '<div><slot name="empty" /></div>' },
          Pagination: true,
          BaseDialog: {
            props: ['show', 'title', 'width', 'closeOnClickOutside', 'zIndex'],
            template:
              '<div v-if="show" data-test="base-dialog" :data-title="title" :data-z-index="zIndex"><slot /><slot name="footer" /></div>'
          },
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

    const view = wrapper.findComponent(SubscriptionsView)
    const vm = view.vm as any
    vm.showAssignModal = true
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

    const dialogs = wrapper.findAll('[data-test="base-dialog"]')
    expect(dialogs.at(-1)?.attributes('data-title')).toBe('admin.subscriptions.duplicateConfirmTitle')
    expect(dialogs.at(-1)?.attributes('data-z-index')).toBe('100')

    await dialogs.at(-1)?.findAll('button').at(-1)?.trigger('click')
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
