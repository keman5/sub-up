import { afterEach, describe, expect, it, vi } from 'vitest'
import { formatCountdown, formatCountdownWithSuffix } from '../format'
import { i18n } from '@/i18n'

describe('formatCountdown', () => {
  const originalT = i18n.global.t

  afterEach(() => {
    vi.useRealTimers()
    i18n.global.t = originalT
  })

  it('does not expose translation keys when countdown messages are missing', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-04T00:00:00Z'))
    i18n.global.t = vi.fn((key: string) => key) as typeof i18n.global.t

    expect(formatCountdown('2026-06-06T03:00:00Z')).toBe('2d 3h')
    expect(formatCountdown('2026-06-04T05:30:00Z')).toBe('5h 30m')
    expect(formatCountdown('2026-06-04T00:15:00Z')).toBe('15m')
    expect(formatCountdownWithSuffix('2026-06-06T03:00:00Z')).toBe('2d 3h to lift')
  })

  it('falls back when i18n returns a message containing the missing key', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-04T00:00:00Z'))
    i18n.global.t = vi.fn((key: string) => `missing:${key}`) as typeof i18n.global.t

    expect(formatCountdown('2026-06-06T03:00:00Z')).toBe('2d 3h')
  })
})
