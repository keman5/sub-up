import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpsHostHealthCard from '../OpsHostHealthCard.vue'

const mockGetHostHealth = vi.fn()

vi.mock('@/api/admin/ops', () => ({
  default: {
    getHostHealth: (...args: any[]) => mockGetHostHealth(...args),
  },
  opsAPI: {
    getHostHealth: (...args: any[]) => mockGetHostHealth(...args),
  },
}))

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, any>) => {
        if (key === 'admin.ops.hostHealth.lastCollected' && params) {
          return `collected ${params.time}`
        }
        if (key === 'admin.ops.hostHealth.age' && params) {
          return `${params.seconds}s`
        }
        return key
      },
    }),
  }
})

const sampleHealth = {
  available: true,
  status: 'ok',
  collected_at: '2026-06-15T01:00:00Z',
  age_seconds: 12,
  stale: false,
  load_average: {
    one: 5.27,
    five: 5.63,
    fifteen: 4.18,
  },
  cpu: {
    usage_percent: 96.8,
    high: true,
  },
  memory: {
    available_mb: 1740,
    swap_used_mb: 177,
  },
  top_containers: [
    { name: 'sub2api-worker', cpu_percent: 163.5, memory: '936MiB / 1.172GiB', pids: 21 },
  ],
  top_processes: [
    { pid: 12345, command: 'python', cpu_percent: 160.0, rss_mb: 936 },
  ],
  diagnosis: 'CPU 压力主要来自 sub2api-worker',
}

describe('OpsHostHealthCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads and renders host CPU cause snapshot', async () => {
    mockGetHostHealth.mockResolvedValue(sampleHealth)

    const wrapper = mount(OpsHostHealthCard, {
      props: {
        refreshToken: 0,
      },
    })
    await flushPromises()

    expect(mockGetHostHealth).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('96.8%')
    expect(wrapper.text()).toContain('sub2api-worker')
    expect(wrapper.text()).toContain('python')
    expect(wrapper.text()).toContain('CPU 压力主要来自 sub2api-worker')
  })

  it('reloads when the dashboard refresh token changes', async () => {
    mockGetHostHealth.mockResolvedValue(sampleHealth)

    const wrapper = mount(OpsHostHealthCard, {
      props: {
        refreshToken: 0,
      },
    })
    await flushPromises()

    await wrapper.setProps({ refreshToken: 1 })
    await flushPromises()

    expect(mockGetHostHealth).toHaveBeenCalledTimes(2)
  })

  it('renders a CPU trend chart from successful refresh snapshots', async () => {
    mockGetHostHealth
      .mockResolvedValueOnce({
        ...sampleHealth,
        collected_at: '2026-06-15T01:00:00Z',
        load_average: { ...sampleHealth.load_average, one: 1.1 },
        cpu: { ...sampleHealth.cpu, usage_percent: 45.2 },
      })
      .mockResolvedValueOnce({
        ...sampleHealth,
        collected_at: '2026-06-15T01:01:00Z',
        load_average: { ...sampleHealth.load_average, one: 5.27 },
        cpu: { ...sampleHealth.cpu, usage_percent: 96.8 },
      })

    const wrapper = mount(OpsHostHealthCard, {
      props: {
        refreshToken: 0,
      },
    })
    await flushPromises()

    await wrapper.setProps({ refreshToken: 1 })
    await flushPromises()

    const chartData = JSON.parse(wrapper.find('.chart-data').text())
    expect(chartData.labels).toHaveLength(2)
    expect(chartData.datasets.map((dataset: any) => dataset.label)).toEqual([
      'admin.ops.hostHealth.cpuUsageTrend',
      'admin.ops.hostHealth.loadOneMinute',
    ])
    expect(chartData.datasets[0].data).toEqual([45.2, 96.8])
    expect(chartData.datasets[1].data).toEqual([1.1, 5.27])
  })

  it('shows unavailable state without surfacing an error toast', async () => {
    mockGetHostHealth.mockResolvedValue({
      available: false,
      status: 'missing',
      message: 'host health snapshot is not available',
      stale: false,
      load_average: {},
      cpu: {},
      memory: {},
    })

    const wrapper = mount(OpsHostHealthCard, {
      props: {
        refreshToken: 0,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.ops.hostHealth.unavailable')
    expect(wrapper.text()).not.toContain('host health snapshot is not available')
  })
})
