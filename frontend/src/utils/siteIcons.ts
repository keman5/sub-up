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
  const icon = ensureIconLink('icon')
  const touchIcon = ensureIconLink('apple-touch-icon')
  const mimeType = resolveIconMimeType(iconUrl)
  if (mimeType) {
    icon.type = mimeType
  } else {
    icon.removeAttribute('type')
  }
  icon.href = iconUrl
  touchIcon.href = iconUrl
}
