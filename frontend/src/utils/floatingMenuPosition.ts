export interface FloatingPoint {
  top: number
  left: number
}

export interface FloatingSize {
  width: number
  height: number
}

export interface FloatingViewport {
  width: number
  height: number
}

export interface TriggerRect {
  top: number
  right: number
  bottom: number
  left: number
  width: number
}

export interface ActionMenuPositionOptions {
  triggerRect: TriggerRect
  pointerX: number
  pointerY: number
  menuSize: FloatingSize
  viewport: FloatingViewport
  padding?: number
  offset?: number
  mobileBreakpoint?: number
}

export function clampFloatingMenuPosition(
  position: FloatingPoint,
  size: FloatingSize,
  viewport: FloatingViewport,
  padding = 8
): FloatingPoint & { maxHeight: number } {
  const maxHeight = Math.max(0, viewport.height - padding * 2)
  const effectiveHeight = Math.min(size.height, maxHeight)
  const effectiveWidth = Math.min(size.width, Math.max(0, viewport.width - padding * 2))

  const top = Math.max(
    padding,
    Math.min(position.top, viewport.height - effectiveHeight - padding)
  )
  const left = Math.max(
    padding,
    Math.min(position.left, viewport.width - effectiveWidth - padding)
  )

  return { top, left, maxHeight }
}

export function getActionMenuPosition(options: ActionMenuPositionOptions): FloatingPoint & { maxHeight: number } {
  const {
    triggerRect,
    pointerX,
    pointerY,
    menuSize,
    viewport,
    padding = 8,
    offset = 4,
    mobileBreakpoint = 768
  } = options

  let left: number
  let top: number

  if (viewport.width < mobileBreakpoint) {
    left = triggerRect.left + triggerRect.width / 2 - menuSize.width / 2
    top = triggerRect.bottom + offset

    if (top + menuSize.height > viewport.height - padding) {
      top = triggerRect.top - menuSize.height - offset
    }
  } else {
    left = pointerX - menuSize.width
    top = pointerY
  }

  return clampFloatingMenuPosition({ top, left }, menuSize, viewport, padding)
}
