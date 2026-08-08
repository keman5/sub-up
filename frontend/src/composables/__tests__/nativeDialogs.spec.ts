import fs from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'

const sourceRoot = path.resolve(process.cwd(), 'src')
const sourceFilePattern = /\.(vue|ts|tsx|js|jsx)$/
const nativeDialogPattern = /\bwindow\.(?:alert|confirm|prompt)\s*\(|(?<![.\w])(?:alert|confirm|prompt)\s*\(/g

function listProductionSourceFiles(directory: string, files: string[] = []): string[] {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.name === '__tests__') continue
    const fullPath = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      listProductionSourceFiles(fullPath, files)
    } else if (sourceFilePattern.test(entry.name)) {
      files.push(fullPath)
    }
  }
  return files
}

describe('native browser dialogs', () => {
  it('uses the application dialog host instead of alert, confirm, or prompt', () => {
    const nativeCalls = listProductionSourceFiles(sourceRoot).flatMap((file) => {
      const source = fs.readFileSync(file, 'utf8')
      const matches = [...source.matchAll(nativeDialogPattern)]
      return matches.map((match) => `${path.relative(sourceRoot, file)}:${match.index}`)
    })

    expect(nativeCalls).toEqual([])
  })
})
