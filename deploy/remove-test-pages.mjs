import { readFile } from 'node:fs/promises'
import { homedir } from 'node:os'

const config = await readFile(`${homedir()}/Library/Preferences/.wrangler/config/default.toml`, 'utf8')
const token = process.env.CLOUDFLARE_API_TOKEN || config.match(/^oauth_token\s*=\s*"([^"]+)"/m)?.[1]
if (!token) throw new Error('Wrangler authentication missing')
const base = '/accounts/1b141aa7926dded899acf369e71e8aa8/pages/projects/sub2api-frontend-test'
async function api(path, method = 'GET') {
  const response = await fetch(`https://api.cloudflare.com/client/v4${path}`, {
    method, headers: { Authorization: `Bearer ${token}` },
  })
  const body = await response.json()
  if (response.status === 404 && method === 'GET') return null
  if (!body.success) throw new Error(JSON.stringify(body.errors))
  return body.result
}
if (await api(base)) {
  for (const domain of await api(`${base}/domains`)) {
    if (domain.name !== 'test.upit.top') throw new Error(`Unexpected test domain: ${domain.name}`)
    await api(`${base}/domains/${domain.name}`, 'DELETE')
    console.log(`Removed test Pages domain: ${domain.name}`)
  }
  await api(base, 'DELETE')
  console.log('Removed sub2api-frontend-test Pages project')
}
const zones = await api('/zones?name=upit.top')
if (zones.length !== 1) throw new Error('Expected exactly one upit.top zone')
for (const name of ['test.upit.top', 'a2t.upit.top', 'test.51tokens.upit.top']) {
  const path = `/zones/${zones[0].id}/dns_records`
  for (const record of await api(`${path}?name=${name}`)) {
    if (record.name !== name) throw new Error('Unexpected DNS record')
    await api(`${path}/${record.id}`, 'DELETE')
    console.log(`Removed retired test DNS: ${name}`)
  }
  if ((await api(`${path}?name=${name}`)).length) throw new Error(`DNS cleanup failed: ${name}`)
}
