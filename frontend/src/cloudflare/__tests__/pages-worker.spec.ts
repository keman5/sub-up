import { afterEach, describe, expect, it, vi } from 'vitest'

import worker from '../../../public/_worker.js'

function createEnv() {
  const assetFetch = vi.fn(async () => new Response('asset'))
  return {
    ASSETS: {
      fetch: assetFetch,
    },
  }
}

describe('Cloudflare Pages worker', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('serves static assets from Pages instead of proxying to the VPS', async () => {
    const env = createEnv()

    const response = await worker.fetch(new Request('https://ai.upit.top/assets/index.js'), env)

    expect(await response.text()).toBe('asset')
    expect(env.ASSETS.fetch).toHaveBeenCalledOnce()
  })

  it('proxies frontend API requests for the primary site to the API origin', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('ok'))

    await worker.fetch(
      new Request('https://ai.upit.top/api/v1/settings/public?timezone=Asia%2FShanghai', {
        headers: {
          Host: 'ai.upit.top',
        },
      }),
      createEnv()
    )

    expect(fetchMock).toHaveBeenCalledOnce()
    expect(fetchMock.mock.calls[0][0]).toBeInstanceOf(Request)
    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://api.upit.top/api/v1/settings/public?timezone=Asia%2FShanghai'
    )
    expect((fetchMock.mock.calls[0][0] as Request).headers.get('Host')).toBeNull()
    expect((fetchMock.mock.calls[0][0] as Request).headers.get('X-Forwarded-Host')).toBe(
      'ai.upit.top'
    )

    fetchMock.mockRestore()
  })

  it('proxies OpenAI-compatible paths on the primary frontend host to the API origin', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('ok'))

    await worker.fetch(new Request('https://ai.upit.top/51Token/v1/chat/completions'), createEnv())

    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://api.upit.top/51Token/v1/chat/completions'
    )

    fetchMock.mockRestore()
  })

  it('proxies a1 frontend and model API requests to the ap1 origin', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('ok'))

    await worker.fetch(new Request('https://a1.upit.top/api/v1/settings/public'), createEnv())
    await worker.fetch(new Request('https://a1.upit.top/51Token/v1/chat/completions'), createEnv())

    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://ap1.upit.top/api/v1/settings/public'
    )
    expect((fetchMock.mock.calls[1][0] as Request).url).toBe(
      'https://ap1.upit.top/51Token/v1/chat/completions'
    )

    fetchMock.mockRestore()
  })

  it('proxies test frontend and model API requests to the a2t origin', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('ok'))

    await worker.fetch(new Request('https://test.upit.top/api/v1/settings/public'), createEnv())
    await worker.fetch(new Request('https://test.upit.top/51Token/v1/chat/completions'), createEnv())

    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://a2t.upit.top/api/v1/settings/public'
    )
    expect((fetchMock.mock.calls[1][0] as Request).url).toBe(
      'https://a2t.upit.top/51Token/v1/chat/completions'
    )

    fetchMock.mockRestore()
  })

  it('keeps the a1 custom domain on ap1 even when SUB2API_ORIGIN is set globally', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('ok'))
    const env = {
      ...createEnv(),
      SUB2API_ORIGIN: 'https://api.upit.top',
    }

    await worker.fetch(new Request('https://a1.upit.top/api/v1/settings/public'), env)

    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://ap1.upit.top/api/v1/settings/public'
    )

    fetchMock.mockRestore()
  })

  it('keeps the test frontend domain on a2t even when SUB2API_ORIGIN is set globally', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('ok'))
    const env = {
      ...createEnv(),
      SUB2API_ORIGIN: 'https://api.upit.top',
    }

    await worker.fetch(new Request('https://test.upit.top/api/v1/settings/public'), env)

    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://a2t.upit.top/api/v1/settings/public'
    )

    fetchMock.mockRestore()
  })

  it('caches public settings at the edge for GET requests', async () => {
    const originResponse = new Response(JSON.stringify({ code: 0 }), {
      headers: { 'Content-Type': 'application/json' },
    })
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(originResponse)
    const cachePut = vi.fn(async () => undefined)
    vi.stubGlobal('caches', {
      default: {
        match: vi.fn(async () => undefined),
        put: cachePut,
      },
    })

    const response = await worker.fetch(
      new Request('https://test.upit.top/api/v1/settings/public', {
        headers: {
          Authorization: 'Bearer user-token',
          Cookie: 'auth_token=user-token',
        },
      }),
      createEnv()
    )

    expect(response.headers.get('X-Sub2API-Edge-Cache')).toBe('MISS')
    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://a2t.upit.top/api/v1/settings/public'
    )
    expect((fetchMock.mock.calls[0][0] as Request).headers.get('Authorization')).toBeNull()
    expect((fetchMock.mock.calls[0][0] as Request).headers.get('Cookie')).toBeNull()
    expect(cachePut).toHaveBeenCalledOnce()

    fetchMock.mockRestore()
  })

  it('serves cached public settings without hitting the origin', async () => {
    const cached = new Response(JSON.stringify({ code: 0, cached: true }), {
      headers: { 'Content-Type': 'application/json' },
    })
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('origin'))
    vi.stubGlobal('caches', {
      default: {
        match: vi.fn(async () => cached),
        put: vi.fn(async () => undefined),
      },
    })

    const response = await worker.fetch(
      new Request('https://test.upit.top/api/v1/settings/public'),
      createEnv()
    )

    expect(fetchMock).not.toHaveBeenCalled()
    expect(response.headers.get('X-Sub2API-Edge-Cache')).toBe('HIT')
    expect(await response.json()).toEqual({ code: 0, cached: true })

    fetchMock.mockRestore()
  })

  it('does not edge-cache mutating or gateway requests', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('ok'))
    const cachePut = vi.fn(async () => undefined)
    vi.stubGlobal('caches', {
      default: {
        match: vi.fn(async () => undefined),
        put: cachePut,
      },
    })

    await worker.fetch(
      new Request('https://test.upit.top/api/v1/auth/login', { method: 'POST' }),
      createEnv()
    )
    await worker.fetch(new Request('https://test.upit.top/51Token/v1/models'), createEnv())

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(cachePut).not.toHaveBeenCalled()

    fetchMock.mockRestore()
  })

  it('does not edge-cache HEAD requests because the origin does not serve public settings with HEAD', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('not found', { status: 404 }))
    const cacheMatch = vi.fn(async () => undefined)
    const cachePut = vi.fn(async () => undefined)
    vi.stubGlobal('caches', {
      default: {
        match: cacheMatch,
        put: cachePut,
      },
    })

    const response = await worker.fetch(
      new Request('https://test.upit.top/api/v1/settings/public', { method: 'HEAD' }),
      createEnv()
    )

    expect(fetchMock).toHaveBeenCalledOnce()
    expect(cacheMatch).not.toHaveBeenCalled()
    expect(cachePut).not.toHaveBeenCalled()
    expect(response.headers.get('X-Sub2API-Edge-Cache')).toBeNull()

    fetchMock.mockRestore()
  })

  it('uses SUB2API_ORIGIN when preview deployments need an explicit origin', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response('ok'))
    const env = {
      ...createEnv(),
      SUB2API_ORIGIN: 'https://preview-origin.example.com/base/',
    }

    await worker.fetch(new Request('https://preview.pages.dev/api/v1/settings/public'), env)

    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://preview-origin.example.com/base/api/v1/settings/public'
    )

    fetchMock.mockRestore()
  })
})
