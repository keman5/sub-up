import { defineComponent, onMounted, watch } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const DataTableStub = {
  props: ['data', 'loading'],
  template: `
    <div data-test="data-table">
      <div v-if="loading" data-test="loading">loading</div>
      <div v-else v-for="row in data" :key="row.id" data-test="row">
        <slot name="cell-usage" :row="row" />
      </div>
    </div>
  `
}

const AccountTableActionsStub = {
  emits: ['refresh'],
  template: '<button data-test="refresh-accounts" @click="$emit(\'refresh\')">refresh</button>'
}

const BulkEditAccountModalStub = {
  emits: ['updated'],
  template: '<button data-test="bulk-updated" @click="$emit(\'updated\')">updated</button>'
}

const usageCellMountedTokens: number[] = []
const usageCellRefreshTransitions: Array<[number | undefined, number | undefined]> = []

const AccountUsageCellStub = defineComponent({
  props: {
    account: { type: Object, required: true },
    manualRefreshToken: { type: Number, default: 0 }
  },
  setup(props) {
    onMounted(() => {
      usageCellMountedTokens.push(props.manualRefreshToken)
    })
    watch(
      () => props.manualRefreshToken,
      (nextToken, prevToken) => {
        usageCellRefreshTransitions.push([prevToken, nextToken])
      }
    )
    return {}
  },
  template: '<div data-test="usage-cell">{{ manualRefreshToken }}</div>'
})

const createAccount = (id: number, name: string) => ({
  id,
  name,
  platform: 'anthropic',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  created_at: '2026-07-07T00:00:00Z',
  updated_at: '2026-07-07T00:00:00Z'
})

const pageResponse = (items: Array<Record<string, unknown>>) => ({
  items,
  total: items.length,
  page: 1,
  page_size: 20,
  pages: 1
})

const mountView = () =>
  mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: AccountTableActionsStub,
        AccountTableFilters: { template: '<div></div>' },
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
        BulkEditAccountModal: BulkEditAccountModalStub,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: AccountUsageCellStub,
        Icon: true
      }
    }
  })

describe('admin AccountsView manual usage refresh', () => {
  beforeEach(() => {
    localStorage.clear()
    usageCellMountedTokens.length = 0
    usageCellRefreshTransitions.length = 0

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('refreshes remounted current-page usage cells after the table reload finishes', async () => {
    listAccounts
      .mockResolvedValueOnce(pageResponse([createAccount(1, 'before-refresh')]))
      .mockResolvedValueOnce(pageResponse([createAccount(1, 'after-refresh')]))

    const wrapper = mountView()
    await flushPromises()

    expect(usageCellMountedTokens).toEqual([0])

    await wrapper.get('[data-test="refresh-accounts"]').trigger('click')
    await flushPromises()

    expect(usageCellMountedTokens).toEqual([0, 0])
    expect(usageCellRefreshTransitions).toContainEqual([0, 1])

    wrapper.unmount()
  })

  it('refreshes current-page usage cells after account reload actions finish', async () => {
    listAccounts
      .mockResolvedValueOnce(pageResponse([createAccount(1, 'before-reload')]))
      .mockResolvedValueOnce(pageResponse([createAccount(1, 'after-reload')]))

    const wrapper = mountView()
    await flushPromises()

    expect(usageCellMountedTokens).toEqual([0])

    await wrapper.get('[data-test="bulk-updated"]').trigger('click')
    await flushPromises()

    expect(usageCellMountedTokens).toEqual([0, 0])
    expect(usageCellRefreshTransitions).toContainEqual([0, 1])

    wrapper.unmount()
  })
})
