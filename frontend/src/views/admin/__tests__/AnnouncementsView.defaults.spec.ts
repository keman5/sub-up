import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AnnouncementsView.vue')
const viewSource = readFileSync(viewPath, 'utf8')

describe('AnnouncementsView defaults', () => {
  it('opens create dialog with enabled popup defaults', () => {
    expect(viewSource).toContain("status: 'active'")
    expect(viewSource).toContain("notify_mode: 'popup'")
    expect(viewSource).toContain("form.status = 'active'")
    expect(viewSource).toContain("form.notify_mode = 'popup'")
  })
})
