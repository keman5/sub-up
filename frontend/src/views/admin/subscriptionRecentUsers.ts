import type { AdminUser } from '@/types'

const RECENT_REGISTERED_WINDOW_MS = 2 * 24 * 60 * 60 * 1000

export function filterRecentRegisteredUsers<T extends Pick<AdminUser, 'created_at'>>(
  users: T[],
  now: Date = new Date()
): T[] {
  const cutoff = now.getTime() - RECENT_REGISTERED_WINDOW_MS
  return users.filter((user) => {
    const createdAt = new Date(user.created_at).getTime()
    return Number.isFinite(createdAt) && createdAt >= cutoff
  })
}
