export function formatAdminUsageModel(model?: string | null, upstreamModel?: string | null): string {
  const requested = model?.trim() || '-'
  const upstream = upstreamModel?.trim()
  if (upstream && upstream !== requested) {
    return `${requested} (${upstream})`
  }
  return requested
}
