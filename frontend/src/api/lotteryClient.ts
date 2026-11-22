/**
 * lotteryd HTTP 客户端：两个独立组件（用户面板 / 管理面板）都只通过这里与
 * 独立后端服务通信，与 Sub2API 主服务零耦合。
 */
import axios, { type AxiosInstance } from 'axios'

export interface LotteryClientOptions {
  /** lotteryd 服务地址，如 http://127.0.0.1:18100 */
  baseURL: string
  /** WebSocket 地址（ws(s)://...，含完整路径）；缺省时由 baseURL 推导 */
  wsURL?: string
  /** 返回当前用户的 Sub2API JWT（含用户与管理员 token）。 */
  getToken: () => string | null
}

export interface LotteryClient {
  http: AxiosInstance
  baseURL: string
  /** 推导后的 WS 地址：显式配置优先；相对 baseURL → 同源路径；绝对 http(s) → 对应 ws(s)。 */
  wsURL: string
}

export function deriveWsURL(baseURL: string, explicit?: string): string {
  if (explicit) return explicit
  const base = baseURL.replace(/\/+$/, '')
  if (base.startsWith('/')) {
    // 同源相对路径（如 /lotteryd）→ ws(s)://当前域名 + 路径
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    return `${proto}://${location.host}${base}`
  }
  // 绝对地址 http(s)://host[:port] → ws(s)://host[:port]
  return base.replace(/^http/, 'ws')
}

export function createLotteryClient(options: LotteryClientOptions): LotteryClient {
  const http = axios.create({
    baseURL: options.baseURL.replace(/\/+$/, ''),
    timeout: 15000
  })
  http.interceptors.request.use((config) => {
    const token = options.getToken()
    if (token) config.headers.Authorization = `Bearer ${token}`
    return config
  })
  http.interceptors.response.use(
    (resp) => resp,
    (error) => {
      const message =
        error?.response?.data?.message || error?.message || 'lotteryd request failed'
      return Promise.reject(new Error(message))
    }
  )
  return { http, baseURL: options.baseURL, wsURL: deriveWsURL(options.baseURL, options.wsURL) }
}

export interface Envelope<T> {
  code: number
  message?: string
  data?: T
}

export async function unwrap<T>(promise: Promise<{ data: Envelope<T> }>): Promise<T> {
  const { data } = await promise
  if (data.code !== 0) throw new Error(data.message || 'request failed')
  return data.data as T
}

/** Sub2API 并入用：按环境变量与主站登录态构建 lotteryd 客户端。 */
export function createDefaultLotteryClient(): LotteryClient {
  return createLotteryClient({
    baseURL: import.meta.env.VITE_LOTTERYD_BASE_URL || '/lotteryd',
    wsURL: import.meta.env.VITE_LOTTERYD_WS_URL || '',
    getToken: () => localStorage.getItem('auth_token')
  })
}
