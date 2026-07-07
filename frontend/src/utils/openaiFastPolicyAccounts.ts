type RawAccountID = number | string | null | undefined

export function normalizeOpenAIFastPolicyOpenAIAccountAllowlist(
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

export function addOpenAIFastPolicyOpenAIAccountID(
  accountIDs: RawAccountID[] | undefined,
  accountID: RawAccountID
): number[] {
  return normalizeOpenAIFastPolicyOpenAIAccountAllowlist([
    ...(accountIDs || []),
    accountID
  ]) || []
}
