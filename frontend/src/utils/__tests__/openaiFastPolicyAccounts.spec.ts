import { describe, expect, it } from 'vitest'

import {
  addOpenAIFastPolicyAccountID,
  normalizeOpenAIFastPolicyAccountAllowlist
} from '../openaiFastPolicyAccounts'

describe('openaiFastPolicyAccounts', () => {
  it('normalizes account allowlist IDs to positive unique integers', () => {
    expect(
      normalizeOpenAIFastPolicyAccountAllowlist([
        3,
        '3',
        0,
        -1,
        4.2,
        '5',
        'not-a-number',
        null,
        undefined
      ])
    ).toEqual([3, 5])
  })

  it('returns undefined when no valid account IDs remain', () => {
    expect(
      normalizeOpenAIFastPolicyAccountAllowlist([0, -1, 'bad', null])
    ).toBeUndefined()
  })

  it('adds a selected account ID without duplicating existing entries', () => {
    expect(addOpenAIFastPolicyAccountID([7, 12], 12)).toEqual([7, 12])
    expect(addOpenAIFastPolicyAccountID([7, 12], 15)).toEqual([7, 12, 15])
  })
})
