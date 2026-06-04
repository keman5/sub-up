import { describe, expect, it } from 'vitest'
import { filterRecentRegisteredUsers } from '../subscriptionRecentUsers'

describe('filterRecentRegisteredUsers', () => {
  it('keeps only users registered within the last two days', () => {
    const now = new Date('2026-06-05T12:00:00Z')
    const users = [
      { id: 1, created_at: '2026-06-05T11:59:00Z' },
      { id: 2, created_at: '2026-06-03T12:00:00Z' },
      { id: 3, created_at: '2026-06-03T11:59:59Z' },
      { id: 4, created_at: 'not-a-date' }
    ]

    expect(filterRecentRegisteredUsers(users, now).map((user) => user.id)).toEqual([1, 2])
  })
})
