import type { SubscriptionProgress, UserSubscription } from '@/types'

type UsageWindowProgress = NonNullable<SubscriptionProgress['daily']>

const getUsedUSD = (window: UsageWindowProgress): number | null => {
  if (typeof window.used_usd === 'number') return window.used_usd
  if (typeof window.used === 'number') return window.used
  return null
}

const getWindowStart = (window: UsageWindowProgress): string | null => {
  if (typeof window.window_start === 'string' && window.window_start) {
    return window.window_start
  }
  return null
}

export function mergeSubscriptionProgress(
  subscription: UserSubscription,
  progress: SubscriptionProgress
): UserSubscription {
  const next = { ...subscription }

  if (progress.daily) {
    const used = getUsedUSD(progress.daily)
    if (used !== null) next.daily_usage_usd = used
    const windowStart = getWindowStart(progress.daily)
    if (windowStart) next.daily_window_start = windowStart
  }

  if (progress.weekly) {
    const used = getUsedUSD(progress.weekly)
    if (used !== null) next.weekly_usage_usd = used
    const windowStart = getWindowStart(progress.weekly)
    if (windowStart) next.weekly_window_start = windowStart
  }

  if (progress.monthly) {
    const used = getUsedUSD(progress.monthly)
    if (used !== null) next.monthly_usage_usd = used
    const windowStart = getWindowStart(progress.monthly)
    if (windowStart) next.monthly_window_start = windowStart
  }

  if (progress.expires_at) {
    next.expires_at = progress.expires_at
  }

  return next
}

export function mergeSubscriptionProgressById(
  subscriptions: UserSubscription[],
  subscriptionId: number,
  progress: SubscriptionProgress
): UserSubscription[] {
  return subscriptions.map(subscription =>
    subscription.id === subscriptionId
      ? mergeSubscriptionProgress(subscription, progress)
      : subscription
  )
}
