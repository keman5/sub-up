import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const sourcePath = resolve(dirname(fileURLToPath(import.meta.url)), '../PaymentProviderDialog.vue')
const source = readFileSync(sourcePath, 'utf8')

describe('PaymentProviderDialog native alerts', () => {
  it('uses the app dialog/toast channel instead of native alert fallbacks', () => {
    expect(source).not.toContain('window alert fallback')
    expect(source).not.toContain('window.alert')
    expect(source).not.toContain('alert(')
    expect(source).toContain('const appStore = useAppStore()')
    expect(source).toContain('appStore.showError(msg)')
  })
})
