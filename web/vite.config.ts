import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  // This development-only setting must never use VITE_: server credentials and
  // the upstream address do not belong in the browser bundle.
  const env = loadEnv(mode, process.cwd(), 'WEB_')
  const target = process.env.WEB_API_PROXY_TARGET || env.WEB_API_PROXY_TARGET || 'http://127.0.0.1:8087'
  const timeoutSetting = process.env.WEB_PROXY_TIMEOUT_SECONDS || env.WEB_PROXY_TIMEOUT_SECONDS || '330'
  const timeoutSeconds = Number(timeoutSetting)
  if (!/^\d{1,4}$/.test(timeoutSetting) || timeoutSeconds < 30 || timeoutSeconds > 3600) {
    throw new Error('WEB_PROXY_TIMEOUT_SECONDS must be an integer between 30 and 3600.')
  }
  const upstream = new URL(target)
  if (!['http:', 'https:'].includes(upstream.protocol) || upstream.username || upstream.password || upstream.pathname !== '/' || upstream.search || upstream.hash) {
    throw new Error('WEB_API_PROXY_TARGET must be an HTTP(S) origin without credentials, path, query or fragment.')
  }
  return {
    plugins: [react()],
    server: {
      host: '127.0.0.1',
      port: 3000,
      proxy: {
        '/api': {
          target,
          changeOrigin: true,
          timeout: timeoutSeconds * 1000,
          proxyTimeout: timeoutSeconds * 1000,
        },
      },
    },
  }
})
