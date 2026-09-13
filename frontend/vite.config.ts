import { defineConfig, loadEnv, Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import type { IncomingMessage, ServerResponse } from 'http'
import { resolve } from 'path'

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  })[character] || character)
}

function isSafeImageUrl(value: string): boolean {
  const trimmed = value.trim()
  if ((trimmed.startsWith('/') && !trimmed.startsWith('//')) || /^data:image\//i.test(trimmed)) {
    return true
  }
  try {
    const parsed = new URL(trimmed)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

function injectBranding(html: string, config: { site_name?: string; site_logo?: string }): string {
  let brandedHtml = html
  const siteName = config.site_name?.trim()
  if (siteName) {
    brandedHtml = brandedHtml.replace(
      /<title>\s*Sub2API(?:\s*-\s*AI API Gateway)?\s*<\/title>/i,
      `<title>${escapeHtml(siteName)} - AI API Gateway</title>`,
    )
  }

  const siteLogo = config.site_logo?.trim()
  if (siteLogo && isSafeImageUrl(siteLogo)) {
    brandedHtml = brandedHtml.replace(
      /<link\s+rel=["']icon["'][^>]*>/i,
      `<link rel="icon" href="${escapeHtml(siteLogo)}" />`,
    )
  }
  return brandedHtml
}

/**
 * Vite 插件：开发模式下注入公开配置到 index.html
 * 与生产模式的后端注入行为保持一致，消除闪烁
 */
function mapAppCatalogPath(pathname: string, method = 'GET'): string | null {
  const verb = method.toUpperCase()
  if (pathname === '/api/v1/app-catalog' || pathname === '/api/v1/app-catalog/') {
    return verb === 'GET' || verb === 'HEAD' ? '/v1/apps' : null
  }
  const prefix = '/api/v1/admin/app-catalog'
  if (pathname !== prefix && !pathname.startsWith(`${prefix}/`)) {
    return null
  }
  const rest = pathname.slice(prefix.length) || '/'
  if (rest === '/agent/health') return '/health'
  if (rest === '/fetch') return '/v1/fetch'
  if (rest === '/' || rest === '') return '/v1/apps'
  if (/^\/\d+$/.test(rest)) return `/v1/apps${rest}`
  return null
}

function sendJSON(res: ServerResponse, status: number, payload: unknown): void {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json')
  res.end(JSON.stringify(payload))
}

async function readRequestBody(req: IncomingMessage): Promise<Buffer> {
  const chunks: Buffer[] = []
  for await (const chunk of req) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
  }
  return Buffer.concat(chunks)
}

/**
 * Dev-only: keep login/API traffic on VITE_DEV_PROXY_TARGET, but serve
 * 应用管理 from a local appcatalogd (SQLite catalog) with the main API envelope.
 */
function proxyAppCatalogDirect(target: string): Plugin {
  const base = target.replace(/\/+$/, '')
  return {
    name: 'proxy-app-catalog-direct',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const url = req.url || ''
        const pathname = url.split('?')[0]
        const mapped = mapAppCatalogPath(pathname, req.method)
        if (!mapped || !req.method) {
          next()
          return
        }
        try {
          const method = req.method.toUpperCase()
          const hasBody = method !== 'GET' && method !== 'HEAD' && method !== 'DELETE'
          const body = hasBody ? await readRequestBody(req) : undefined
          const upstream = await fetch(`${base}${mapped}`, {
            method,
            headers: hasBody ? { 'Content-Type': req.headers['content-type'] || 'application/json' } : undefined,
            body,
            signal: AbortSignal.timeout(90000),
          })
          const raw = await upstream.text()
          let parsed: unknown = null
          if (raw) {
            try {
              parsed = JSON.parse(raw)
            } catch {
              parsed = { error: raw }
            }
          }
          if (mapped === '/health') {
            const health = (parsed && typeof parsed === 'object' ? parsed : {}) as Record<string, unknown>
            sendJSON(res, 200, {
              code: 0,
              message: 'success',
              data: {
                enabled: upstream.ok,
                reason: upstream.ok ? undefined : 'APP_CATALOG_AGENT_UNAVAILABLE',
                base_url: base,
                status: health.status,
                version: health.version,
                uptime_seconds: health.uptime_seconds,
              },
            })
            return
          }
          if (!upstream.ok) {
            const err = (parsed && typeof parsed === 'object' ? parsed : {}) as { error?: string }
            sendJSON(res, upstream.status, {
              code: upstream.status,
              message: err.error || raw || `app catalog HTTP ${upstream.status}`,
            })
            return
          }
          sendJSON(res, 200, { code: 0, message: 'success', data: parsed })
        } catch (error) {
          sendJSON(res, 503, {
            code: 503,
            message: error instanceof Error ? error.message : 'app catalog service is unavailable',
            reason: 'APP_CATALOG_AGENT_UNAVAILABLE',
            data: {
              enabled: false,
              reason: 'APP_CATALOG_AGENT_UNAVAILABLE',
              base_url: base,
            },
          })
        }
      })
    },
  }
}

