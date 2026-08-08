import { describe, expect, it } from 'vitest'

import { enqueueActiveUsageRefresh } from '../usageLoadQueue'

describe('enqueueActiveUsageRefresh', () => {
  it('runs explicit upstream refreshes one at a time', async () => {
    const calls: string[] = []
    let releaseFirst: (() => void) | undefined
    const firstGate = new Promise<void>((resolve) => {
      releaseFirst = resolve
    })

    const first = enqueueActiveUsageRefresh(async () => {
      calls.push('first:start')
      await firstGate
      calls.push('first:end')
      return 'first'
    })
    const second = enqueueActiveUsageRefresh(async () => {
      calls.push('second:start')
      return 'second'
    })

    await Promise.resolve()
    expect(calls).toEqual(['first:start'])

    releaseFirst?.()
    await expect(first).resolves.toBe('first')
    await expect(second).resolves.toBe('second')
    expect(calls).toEqual(['first:start', 'first:end', 'second:start'])
  })

  it('continues after a failed refresh', async () => {
    await expect(enqueueActiveUsageRefresh(async () => {
      throw new Error('upstream unavailable')
    })).rejects.toThrow('upstream unavailable')

    await expect(enqueueActiveUsageRefresh(async () => 'next account')).resolves.toBe('next account')
  })
})
