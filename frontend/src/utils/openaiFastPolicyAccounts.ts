type RawAccountID = number | string | null | undefined

export function normalizeOpenAIFastPolicyAccountAllowlist(
  accountIDs?: RawAccountID[]
): number[] | undefined {
  const normalized = Array.from(
    new Set(
      (accountIDs || [])
        .map((id) => Number(id))
        .filter((id) => Number.isInteger(id) && id > 0)
    )
  )
  return normalized.length > 0 ? normalized : undefined
}

export function addOpenAIFastPolicyAccountID(
  accountIDs: RawAccountID[] | undefined,
  accountID: RawAccountID
): number[] {
  return normalizeOpenAIFastPolicyAccountAllowlist([
    ...(accountIDs || []),
    accountID
  ]) || []
}
