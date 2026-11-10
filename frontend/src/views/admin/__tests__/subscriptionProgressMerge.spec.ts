import { describe, expect, it } from 'vitest'
import type { SubscriptionProgress, UserSubscription } from '@/types'
import { mergeSubscriptionProgressById } from '../subscriptionProgressMerge'

const makeSubscription = (id: number): UserSubscription => ({
  id,
  user_id: id + 10,
  group_id: 1,
  status: 'active',
  starts_at: '2026-06-01T00:00:00Z',
  expires_at: '2026-07-01T00:00:00Z',
  daily_usage_usd: 1,
  weekly_usage_usd: 2,
  monthly_usage_usd: 3,
  total_usage_usd: 9,
  daily_window_start: '2026-06-03T00:00:00Z',
  weekly_window_start: '2026-06-01T00:00:00Z',
  monthly_window_start: '2026-06-01T00:00:00Z',
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z'
})

describe('mergeSubscriptionProgressById', () => {
  it.each([
    [{ used_usd: 7, used: 99, percentage: 7 }, 7],
    [{ used_usd: 0, used: 99, percentage: 0 }, 0],
    [{ used: 6, percentage: 6 }, 6],
    [{ percentage: 0 }, 9],
    [null, 9],
    [undefined, 9]
  ])('merges total using the existing used_usd/used precedence without changing other fields (%j)', (total, expected) => {
    const subscriptions = [makeSubscription(1), makeSubscription(2)]
    const before = structuredClone(subscriptions)
    const next = mergeSubscriptionProgressById(subscriptions, 2, {
      subscription_id: 2, daily: null, weekly: null, monthly: null,
      total, expires_at: null, days_remaining: null
    })
    expect(next[0]).toBe(subscriptions[0])
    expect(next[1]).toEqual({ ...subscriptions[1], total_usage_usd: expected })
    expect(subscriptions).toEqual(before)
  })

  it('updates only the matching subscription with latest progress fields', () => {
    const subscriptions = [makeSubscription(1), makeSubscription(2)]
    const progress: SubscriptionProgress = {
      id: 2,
      daily: {
        used_usd: 4.5,
        limit_usd: 10,
        percentage: 45,
        window_start: '2026-06-04T00:00:00Z',
        resets_at: '2026-06-05T00:00:00Z',
        resets_in_seconds: 3600
      },
      weekly: {
        used_usd: 8,
        limit_usd: 50,
        percentage: 16,
        window_start: '2026-06-02T00:00:00Z',
        resets_at: '2026-06-09T00:00:00Z',
        resets_in_seconds: 7200
      },
      monthly: null,
      expires_at: '2026-07-01T00:00:00Z',
      days_remaining: 27
    }

    const next = mergeSubscriptionProgressById(subscriptions, 2, progress)

    expect(next[0]).toEqual(subscriptions[0])
    expect(next[1]).toMatchObject({
      daily_usage_usd: 4.5,
      weekly_usage_usd: 8,
      monthly_usage_usd: 3,
      daily_window_start: '2026-06-04T00:00:00Z',
      weekly_window_start: '2026-06-02T00:00:00Z',
      monthly_window_start: '2026-06-01T00:00:00Z'
    })
  })
})
