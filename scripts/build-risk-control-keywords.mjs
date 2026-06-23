#!/usr/bin/env node
import { readFileSync } from 'node:fs'

const files = process.argv.slice(2)

if (files.length === 0) {
  console.error('Usage: node scripts/build-risk-control-keywords.mjs <keyword-file>...')
  process.exit(1)
}

const seen = new Set()
const merged = []

for (const file of files) {
  const content = readFileSync(file, 'utf8')
  for (const rawLine of content.split(/\r?\n/)) {
    const keyword = rawLine.trim()
    if (!keyword || keyword.startsWith('#')) continue
    const dedupeKey = keyword.toLocaleLowerCase()
    if (seen.has(dedupeKey)) continue
    seen.add(dedupeKey)
    merged.push(keyword)
  }
}

process.stdout.write(`${merged.join('\n')}\n`)
