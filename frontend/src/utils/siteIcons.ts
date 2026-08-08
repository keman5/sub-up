import { sanitizeUrl } from '@/utils/url'

export function resolveIconMimeType(iconUrl: string): string {
  const pathname = iconUrl.split(/[?#]/, 1)[0]?.toLowerCase() || ''
  if (pathname.endsWith('.svg')) return 'image/svg+xml'
  if (pathname.endsWith('.png')) return 'image/png'
  if (pathname.endsWith('.ico')) return 'image/x-icon'
  return ''
}

function ensureIconLink(rel: string) {
  let link = document.querySelector<HTMLLinkElement>(`link[rel="${rel}"]`)
  if (!link) {
    link = document.createElement('link')
    link.rel = rel
    document.head.appendChild(link)
  }
  return link
}

export function applySiteIcons(iconUrl: string) {
  // Public settings can supply this URL, so reject unsafe schemes before
  // assigning it to DOM link elements.
  const sanitizedIconUrl = sanitizeUrl(iconUrl, {
    allowRelative: true,
    allowDataUrl: true,
  })
  if (!sanitizedIconUrl) return

  const icon = ensureIconLink('icon')
  const touchIcon = ensureIconLink('apple-touch-icon')
  const mimeType = resolveIconMimeType(sanitizedIconUrl)
  if (mimeType) {
    icon.type = mimeType
  } else {
    icon.removeAttribute('type')
  }
  icon.href = sanitizedIconUrl
  touchIcon.href = sanitizedIconUrl
}
