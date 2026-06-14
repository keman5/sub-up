import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpsHeadroomStatsCard from '../OpsHeadroomStatsCard.vue'

const mockGetHeadroomStats = vi.fn()

vi.mock('@/api/admin/ops', () => ({
  default: {
    getHeadroomStats: (...args: any[]) => mockGetHeadroomStats(...args),
  },
  opsAPI: {
    getHeadroomStats: (...args: any[]) => mockGetHeadroomStats(...args),
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, any>) => {
        if (key === 'admin.ops.headroomStats.lastFetched' && params) {
          return `fetched ${params.time}`
        }
        return key
      },
    }),
  }
})

const sampleStats = {
  mode: 'token',
  api_requests: 12,
  requests_total: 12,
  requests_failed: 1,
  requests_compressed: 9,
  input_tokens: 100_000,
  output_tokens: 12_000,
  tokens_saved: 34_567,
  proxy_compression_saved: 34_567,
  total_before_compression: 134_567,
  savings_percent: 25.69,
  average_compression_percent: 31.2,
  total_saved_usd: 0.1234,
  cost_savings_percent: 18.5,
  by_provider: {
    openai: 12,
  },
  by_model: {
    'gpt-5.3-codex-spark': 8,
    'gpt-5.3-codex': 4,
  },
  fetched_at: '2026-06-13T10:00:00Z',
}

describe('OpsHeadroomStatsCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads and renders Headroom token savings snapshot', async () => {
    mockGetHeadroomStats.mockResolvedValue(sampleStats)

    const wrapper = mount(OpsHeadroomStatsCard, {
      props: {
        refreshToken: 0,
      },
    })
    await flushPromises()

    expect(mockGetHeadroomStats).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('34.6K')
    expect(wrapper.text()).toContain('25.69%')
    expect(wrapper.text()).toContain('9')
    expect(wrapper.text()).toContain('gpt-5.3-codex-spark')
  })

  it('reloads when the dashboard refresh token changes', async () => {
    mockGetHeadroomStats.mockResolvedValue(sampleStats)

    const wrapper = mount(OpsHeadroomStatsCard, {
      props: {
        refreshToken: 0,
      },
    })
    await flushPromises()

    await wrapper.setProps({ refreshToken: 1 })
    await flushPromises()

    expect(mockGetHeadroomStats).toHaveBeenCalledTimes(2)
  })

  it('refreshes the Headroom snapshot from the card action', async () => {
    mockGetHeadroomStats
      .mockResolvedValueOnce(sampleStats)
      .mockResolvedValueOnce({
        ...sampleStats,
        tokens_saved: 45_678,
        requests_compressed: 10,
        fetched_at: '2026-06-13T10:01:00Z',
      })

    const wrapper = mount(OpsHeadroomStatsCard, {
      props: {
        refreshToken: 0,
      },
    })
    await flushPromises()

    await wrapper.get('[data-testid="headroom-stats-refresh"]').trigger('click')
    await flushPromises()

    expect(mockGetHeadroomStats).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('45.7K')
    expect(wrapper.text()).toContain('10')
  })

  it('shows a neutral disabled state when Headroom stats are disabled', async () => {
    mockGetHeadroomStats.mockRejectedValue({
      status: 503,
      message: 'Headroom stats disabled',
    })

    const wrapper = mount(OpsHeadroomStatsCard, {
      props: {
        refreshToken: 0,
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.ops.headroomStats.disabled')
    expect(wrapper.text()).not.toContain('Headroom stats disabled')
  })
})
