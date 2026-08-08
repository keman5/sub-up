import { describe, expect, it } from 'vitest'

import {
  extractPublicSettings,
  injectPublicSettingsIntoHtml,
  resolvePublicSettingsUrl,
} from '../../../../scripts/cloudflare-pages-config.mjs'

describe('Cloudflare Pages public settings injection', () => {
  it('derives the settings URL from the public frontend URL', () => {
    expect(resolvePublicSettingsUrl({ publicUrl: 'https://test.upit.top/login' })).toBe(
      'https://test.upit.top/api/v1/settings/public'
    )
  })

  it('keeps an explicit settings URL for advanced deployments', () => {
    expect(
      resolvePublicSettingsUrl({
        publicUrl: 'https://test.upit.top',
        settingsUrl: 'https://example.com/custom/settings',
      })
    ).toBe('https://example.com/custom/settings')
  })

  it('unwraps the public settings response data object', () => {
    const settings = extractPublicSettings({
      code: 0,
      message: 'success',
      data: {
        site_name: '51Token A2',
        api_base_url: 'https://a2t.upit.top',
      },
    })

    expect(settings).toEqual({
      site_name: '51Token A2',
      api_base_url: 'https://a2t.upit.top',
    })
  })

  it('injects safe inline config and replaces the initial title', () => {
    const html = '<!doctype html><html><head><title>Sub2API</title></head><body></body></html>'
    const rendered = injectPublicSettingsIntoHtml(html, {
      site_name: 'A2 <Inner>',
      api_base_url: 'https://a2t.upit.top',
      dangerous: '</script><img src=x onerror=alert(1)>',
    })

    expect(rendered).toContain('<title>A2 &lt;Inner&gt; - AI API Gateway</title>')
    expect(rendered).toContain('window.__APP_CONFIG__=')
    expect(rendered).toContain('"api_base_url":"https://a2t.upit.top"')
    expect(rendered).toContain('\\u003c/script\\u003e')
    expect(rendered).not.toContain('</script><img')
    expect(rendered.indexOf('window.__APP_CONFIG__')).toBeLessThan(rendered.indexOf('</head>'))
  })

  it('replaces an existing Pages config injection when run repeatedly', () => {
    const html = injectPublicSettingsIntoHtml(
      '<html><head><title>Old</title></head><body></body></html>',
      { site_name: 'Old', api_base_url: 'https://old.example.com' }
    )

    const rendered = injectPublicSettingsIntoHtml(html, {
      site_name: 'New',
      api_base_url: 'https://new.example.com',
    })

    expect(rendered.match(/window\.__APP_CONFIG__/g)).toHaveLength(1)
    expect(rendered).toContain('"api_base_url":"https://new.example.com"')
    expect(rendered).not.toContain('https://old.example.com')
    expect(rendered).toContain('<title>New - AI API Gateway</title>')
  })
})
