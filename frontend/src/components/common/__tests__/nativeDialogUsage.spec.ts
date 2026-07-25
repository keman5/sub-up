import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const srcRoot = resolve(__dirname, '../../..')
const allowedExtensions = new Set(['.ts', '.vue'])
const ignoredDirs = new Set(['__tests__'])
const ignoredRelativeDirs = new Set(['i18n/locales'])

function extensionOf(path: string) {
  const index = path.lastIndexOf('.')
  return index === -1 ? '' : path.slice(index)
}

function collectSourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    if (ignoredDirs.has(entry)) continue
    const path = join(dir, entry)
    if (ignoredRelativeDirs.has(relative(srcRoot, path))) continue
    const stat = statSync(path)
    if (stat.isDirectory()) {
      out.push(...collectSourceFiles(path))
    } else if (allowedExtensions.has(extensionOf(path))) {
      out.push(path)
    }
  }
  return out
}

describe('native browser dialogs', () => {
  it('uses app dialog components instead of alert/confirm/prompt', () => {
    const offenders: string[] = []
    const nativeDialogPattern =
      /(?:\bwindow\.(?:alert|confirm|prompt)\b|(?<![\w.])(?:alert|confirm|prompt)\s*\()/

    for (const file of collectSourceFiles(srcRoot)) {
      const source = readFileSync(file, 'utf8')
      if (nativeDialogPattern.test(source)) {
        offenders.push(relative(srcRoot, file))
      }
    }

    expect(offenders).toEqual([])
  })
})
