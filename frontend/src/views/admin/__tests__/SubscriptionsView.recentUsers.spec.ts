import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import SubscriptionsView from '../SubscriptionsView.vue'

const mocks = vi.hoisted(() => ({ users: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: {
  subscriptions: { list: async () => ({ items: [], total: 0, pages: 0 }) },
  groups: { getAll: async () => [] },
  usage: { searchUsers: async () => [] },
  users: { list: mocks.users }
} }))

const user = (id: number, created_at: string, notes = '') => ({
  id, email: `user${id}@example.com`, username: `name${id}`, notes, created_at
})
let wrapper: VueWrapper
const open = async () => {
  wrapper = mount(SubscriptionsView, { global: {
    plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: {} }, missing: (_locale, key) => key })],
    stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/></div>' },
      DataTable: { template: '<div><slot name="empty"/></div>' },
      EmptyState: { emits: ['action'], template: '<button data-empty-assign @click="$emit(\'action\')">assign empty</button>' },
      BaseDialog: { props: ['show'], template: '<section v-if="show"><slot/><slot name="footer"/></section>' },
      ConfirmDialog: true, Pagination: true, Select: true, Icon: true, RouterLink: true,
      SearchSuggestInput: true, BulkSubscriptionActionDialog: true
    }
  } })
  await flushPromises()
  await wrapper.findAll('button').find(button => button.text() === 'admin.subscriptions.assignSubscription')!.trigger('click')
  await flushPromises()
}

describe('subscription assignment recent users on the real page', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-10T12:00:00Z'))
    mocks.users.mockReset()
  })
  afterEach(() => { wrapper?.unmount(); vi.useRealTimers() })

  it('loads recent users across API pages, keeps the exact 48h boundary, and displays notes', async () => {
    mocks.users
      .mockResolvedValueOnce({ items: [user(1, '2026-11-10T11:00:00Z', 'recent note')], pages: 3 })
      .mockResolvedValueOnce({ items: [user(2, '2026-11-08T12:00:00Z'), user(3, '2026-11-08T11:59:59Z')], pages: 3 })
    await open()
    const picker = wrapper.get('[data-assign-user-search]')
    expect(picker.text()).toContain('user1@example.com')
    expect(picker.text()).toContain('recent note')
    expect(picker.text()).toContain('user2@example.com')
    expect(picker.text()).not.toContain('user3@example.com')
    expect(mocks.users).toHaveBeenCalledTimes(2)
    expect(mocks.users).toHaveBeenNthCalledWith(2, 2, 100, { sort_by: 'created_at', sort_order: 'desc' })
  })

  it('searches old users without a time filter and restores recent users when selection is cleared', async () => {
    mocks.users.mockImplementation((_page, _size, filters) => Promise.resolve({
      items: filters.search ? [user(9, '2020-01-01T00:00:00Z', 'old matching note')] : [user(1, '2026-11-10T11:00:00Z')], pages: 1
    }))
    await open()
    const picker = wrapper.get('[data-assign-user-search]')
    await picker.get('input').setValue('old matching note')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(picker.text()).toContain('user9@example.com')
    expect(mocks.users).toHaveBeenLastCalledWith(1, 30, { search: 'old matching note', sort_by: 'email', sort_order: 'asc' })
    await picker.findAll('button').find(button => button.text().includes('user9@example.com'))!.trigger('click')
    await picker.get('button').trigger('click')
    await flushPromises()
    expect(picker.text()).toContain('user1@example.com')
    expect(picker.text()).not.toContain('user9@example.com')
  })

  it('loads the same recent picker from the empty-table assignment action', async () => {
    mocks.users.mockResolvedValue({ items: [user(1, '2026-11-10T11:00:00Z')], pages: 1 })
    await open()
    // Close through the actual dialog footer, then use the independent empty-state entry.
    await wrapper.findAll('button').find(button => button.text() === 'common.cancel')!.trigger('click')
    mocks.users.mockClear()
    await wrapper.get('[data-empty-assign]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-assign-user-search]').text()).toContain('user1@example.com')
    expect(mocks.users).toHaveBeenCalledTimes(1)
  })

  it('cancels a pending debounced search when the assignment dialog closes', async () => {
    mocks.users.mockResolvedValue({ items: [user(1, '2026-11-10T11:00:00Z')], pages: 1 })
    await open()
    await wrapper.get('[data-assign-user-search] input').setValue('pending search')
    await wrapper.findAll('button').find(button => button.text() === 'common.cancel')!.trigger('click')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(mocks.users).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-assign-user-search]').exists()).toBe(false)
  })

  it('discards an already requested response after the assignment dialog closes', async () => {
    let resolveSearch!: (result: { items: ReturnType<typeof user>[]; pages: number }) => void
    mocks.users.mockResolvedValueOnce({ items: [user(1, '2026-11-10T11:00:00Z')], pages: 1 })
      .mockImplementationOnce(() => new Promise(resolve => { resolveSearch = resolve }))
      .mockResolvedValue({ items: [user(2, '2026-11-10T11:00:00Z')], pages: 1 })
    await open()
    await wrapper.get('[data-assign-user-search] input').setValue('old search')
    await vi.advanceTimersByTimeAsync(300)
    expect(mocks.users).toHaveBeenCalledTimes(2)
    await wrapper.findAll('button').find(button => button.text() === 'common.cancel')!.trigger('click')
    resolveSearch({ items: [user(9, '2020-01-01T00:00:00Z')], pages: 1 })
    await flushPromises()
    expect((wrapper.vm as unknown as { userSearchResults: unknown[] }).userSearchResults).toEqual([])
    await wrapper.get('[data-empty-assign]').trigger('click')
    await flushPromises()
    const picker = wrapper.get('[data-assign-user-search]')
    expect(picker.text()).toContain('user2@example.com')
    expect(picker.text()).not.toContain('user9@example.com')
  })
})
