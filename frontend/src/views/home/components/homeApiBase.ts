export type HomeSnippetUrls = {
  apiBaseUrl: string
  claudeBaseUrl: string
}

const DEFAULT_API_PREFIX = '/51Token/v1'
const CLAUDE_API_PREFIX = '/51Token'

function getRuntimeOrigin(): string {
  if (typeof window === 'undefined') return ''
  return window.location.origin || ''
}

function trimTrailingSlashes(value: string): string {
  return value.trim().replace(/\/+$/, '')
}

export function resolveHomeApiBaseUrl(configuredBaseUrl?: string | null, origin = getRuntimeOrigin()): string {
  const configured = trimTrailingSlashes(configuredBaseUrl || '')
  if (configured) return configured

  const normalizedOrigin = trimTrailingSlashes(origin)
  if (!normalizedOrigin) return DEFAULT_API_PREFIX
  return `${normalizedOrigin}${DEFAULT_API_PREFIX}`
}

export function buildClaudeBaseUrl(apiBaseUrl: string): string {
  const normalized = trimTrailingSlashes(apiBaseUrl)
  if (!normalized) return CLAUDE_API_PREFIX

  try {
    const parsed = new URL(normalized)
    const withoutV1 = parsed.pathname.replace(/\/v1$/i, '').replace(/\/+$/, '')
    parsed.pathname = withoutV1 || CLAUDE_API_PREFIX
    parsed.search = ''
    parsed.hash = ''
    return trimTrailingSlashes(parsed.toString())
  } catch {
    const withoutV1 = normalized.replace(/\/v1$/i, '')
    return withoutV1 === normalized && !/\/51Token$/i.test(withoutV1)
      ? `${withoutV1}${CLAUDE_API_PREFIX}`
      : withoutV1
  }
}

export function buildHomeSnippetUrls(configuredBaseUrl?: string | null, origin?: string): HomeSnippetUrls {
  const apiBaseUrl = resolveHomeApiBaseUrl(configuredBaseUrl, origin)
  return {
    apiBaseUrl,
    claudeBaseUrl: buildClaudeBaseUrl(apiBaseUrl)
  }
}
