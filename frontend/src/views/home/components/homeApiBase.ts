export type HomeSnippetUrls = {
  apiBaseUrl: string
  claudeBaseUrl: string
}

const DEFAULT_API_PREFIX = '/51Token/v1'

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
  return trimTrailingSlashes(apiBaseUrl).replace(/\/v1$/i, '')
}

export function buildHomeSnippetUrls(configuredBaseUrl?: string | null, origin?: string): HomeSnippetUrls {
  const apiBaseUrl = resolveHomeApiBaseUrl(configuredBaseUrl, origin)
  return {
    apiBaseUrl,
    claudeBaseUrl: buildClaudeBaseUrl(apiBaseUrl)
  }
}
