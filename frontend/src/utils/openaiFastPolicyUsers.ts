type RawUserID = number | string | null | undefined

export function normalizeOpenAIFastPolicyUserAllowlist(
  userIDs?: RawUserID[]
): number[] | undefined {
  const normalized = Array.from(
    new Set(
      (userIDs || [])
        .map((id) => Number(id))
        .filter((id) => Number.isInteger(id) && id > 0)
    )
  )
  return normalized.length > 0 ? normalized : undefined
}

export function addOpenAIFastPolicyUserID(
  userIDs: RawUserID[] | undefined,
  userID: RawUserID
): number[] {
  return normalizeOpenAIFastPolicyUserAllowlist([
    ...(userIDs || []),
    userID
  ]) || []
}
