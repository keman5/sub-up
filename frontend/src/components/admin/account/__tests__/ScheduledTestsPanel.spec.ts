import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const mocks = vi.hoisted(() => ({
  listByAccount: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    scheduledTests: {
      listByAccount: mocks.listByAccount,
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      listResults: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: mocks.showError,
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        const messages: Record<string, string> = {
          'admin.scheduledTests.loadPlansFailed': '加载定时测试计划失败',
          'admin.scheduledTests.title': '定时测试',
          'admin.scheduledTests.addPlan': '添加计划',
          'admin.scheduledTests.deletePlan': '删除计划',
          'admin.scheduledTests.confirmDelete': '确认删除',
          'common.delete': '删除',
          'common.cancel': '取消',
        }
        return messages[key] ?? key
      },
    }),
  }
})

import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'

describe('ScheduledTestsPanel', () => {
  it('uses localized fallback when loading plans fails without error message', async () => {
    mocks.listByAccount.mockRejectedValueOnce({})

    const wrapper = mount(ScheduledTestsPanel, {
      props: {
        show: false,
        accountId: 1,
        modelOptions: [],
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /></div>',
            props: ['show', 'title'],
          },
          ConfirmDialog: true,
          HelpTooltip: true,
          Select: true,
          Input: true,
          Toggle: true,
          Icon: true,
        },
      },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(mocks.showError).toHaveBeenCalledWith('加载定时测试计划失败')
  })
})
