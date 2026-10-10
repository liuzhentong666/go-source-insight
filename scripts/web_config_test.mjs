// Run with Node after installing Web dependencies; no API server is contacted.
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { fileURLToPath, pathToFileURL } from 'node:url'

const webDirectory = fileURLToPath(new URL('../web/', import.meta.url))
const require = createRequire(new URL('../web/package.json', import.meta.url))
const { loadConfigFromFile } = await import(pathToFileURL(require.resolve('vite')).href)
const configPath = fileURLToPath(new URL('../web/vite.config.ts', import.meta.url))
const savedDirectory = process.cwd()
const savedTarget = process.env.WEB_API_PROXY_TARGET
const savedTimeout = process.env.WEB_PROXY_TIMEOUT_SECONDS
process.chdir(webDirectory)

async function config(target, timeout = '330') {
  process.env.WEB_API_PROXY_TARGET = target
  process.env.WEB_PROXY_TIMEOUT_SECONDS = timeout
  const result = await loadConfigFromFile({ command: 'serve', mode: 'development' }, configPath, undefined, 'silent')
  return result.config
}

try {
  const valid = await config('http://127.0.0.1:8123')
  assert.equal(valid.server.host, '127.0.0.1')
  assert.equal(valid.server.proxy['/api'].target, 'http://127.0.0.1:8123')
  assert.equal(valid.server.proxy['/api'].timeout, 330000)
  assert.equal(valid.server.proxy['/api'].proxyTimeout, 330000)
  assert.equal(valid.server.proxy['/api'].rewrite, undefined)
  assert.equal((await config('https://api.invalid', '60')).server.proxy['/api'].proxyTimeout, 60000)
  for (const target of ['ftp://api.invalid', 'http://user:credential@api.invalid', 'http://api.invalid/path', 'http://api.invalid?query=1', 'http://api.invalid#fragment']) {
    await assert.rejects(config(target), /WEB_API_PROXY_TARGET/)
  }
  for (const timeout of ['29', '3601', '1.5', 'invalid', '1e3', '+330']) {
    await assert.rejects(config('http://api.invalid', timeout), /WEB_PROXY_TIMEOUT_SECONDS/)
  }
  console.log('Web development proxy environment validation passed.')
} finally {
  process.chdir(savedDirectory)
  if (savedTarget === undefined) delete process.env.WEB_API_PROXY_TARGET
  else process.env.WEB_API_PROXY_TARGET = savedTarget
  if (savedTimeout === undefined) delete process.env.WEB_PROXY_TIMEOUT_SECONDS
  else process.env.WEB_PROXY_TIMEOUT_SECONDS = savedTimeout
}
