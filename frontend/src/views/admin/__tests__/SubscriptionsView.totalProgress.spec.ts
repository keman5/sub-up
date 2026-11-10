import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { describe, expect, it, vi } from 'vitest'
import SubscriptionsView from '../SubscriptionsView.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), progress: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: {
  subscriptions: { list: mocks.list, getProgress: mocks.progress },
  groups: { getAll: async () => [] },
  usage: { searchUsers: async () => [] }
} }))

describe('subscription total progress on the real page', () => {
  it('refreshes a total-only row and renders its latest usage instead of the list snapshot', async () => {
    setActivePinia(createPinia())
    let resolveProgress!: (value: unknown) => void
    mocks.list.mockResolvedValue({ items: [{
      id: 9, user_id: 21, status: 'active', total_usage_usd: 1,
      group: { total_limit_usd: 100 }
    }], total: 1, pages: 1 })
    mocks.progress.mockImplementation(() => new Promise(resolve => { resolveProgress = resolve }))
    const wrapper = mount(SubscriptionsView, { global: {
      plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: {} }, missing: (_locale, key) => key })],
      stubs: {
        AppLayout: { template: '<main><slot/></main>' },
        TablePageLayout: { template: '<div><slot name="table"/></div>' },
        DataTable: { props: ['data'], template: '<div><div v-for="row in data" :key="row.id" data-usage><slot name="cell-usage" :row="row"/></div></div>' },
        BaseDialog: true, ConfirmDialog: true, Pagination: true, Select: true, Icon: true,
        RouterLink: true, BulkSubscriptionActionDialog: true
      }
    } })
    try {
      await flushPromises()
      expect(mocks.progress).toHaveBeenCalledWith(9)
      expect(wrapper.get('[data-usage]').text()).toContain('$1.00')
      resolveProgress({ subscription_id: 9, daily: null, weekly: null, monthly: null,
        total: { used_usd: 37, percentage: 37 }, expires_at: null, days_remaining: null })
      await flushPromises()
      expect(wrapper.get('[data-usage]').text()).toContain('$37.00')
      expect(wrapper.get('[data-usage]').text()).not.toContain('$1.00')
      expect(wrapper.get('[data-usage] .transition-all').attributes('style')).toContain('width: 37%')
    } finally {
      wrapper.unmount()
    }
  })
})
