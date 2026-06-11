const PRIMARY_API_ORIGIN = 'https://api.upit.top'
const A2_API_ORIGIN = 'https://ap2.upit.top'

const API_PATH_PREFIXES = [
  '/api/',
  '/51Token/',
  '/v1/',
  '/v1beta/',
  '/backend-api/',
  '/antigravity/',
  '/setup/',
  '/responses/',
  '/images/',
]

const API_EXACT_PATHS = new Set([
  '/health',
  '/responses',
])

function normalizeOrigin(origin) {
  return String(origin || '').trim().replace(/\/+$/, '')
}

function shouldProxyToOrigin(pathname) {
  if (API_EXACT_PATHS.has(pathname)) {
    return true
  }
  return API_PATH_PREFIXES.some((prefix) => pathname.startsWith(prefix))
}

function originForRequest(url, env) {
  const configuredOrigin = normalizeOrigin(env.SUB2API_ORIGIN)
  if (configuredOrigin) {
    return configuredOrigin
  }

  if (url.hostname === 'a2.upit.top') {
    return A2_API_ORIGIN
  }

  return PRIMARY_API_ORIGIN
}

function createOriginRequest(request, origin) {
  const sourceUrl = new URL(request.url)
  const targetUrl = new URL(origin)
  const originPath = targetUrl.pathname.replace(/\/+$/, '')
  targetUrl.pathname = `${originPath}${sourceUrl.pathname}`
  targetUrl.search = sourceUrl.search

  const headers = new Headers(request.headers)
  headers.delete('Host')
  headers.delete('Connection')
  headers.delete('Upgrade')
  headers.set('X-Forwarded-Host', sourceUrl.host)
  headers.set('X-Forwarded-Proto', sourceUrl.protocol.replace(':', ''))

  return new Request(targetUrl.toString(), {
    method: request.method,
    headers,
    body: request.body,
    redirect: request.redirect,
    cf: request.cf,
  })
}

const worker = {
  async fetch(request, env) {
    const url = new URL(request.url)
    if (shouldProxyToOrigin(url.pathname)) {
      return fetch(createOriginRequest(request, originForRequest(url, env || {})))
    }

    return env.ASSETS.fetch(request)
  },
}

export { createOriginRequest, originForRequest, shouldProxyToOrigin }
export default worker
