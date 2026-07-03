import { describe, expect, it } from 'vitest'

import { clampFloatingMenuPosition } from '../floatingMenuPosition'

describe('floating menu positioning', () => {
  it('keeps a measured menu inside the bottom edge of the viewport', () => {
    const result = clampFloatingMenuPosition(
      { top: 552, left: 700 },
      { width: 208, height: 400 },
      { width: 1024, height: 800 },
      8
    )

    expect(result.top).toBe(392)
    expect(result.left).toBe(700)
    expect(result.maxHeight).toBe(784)
  })

  it('limits the menu height when the viewport is shorter than the menu', () => {
    const result = clampFloatingMenuPosition(
      { top: 20, left: 20 },
      { width: 208, height: 700 },
      { width: 390, height: 360 },
      8
    )

    expect(result.top).toBe(8)
    expect(result.left).toBe(20)
    expect(result.maxHeight).toBe(344)
  })
})
