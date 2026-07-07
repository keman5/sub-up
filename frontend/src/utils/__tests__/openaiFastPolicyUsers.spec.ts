import { describe, expect, it } from 'vitest'

import {
  addOpenAIFastPolicyUserID,
  normalizeOpenAIFastPolicyUserAllowlist
} from '../openaiFastPolicyUsers'

describe('openaiFastPolicyUsers', () => {
  it('normalizes user allowlist IDs to positive unique integers', () => {
    expect(
      normalizeOpenAIFastPolicyUserAllowlist([
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

  it('returns undefined when no valid user IDs remain', () => {
    expect(
      normalizeOpenAIFastPolicyUserAllowlist([0, -1, 'bad', null])
    ).toBeUndefined()
  })

  it('adds a selected user ID without duplicating existing entries', () => {
    expect(addOpenAIFastPolicyUserID([7, 12], 12)).toEqual([7, 12])
    expect(addOpenAIFastPolicyUserID([7, 12], 15)).toEqual([7, 12, 15])
  })
})
