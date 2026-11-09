import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PublicSettings } from '@/types'
import { FeatureFlags, isFeatureFlagEnabled, resolveFeatureFlag } from '../featureFlags'

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: null as null | Record<string, unknown>,
  fetchPublicSettings: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

describe('feature flag refresh helpers', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockReset()
  })

  it('refreshes stale opt-in flags before resolving them', async () => {
    const { FeatureFlags, refreshAndResolveFeatureFlag } = await import('../featureFlags')
    appStore.cachedPublicSettings = { risk_control_enabled: false }
    appStore.fetchPublicSettings.mockImplementation(async () => {
      appStore.cachedPublicSettings = { risk_control_enabled: true }
      return appStore.cachedPublicSettings
    })

    await expect(refreshAndResolveFeatureFlag(FeatureFlags.riskControl)).resolves.toBe(true)
    expect(appStore.fetchPublicSettings).toHaveBeenCalledWith(true)
  })

  it('does not refresh when the current flag value already allows access', async () => {
    const { FeatureFlags, refreshAndResolveFeatureFlag } = await import('../featureFlags')
    appStore.cachedPublicSettings = { risk_control_enabled: true }

    await expect(refreshAndResolveFeatureFlag(FeatureFlags.riskControl)).resolves.toBe(true)
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })
})

describe('FeatureFlags.mcp', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = null
    delete (window as any).__APP_CONFIG__
  })

  it('stays hidden unless the deployment explicitly enables MCP', () => {
    expect(FeatureFlags.mcp.key).toBe('mcp_enabled')
    expect(FeatureFlags.mcp.mode).toBe('opt-in')
    expect(isFeatureFlagEnabled(FeatureFlags.mcp)).toBe(false)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.mcp)).toBe(false)
    expect(resolveFeatureFlag({ mcp_enabled: false } as PublicSettings, FeatureFlags.mcp)).toBe(false)
    expect(resolveFeatureFlag({ mcp_enabled: true } as PublicSettings, FeatureFlags.mcp)).toBe(true)
  })
})
