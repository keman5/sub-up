import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SubscriptionsView from '../SubscriptionsView.vue'

const mocks = vi.hoisted(() => ({
  auth: { user: { email: 'macseek@upit.top' } },
  getSubscriptionSyncRequests: vi.fn(), requestSubscriptionSyncBySubscription: vi.fn(),
  list: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: mocks }))
vi.mock('@/api/admin', () => ({ adminAPI: {
  subscriptions: { list: mocks.list }, groups: { getAll: async () => [] },
  usage: { searchUsers: async () => [] },
} }))
vi.mock('vue-i18n', async (original) => ({
  ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }),
}))
function render() {
  return mount(SubscriptionsView, { global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    TablePageLayout: { template: '<div><slot /><slot name="table" /></div>' },
    DataTable: { props: ['data'], template: '<div><slot v-for="row in data" name="cell-actions" :row="row" /></div>' },
    BaseDialog: true, ConfirmDialog: true, BulkSubscriptionActionDialog: true,
    Pagination: true, Select: true, Icon: true, RouterLink: true,
  } } })
}
describe('subscription sync controls', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.auth.user.email = 'macseek@upit.top'
    mocks.list.mockResolvedValue({ items: [{ id: 9, user_id: 21, status: 'active' }], total: 1, pages: 1 })
    mocks.getSubscriptionSyncRequests.mockResolvedValue({ data: [] })
    mocks.requestSubscriptionSyncBySubscription.mockImplementation((_id, enabled) => {
      const state = { user_id: 21, enabled, synced: enabled, executed: true, updated_at: '2026-10-11T00:00:00Z' }
      mocks.getSubscriptionSyncRequests.mockResolvedValue({ data: [state] })
      return Promise.resolve({ data: state })
    })
  })
  it('submits both operations using the subscription ID', async () => {
    const wrapper = render()
    await flushPromises()
    for (const [label, enabled] of [['syncToPrimary', true], ['cancelSubscriptionSync', false]] as const) {
      await wrapper.findAll('button').find(button => button.text() === `payment.admin.${label}`)!.trigger('click')
      await flushPromises()
      expect(mocks.requestSubscriptionSyncBySubscription).toHaveBeenLastCalledWith(9, enabled)
    }
    expect(wrapper.text()).toContain('payment.admin.syncCancelled')
    expect(wrapper.text()).not.toContain('payment.admin.cancelSubscriptionSync')
    wrapper.unmount()
  })
  it('does not show cancellation for users without an executed sync result', async () => {
    mocks.getSubscriptionSyncRequests.mockResolvedValue({ data: [{ user_id: 21, enabled: true, synced: false, executed: false }] })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).not.toContain('payment.admin.cancelSubscriptionSync')
    expect(wrapper.text()).not.toContain('payment.admin.syncCompleted')
    wrapper.unmount()
  })
  it('hides controls and does not read state for other administrators', async () => {
    mocks.auth.user.email = 'other@upit.top'
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).not.toContain('payment.admin.syncToPrimary')
    expect(mocks.getSubscriptionSyncRequests).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('waits for the matching execution receipt before reporting cancellation', async () => {
    vi.useFakeTimers()
    const previous = { user_id: 21, enabled: true, synced: true, executed: true, updated_at: '2026-10-11T00:00:00Z' }
    const cancelled = { ...previous, enabled: false, synced: false, updated_at: '2026-10-11T00:00:01Z' }
    mocks.getSubscriptionSyncRequests.mockResolvedValue({ data: [previous] })
    mocks.requestSubscriptionSyncBySubscription.mockResolvedValue({ data: cancelled })
    const wrapper = render()
    try {
      await flushPromises()
      await wrapper.findAll('button').find(button => button.text() === 'payment.admin.cancelSubscriptionSync')!.trigger('click')
      await flushPromises()
      expect(mocks.showSuccess).not.toHaveBeenCalled()
      expect(wrapper.text()).toContain('payment.admin.cancelSubscriptionSync')
      mocks.getSubscriptionSyncRequests.mockResolvedValue({ data: [cancelled] })
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      expect(mocks.showSuccess).toHaveBeenCalledWith('payment.admin.syncCancelled')
      expect(wrapper.text()).not.toContain('payment.admin.cancelSubscriptionSync')
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })
})
