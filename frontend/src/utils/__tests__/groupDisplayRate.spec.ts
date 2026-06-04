import { describe, expect, it } from 'vitest'
import { getGroupDisplayRateMultiplier } from '../groupDisplayRate'

describe('getGroupDisplayRateMultiplier', () => {
  it('uses the configured user-facing display multiplier', () => {
    expect(getGroupDisplayRateMultiplier({ rate_multiplier: 2, display_rate_multiplier: 1 })).toBe(1)
    expect(getGroupDisplayRateMultiplier({ rate_multiplier: 2, display_rate_multiplier: 0 })).toBe(1)
    expect(getGroupDisplayRateMultiplier({ rate_multiplier: 2 })).toBe(1)
  })
})
