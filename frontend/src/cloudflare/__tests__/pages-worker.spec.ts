import { describe, expect, it, vi } from 'vitest'

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

  it('proxies a2 frontend and model API requests to the ap2 origin', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('ok'))

    await worker.fetch(new Request('https://a2.upit.top/api/v1/settings/public'), createEnv())
    await worker.fetch(new Request('https://a2.upit.top/51Token/v1/chat/completions'), createEnv())

    expect((fetchMock.mock.calls[0][0] as Request).url).toBe(
      'https://ap2.upit.top/api/v1/settings/public'
    )
    expect((fetchMock.mock.calls[1][0] as Request).url).toBe(
      'https://ap2.upit.top/51Token/v1/chat/completions'
    )

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
