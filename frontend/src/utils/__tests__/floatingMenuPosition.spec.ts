import { describe, expect, it } from 'vitest'

import { clampFloatingMenuPosition, getActionMenuPosition } from '../floatingMenuPosition'

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

  it('keeps a mobile action menu inside the left edge when its trigger is near the edge', () => {
    const result = getActionMenuPosition({
      triggerRect: { top: 120, right: 32, bottom: 152, left: 8, width: 24 },
      pointerX: 20,
      pointerY: 136,
      menuSize: { width: 208, height: 240 },
      viewport: { width: 320, height: 640 }
    })

    expect(result.left).toBe(8)
    expect(result.left + 208).toBeLessThanOrEqual(312)
  })

  it('clamps a wide dropdown that would otherwise start outside the viewport', () => {
    const result = clampFloatingMenuPosition(
      { top: 80, left: -120 },
      { width: 288, height: 420 },
      { width: 320, height: 640 }
    )

    expect(result.left).toBe(8)
    expect(result.left + 288).toBeLessThanOrEqual(312)
  })
})
