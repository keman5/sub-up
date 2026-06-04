import type { UserSubscription } from '@/types'

type UserColumnMode = 'email' | 'username'
type SubscriptionUserDisplay = Pick<UserSubscription, 'user_id'> & {
  user?: UserSubscription['user'] & { notes?: string | null }
}

export const getSubscriptionUserLabel = (
  subscription: SubscriptionUserDisplay,
  mode: UserColumnMode,
  fallback: string
) => {
  const user = subscription.user
  return mode === 'email'
    ? (user?.email || fallback)
    : (user?.username || '-')
}

export const getSubscriptionUserNotes = (
  subscription: Pick<SubscriptionUserDisplay, 'user'>
) => {
  const notes = subscription.user?.notes?.trim()
  return notes || ''
}
