import fs from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleObject = Record<string, unknown>

const srcDir = path.resolve(process.cwd(), 'src')
const staticKeyPatterns = [
  /\b(?:t|\$t)\(\s*['"]([A-Za-z0-9_.-]+)['"]/g,
  /\b(?:titleKey|descriptionKey|messageKey|labelKey|placeholderKey)\s*[:=]\s*['"]([A-Za-z0-9_.-]+)['"]/g,
  /\bi18nKey\s*[:=]\s*['"]([A-Za-z0-9_.-]+)['"]/g
]

function flattenKeys(value: unknown, prefix = '', out = new Set<string>()): Set<string> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    for (const [key, nested] of Object.entries(value as LocaleObject)) {
      flattenKeys(nested, prefix ? `${prefix}.${key}` : key, out)
    }
  } else if (prefix) {
    out.add(prefix)
  }
  return out
}

function listSourceFiles(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === '__tests__' || entry.name === 'locales') continue
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      listSourceFiles(fullPath, out)
    } else if (/\.(vue|ts|tsx|js|jsx)$/.test(entry.name)) {
      out.push(fullPath)
    }
  }
  return out
}

function collectStaticRefs(): string[] {
  const refs = new Set<string>()
  for (const file of listSourceFiles(srcDir)) {
    const source = fs.readFileSync(file, 'utf8')
    for (const pattern of staticKeyPatterns) {
      let match: RegExpExecArray | null
      while ((match = pattern.exec(source))) {
        const key = match[1]
        if (key.includes('.') && !key.endsWith('.')) {
          refs.add(key)
        }
      }
    }
  }
  return [...refs].sort()
}

describe('locale static references', () => {
  it('keeps English and Chinese locale leaf keys in sync', () => {
    const enKeys = flattenKeys(en)
    const zhKeys = flattenKeys(zh)

    expect(
      [...enKeys].filter((key) => !zhKeys.has(key)).sort(),
      'English-only locale keys'
    ).toEqual([])
    expect(
      [...zhKeys].filter((key) => !enKeys.has(key)).sort(),
      'Chinese-only locale keys'
    ).toEqual([])
  })

  it('all production static i18n keys exist in English and Chinese locales', () => {
    const refs = collectStaticRefs()
    const enKeys = flattenKeys(en)
    const zhKeys = flattenKeys(zh)

    expect(
      refs.filter((key) => !enKeys.has(key)),
      'missing English locale keys'
    ).toEqual([])
    expect(
      refs.filter((key) => !zhKeys.has(key)),
      'missing Chinese locale keys'
    ).toEqual([])
  })
})
