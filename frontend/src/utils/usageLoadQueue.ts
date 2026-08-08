/**
 * Usage request scheduler.
 *
 * Passive usage reads can execute immediately. Explicit list refreshes perform
 * real upstream quota queries, so they are serialized to avoid a burst of
 * requests from the same browser session.
 */

import type { Account } from '@/types'

/**
 * Schedule a usage fetch. All requests execute immediately.
 */
export function enqueueUsageRequest<T>(
  _account: Account,
  fn: () => Promise<T>
): Promise<T> {
  return fn()
}

let activeUsageRefreshQueue: Promise<void> = Promise.resolve()

/**
 * Run an explicit upstream usage refresh after the previously queued refresh.
 * A failed refresh must not prevent later accounts from being queried.
 */
export function enqueueActiveUsageRefresh<T>(fn: () => Promise<T>): Promise<T> {
  const result = activeUsageRefreshQueue.then(fn, fn)
  activeUsageRefreshQueue = result.then(
    () => undefined,
    () => undefined
  )
  return result
}
