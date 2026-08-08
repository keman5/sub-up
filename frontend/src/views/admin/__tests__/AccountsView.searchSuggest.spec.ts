import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups,
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
}))

const accountTableFiltersEvents: {
  updateSearchQuery?: ((value: string) => void)
  change?: (() => void)
} = {}

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
    },
    proxies: {
      getAll: getAllProxies,
    },
    groups: {
      getAll: getAllGroups,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token',
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const AccountTableFiltersStub = {
  props: ['searchQuery', 'filters', 'groups'],
  emits: ['update:searchQuery', 'update:filters', 'change'],
  setup(_: unknown, { emit }: { emit: (event: string, ...args: any[]) => void }) {
    accountTableFiltersEvents.updateSearchQuery = (value: string) => emit('update:searchQuery', value)
    accountTableFiltersEvents.change = () => emit('change')
    return {}
  },
  template: '<div data-test="account-table-filters"></div>',
}

describe('admin AccountsView search suggest wiring', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    accountTableFiltersEvents.updateSearchQuery = undefined
    accountTableFiltersEvents.change = undefined

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0,
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null,
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  afterEach(async () => {
    await vi.runOnlyPendingTimersAsync()
    await flushPromises()
    vi.useRealTimers()
  })

  it('loads active accounts by default', async () => {
    mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
          },
          DataTable: { template: '<div></div>' },
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: AccountTableFiltersStub,
          AccountBulkActionsBar: true,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: true,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({ status: 'active' }),
      expect.any(Object),
    )
  })

  it('does not reload the table on update:searchQuery alone, but still reloads on change', async () => {
    mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
          },
          DataTable: { template: '<div></div>' },
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: AccountTableFiltersStub,
          AccountBulkActionsBar: true,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: true,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true,
        },
      },
    })

    await flushPromises()
    expect(listAccounts).toHaveBeenCalledTimes(1)

    accountTableFiltersEvents.updateSearchQuery?.('')
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledTimes(1)

    accountTableFiltersEvents.change?.()
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledTimes(2)
  })
})
