type RateGroup = {
  rate_multiplier?: number
  display_rate_multiplier?: number
}

export function getGroupDisplayRateMultiplier(group?: RateGroup | null): number {
  const displayRate = group?.display_rate_multiplier
  if (typeof displayRate === 'number' && Number.isFinite(displayRate) && displayRate > 0) {
    return displayRate
  }
  return 1
}
