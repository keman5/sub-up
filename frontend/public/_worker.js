const PRIMARY_API_ORIGIN = 'https://api.upit.top'
const A1_API_ORIGIN = 'https://ap1.upit.top'
const TEST_API_ORIGIN = 'https://a2t.upit.top'

const PRODUCTION_API_ORIGINS_BY_HOST = new Map([
  ['ai.upit.top', PRIMARY_API_ORIGIN],
  ['a1.upit.top', A1_API_ORIGIN],
  ['test.upit.top', TEST_API_ORIGIN],
])

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

const EDGE_CACHE_TTL_SECONDS = 60
const EDGE_CACHEABLE_PUBLIC_PATHS = new Set([
  '/api/v1/settings/public',
  '/api/status',
  '/api/home_page_content',
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
  const productionOrigin = PRODUCTION_API_ORIGINS_BY_HOST.get(url.hostname)
  if (productionOrigin) {
    return productionOrigin
  }

  const configuredOrigin = normalizeOrigin(env.SUB2API_ORIGIN)
  if (configuredOrigin) {
    return configuredOrigin
  }

  return PRIMARY_API_ORIGIN
}

function createOriginRequest(request, origin, options = {}) {
  const sourceUrl = new URL(request.url)
  const targetUrl = new URL(origin)
  const originPath = targetUrl.pathname.replace(/\/+$/, '')
  targetUrl.pathname = `${originPath}${sourceUrl.pathname}`
  targetUrl.search = sourceUrl.search

  const headers = new Headers(request.headers)
  headers.delete('Host')
  headers.delete('Connection')
  headers.delete('Upgrade')
  if (options.stripSensitiveHeaders) {
    headers.delete('Authorization')
    headers.delete('Cookie')
  }
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

function isEdgeCacheablePublicRequest(request, url) {
  return (
    request.method === 'GET' &&
    EDGE_CACHEABLE_PUBLIC_PATHS.has(url.pathname)
  )
}

function shouldBypassEdgeCache(request) {
  const cacheControl = request.headers.get('Cache-Control') || ''
  const pragma = request.headers.get('Pragma') || ''
  return /\bno-cache\b|\bno-store\b/i.test(cacheControl) || /\bno-cache\b/i.test(pragma)
}

function getEdgeCache() {
  if (typeof caches === 'undefined' || !caches.default) {
    return null
  }
  return caches.default
}

function createPublicEdgeCacheKey(request) {
  const url = new URL(request.url)
  return new Request(url.toString(), { method: 'GET' })
}

function withEdgeCacheHeader(response, value) {
  const headers = new Headers(response.headers)
  headers.set('X-Sub2API-Edge-Cache', value)
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  })
}

function withPublicCacheHeaders(response) {
  const headers = new Headers(response.headers)
  headers.set('Cache-Control', `public, max-age=30, s-maxage=${EDGE_CACHE_TTL_SECONDS}`)
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  })
}

async function fetchWithPublicEdgeCache(request, origin) {
  const originRequest = createOriginRequest(request, origin, { stripSensitiveHeaders: true })
  const cache = getEdgeCache()
  if (!cache || shouldBypassEdgeCache(request)) {
    const response = await fetch(originRequest)
    return withEdgeCacheHeader(response, cache ? 'BYPASS' : 'UNAVAILABLE')
  }

  const cacheKey = createPublicEdgeCacheKey(request)
  const cached = await cache.match(cacheKey)
  if (cached) {
    return withEdgeCacheHeader(cached, 'HIT')
  }

  const originResponse = await fetch(originRequest)
  const response = withPublicCacheHeaders(originResponse)
  if (response.status === 200) {
    await cache.put(cacheKey, response.clone())
  }
  return withEdgeCacheHeader(response, 'MISS')
}

const worker = {
  async fetch(request, env) {
    const url = new URL(request.url)
    if (shouldProxyToOrigin(url.pathname)) {
      const origin = originForRequest(url, env || {})
      if (isEdgeCacheablePublicRequest(request, url)) {
        return fetchWithPublicEdgeCache(request, origin)
      }
      return fetch(createOriginRequest(request, origin))
    }

    return env.ASSETS.fetch(request)
  },
}

export {
  createOriginRequest,
  isEdgeCacheablePublicRequest,
  originForRequest,
  shouldProxyToOrigin,
}
export default worker
