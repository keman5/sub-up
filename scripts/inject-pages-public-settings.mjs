#!/usr/bin/env node

import { injectPublicSettingsFile } from './cloudflare-pages-config.mjs'

function parseArgs(argv) {
  const args = {}
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (!arg.startsWith('--')) {
      throw new Error(`Unexpected argument: ${arg}`)
    }
    const key = arg.slice(2)
    const value = argv[index + 1]
    if (!value || value.startsWith('--')) {
      throw new Error(`Missing value for ${arg}`)
    }
    args[key] = value
    index += 1
  }
  return args
}

async function main() {
  const args = parseArgs(process.argv.slice(2))
  const result = await injectPublicSettingsFile({
    htmlPath: args.html,
    publicUrl: args['public-url'],
    settingsUrl: args['settings-url'],
  })

  console.log(
    `Injected public settings into ${result.htmlPath} from ${result.settingsUrl}` +
      (result.siteName ? ` (site_name=${result.siteName})` : '')
  )
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error))
  process.exitCode = 1
})
