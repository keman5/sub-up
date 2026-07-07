import { describe, expect, it } from 'vitest'

import {
  addOpenAIFastPolicyOpenAIAccountID,
  normalizeOpenAIFastPolicyOpenAIAccountAllowlist
} from '../openaiFastPolicyAccounts'

describe('openaiFastPolicyAccounts', () => {
  it('normalizes OpenAI account allowlist IDs to positive unique integers', () => {
    expect(
      normalizeOpenAIFastPolicyOpenAIAccountAllowlist([
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

  it('returns undefined when no valid OpenAI account IDs remain', () => {
    expect(
      normalizeOpenAIFastPolicyOpenAIAccountAllowlist([0, -1, 'bad', null])
    ).toBeUndefined()
  })

  it('adds a selected OpenAI account ID without duplicating existing entries', () => {
    expect(addOpenAIFastPolicyOpenAIAccountID([7, 12], 12)).toEqual([7, 12])
    expect(addOpenAIFastPolicyOpenAIAccountID([7, 12], 15)).toEqual([7, 12, 15])
  })
})
