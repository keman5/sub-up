import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { adminSettingsState, routeQuery, replaceMock, opsMocks } = vi.hoisted(() => ({
  adminSettingsState: {
    opsMonitoringEnabled: true,
    opsQueryModeDefault: 'auto',
    fetch: vi.fn(),
  },
  routeQuery: { value: {} as Record<string, unknown> },
  replaceMock: vi.fn(),
  opsMocks: {
    getHostHealth: vi.fn(),
    getAdvancedSettings: vi.fn(),
    getMetricThresholds: vi.fn(),
    getDashboardSnapshotV2: vi.fn(),
    getThroughputTrend: vi.fn(),
    getLatencyHistogram: vi.fn(),
    getErrorDistribution: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
  }),
  useAdminSettingsStore: () => adminSettingsState,
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: routeQuery.value }),
  useRouter: () => ({ replace: replaceMock }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/i18n', () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
}))

vi.mock('@/api/admin/ops', () => ({
  default: opsMocks,
  opsAPI: opsMocks,
}))

describe('OpsDashboard host health visibility', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.unstubAllEnvs()
    adminSettingsState.opsMonitoringEnabled = true
    adminSettingsState.opsQueryModeDefault = 'auto'
    adminSettingsState.fetch.mockReset().mockResolvedValue(undefined)
    replaceMock.mockReset()
    Object.values(opsMocks).forEach((mock) => mock.mockReset())
    opsMocks.getAdvancedSettings.mockResolvedValue({
      display_alert_events: false,
      display_openai_token_stats: false,
      auto_refresh_enabled: false,
      auto_refresh_interval_seconds: 30,
    })
    opsMocks.getMetricThresholds.mockResolvedValue(null)
    opsMocks.getDashboardSnapshotV2.mockResolvedValue({
      overview: null,
      throughput_trend: { points: [], by_platform: [], top_groups: [] },
      error_trend: { points: [] },
    })
    opsMocks.getThroughputTrend.mockResolvedValue({ points: [], by_platform: [], top_groups: [] })
    opsMocks.getLatencyHistogram.mockResolvedValue({ buckets: [] })
    opsMocks.getErrorDistribution.mockResolvedValue({ items: [] })
    opsMocks.getHostHealth.mockResolvedValue({
      available: false,
      status: 'missing',
      stale: false,
      load_average: {},
      cpu: {},
      memory: {},
    })
  })

  it('does not mount the host CPU panel when the current environment hides it', async () => {
    vi.stubEnv('VITE_OPS_HOST_HEALTH_VISIBLE', 'false')
    const { default: OpsDashboard } = await import('../OpsDashboard.vue')

    const wrapper = mount(OpsDashboard, {
      global: {
        stubs: dashboardStubs(),
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="host-health-card"]').exists()).toBe(false)
    expect(opsMocks.getHostHealth).not.toHaveBeenCalled()
  })

  it('mounts the host CPU panel when the current environment exposes it', async () => {
    vi.stubEnv('VITE_OPS_HOST_HEALTH_VISIBLE', 'true')
    const { default: OpsDashboard } = await import('../OpsDashboard.vue')

    const wrapper = mount(OpsDashboard, {
      global: {
        stubs: dashboardStubs(),
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="host-health-card"]').exists()).toBe(true)
    expect(opsMocks.getHostHealth).toHaveBeenCalled()
  })
})

function dashboardStubs() {
  return {
    AppLayout: { template: '<div><slot /></div>' },
    OpsDashboardHeader: true,
    OpsDashboardSkeleton: true,
    OpsConcurrencyCard: true,
    OpsSwitchRateTrendChart: true,
    OpsThroughputTrendChart: true,
    OpsLatencyChart: true,
    OpsErrorDistributionChart: true,
    OpsErrorTrendChart: true,
    OpsOpenAITokenStatsCard: true,
    OpsHostHealthCard: {
      props: ['refreshToken'],
      template: '<section data-testid="host-health-card" />',
      mounted() {
        opsMocks.getHostHealth()
      },
    },
    OpsAlertEventsCard: true,
    OpsSystemLogTable: true,
    OpsSettingsDialog: true,
    BaseDialog: { template: '<div><slot /></div>' },
    OpsAlertRulesCard: true,
    OpsErrorDetailsModal: true,
    OpsErrorDetailModal: true,
    OpsRequestDetailsModal: true,
  }
}
