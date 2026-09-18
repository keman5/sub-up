import { beforeEach, describe, expect, it, vi } from 'vitest'

const get = vi.hoisted(() => vi.fn())

vi.mock('@/api/client', () => ({
  apiClient: { get }
}))

import { getPublicSettings } from '@/api/auth'

describe('public settings API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('bypasses the Pages public-settings cache when loading runtime site configuration', async () => {
    get.mockResolvedValue({ data: { site_name: 'Live Site' } })

    await expect(getPublicSettings()).resolves.toEqual({ site_name: 'Live Site' })
    expect(get).toHaveBeenCalledWith('/settings/public', {
      headers: { 'Cache-Control': 'no-cache' }
    })
  })
})
