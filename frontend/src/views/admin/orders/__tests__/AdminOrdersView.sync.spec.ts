import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AdminOrdersView from '../AdminOrdersView.vue'

const mocks = vi.hoisted(() => ({
  auth: { user: { email: 'macseek@upit.top' } },
  getOrders: vi.fn(),
  getSubscriptionSyncRequests: vi.fn(),
  requestSubscriptionSync: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: mocks, default: mocks }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

function render() {
  return mount(AdminOrdersView, {
    global: { stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      OrderTable: { props: ['orders'], template: '<div><slot v-for="row in orders" name="actions" :row="row" /></div>' },
      Icon: true, Select: true, Pagination: true, BaseDialog: true, AdminRefundDialog: true,
    } },
  })
}

describe('order subscription sync controls', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.auth.user.email = 'macseek@upit.top'
    mocks.getOrders.mockResolvedValue({ data: { items: [
      { id: 7, user_id: 21, order_type: 'subscription', status: 'COMPLETED' },
      { id: 8, user_id: 22, order_type: 'balance', status: 'COMPLETED' },
    ], total: 0 } })
    mocks.getSubscriptionSyncRequests.mockResolvedValue({ data: [] })
    mocks.requestSubscriptionSync.mockImplementation((order_id: number, enabled: boolean) => Promise.resolve({ data: { user_id: 21, order_id, enabled } }))
  })

  it('keeps subscription sync controls out of order management', async () => {
    const wrapper = render()
    await flushPromises()
    const sync = wrapper.findAll('button').filter(button => button.text() === 'payment.admin.syncToPrimary')
    expect(sync).toHaveLength(0)
    expect(wrapper.text()).not.toContain('payment.admin.cancelSubscriptionSync')
    expect(mocks.getSubscriptionSyncRequests).not.toHaveBeenCalled()
  })

  it('hides controls and does not fetch sync state for other administrators', async () => {
    mocks.auth.user.email = 'other@upit.top'
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).not.toContain('payment.admin.syncToPrimary')
    expect(wrapper.text()).not.toContain('payment.admin.cancelSubscriptionSync')
    expect(mocks.getSubscriptionSyncRequests).not.toHaveBeenCalled()
  })
})