function injectPublicSettings(backendUrl: string): Plugin {
  return {
    name: 'inject-public-settings',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        try {
          const response = await fetch(`${backendUrl}/api/v1/settings/public`, {
            signal: AbortSignal.timeout(2000)
          })
          if (response.ok) {
            const data = await response.json()
            if (data.code === 0 && data.data) {
              const script = `<script>window.__APP_CONFIG__=${JSON.stringify(data.data)};</script>`
              return injectBranding(html, data.data).replace('</head>', `${script}\n</head>`)
            }
          }
        } catch (e) {
          console.warn('[vite] 无法获取公开配置，将回退到 API 调用:', (e as Error).message)
        }
        return html
      }
    }
  }
}

export default defineConfig(({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, process.cwd(), '')
  const backendUrl = env.VITE_DEV_PROXY_TARGET || 'http://localhost:8080'
  const devPort = Number(env.VITE_DEV_PORT || 3000)
  const appCatalogDirect = (process.env.VITE_APP_CATALOG_DIRECT || env.VITE_APP_CATALOG_DIRECT || '').trim()

  return {
    plugins: [
      vue(),
      ...(appCatalogDirect ? [proxyAppCatalogDirect(appCatalogDirect)] : []),
      injectPublicSettings(backendUrl)
    ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      'vue-demi': resolve(__dirname, 'src/shims/vue-demi.ts'),
      // 使用 vue-i18n 运行时版本，避免 CSP unsafe-eval 问题
      'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'
    }
  },
    define: {
    // 启用 vue-i18n JIT 编译，在 CSP 环境下处理消息插值
    // JIT 编译器生成 AST 对象而非 JS 代码，无需 unsafe-eval
      __INTLIFY_JIT_COMPILATION__: true
    },
    optimizeDeps: {
      // The dependency tree is already installed from the lockfile. Avoid a
      // broad filesystem scan on Windows that can terminate the dev server.
      noDiscovery: true,
      // CJS packages still need explicit pre-bundling when dependency
      // discovery is disabled; otherwise the browser receives the raw UMD
      // file and named/default imports fail during route loading.
      include: ['file-saver', 'qrcode']
    },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        /**
         * 手动分包配置
         * 分离第三方库并按功能合并应用代码，避免循环依赖
         */
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // Vue 核心库
            if (
              id.includes('/vue/') ||
              id.includes('/vue-router/') ||
              id.includes('/pinia/') ||
              id.includes('/@vue/')
            ) {
              return 'vendor-vue'
            }

            // UI 工具库（较大，单独分离）
            if (id.includes('/@vueuse/') || id.includes('/xlsx/')) {
              return 'vendor-ui'
            }

            // 图表库
            if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
              return 'vendor-chart'
            }

            // 国际化
            if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
              return 'vendor-i18n'
            }

            // Stripe 仅在支付流程中按需加载，避免进入首页公共依赖。
            if (id.includes('/@stripe/stripe-js/')) {
              return 'vendor-stripe'
            }

            // 其他小型第三方库合并
            return 'vendor-misc'
          }

          // 应用代码：按入口点自动分包，不手动干预
          // 这样可以避免循环依赖，同时保持合理的 chunk 数量
        }
      }
    }
  },
    server: {
      host: '0.0.0.0',
      port: devPort,
      proxy: {
        '/api': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1': {
          target: backendUrl,
          changeOrigin: true
        },
        '/setup': {
          target: backendUrl,
          changeOrigin: true
        }
      }
    }
  }
})
