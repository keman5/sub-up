import { readFile, writeFile } from 'node:fs/promises'

const INJECTION_START = '<!-- sub2api-pages-public-settings:start -->'
const INJECTION_END = '<!-- sub2api-pages-public-settings:end -->'
const DEFAULT_TITLE_SUFFIX = ' - AI API Gateway'

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function escapeInlineJson(value) {
  return JSON.stringify(value)
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029')
}

function escapeHtmlText(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function removeExistingInjection(html) {
  const markerPattern = new RegExp(
    `${INJECTION_START}[\\s\\S]*?${INJECTION_END}\\s*`,
    'g'
  )
  return html.replace(markerPattern, '')
}

function replaceSiteTitle(html, settings) {
  const siteName = typeof settings.site_name === 'string' ? settings.site_name.trim() : ''
  if (!siteName) {
    return html
  }

  const nextTitle = `<title>${escapeHtmlText(siteName)}${DEFAULT_TITLE_SUFFIX}</title>`
  if (/<title>[\s\S]*?<\/title>/i.test(html)) {
    return html.replace(/<title>[\s\S]*?<\/title>/i, nextTitle)
  }
  return html.replace(/<\/head>/i, `${nextTitle}</head>`)
}

function extractPublicSettings(payload) {
  if (!isPlainObject(payload)) {
    throw new Error('Public settings response must be a JSON object')
  }

  if ('data' in payload) {
    if (!isPlainObject(payload.data)) {
      throw new Error('Public settings response data must be a JSON object')
    }
    return payload.data
  }

  return payload
}

function injectPublicSettingsIntoHtml(html, settings) {
  if (typeof html !== 'string' || !html.includes('</head>')) {
    throw new Error('HTML must contain a </head> tag')
  }
  if (!isPlainObject(settings)) {
    throw new Error('Public settings must be a JSON object')
  }

  const cleanHtml = removeExistingInjection(html)
  const json = escapeInlineJson(settings)
  const script = `${INJECTION_START}<script>window.__APP_CONFIG__=${json};</script>${INJECTION_END}`
  const injected = cleanHtml.replace(/<\/head>/i, `${script}</head>`)
  return replaceSiteTitle(injected, settings)
}

async function fetchPublicSettings(settingsUrl) {
  const response = await fetch(settingsUrl, {
    headers: {
      Accept: 'application/json',
      'Cache-Control': 'no-cache',
      Pragma: 'no-cache',
    },
  })
  if (!response.ok) {
    throw new Error(`Failed to fetch public settings: ${response.status} ${response.statusText}`)
  }
  return extractPublicSettings(await response.json())
}

function resolvePublicSettingsUrl({ publicUrl, settingsUrl } = {}) {
  if (settingsUrl) {
    return settingsUrl
  }
  if (!publicUrl) {
    throw new Error('Missing --public-url or --settings-url')
  }

  const url = new URL(publicUrl)
  url.pathname = '/api/v1/settings/public'
  url.search = ''
  url.hash = ''
  return url.toString()
}

async function injectPublicSettingsFile({ htmlPath, publicUrl, settingsUrl }) {
  if (!htmlPath) {
    throw new Error('Missing --html path')
  }

  const resolvedSettingsUrl = resolvePublicSettingsUrl({ publicUrl, settingsUrl })
  const [html, settings] = await Promise.all([
    readFile(htmlPath, 'utf8'),
    fetchPublicSettings(resolvedSettingsUrl),
  ])
  const rendered = injectPublicSettingsIntoHtml(html, settings)
  await writeFile(htmlPath, rendered, 'utf8')
  return {
    htmlPath,
    settingsUrl: resolvedSettingsUrl,
    siteName: typeof settings.site_name === 'string' ? settings.site_name : '',
    apiBaseUrl: typeof settings.api_base_url === 'string' ? settings.api_base_url : '',
  }
}

export {
  extractPublicSettings,
  fetchPublicSettings,
  injectPublicSettingsFile,
  injectPublicSettingsIntoHtml,
  resolvePublicSettingsUrl,
}
