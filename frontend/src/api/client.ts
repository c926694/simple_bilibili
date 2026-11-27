import type { ApiEnvelope, TokenPair } from '../types/user'

export const ERRCODE_SUCCESS = 0
export const ERRCODE_UNAUTHORIZED = 10002

const ACCESS_TOKEN_KEY = 'simple_bilibili_access_token'
const REFRESH_TOKEN_KEY = 'simple_bilibili_refresh_token'

export class ApiError extends Error {
  readonly code: number

  constructor(code: number, msg: string) {
    super(msg)
    this.name = 'ApiError'
    this.code = code
  }
}

export const tokenStorage = {
  getAccessToken(): string | null {
    return localStorage.getItem(ACCESS_TOKEN_KEY)
  },
  getRefreshToken(): string | null {
    return localStorage.getItem(REFRESH_TOKEN_KEY)
  },
  setTokens(pair: Pick<TokenPair, 'access_token' | 'refresh_token'>): void {
    localStorage.setItem(ACCESS_TOKEN_KEY, pair.access_token)
    localStorage.setItem(REFRESH_TOKEN_KEY, pair.refresh_token)
  },
  clear(): void {
    localStorage.removeItem(ACCESS_TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
  },
}

let unauthorizedListener: (() => void) | null = null
let refreshingPromise: Promise<boolean> | null = null
let pendingRequestCount = 0
const pendingListeners = new Set<(count: number) => void>()

export function subscribePendingRequests(listener: (count: number) => void): () => void {
  pendingListeners.add(listener)
  listener(pendingRequestCount)
  return () => {
    pendingListeners.delete(listener)
  }
}

function notifyPendingRequests(delta: 1 | -1): void {
  pendingRequestCount = Math.max(0, pendingRequestCount + delta)
  for (const listener of pendingListeners) {
    listener(pendingRequestCount)
  }
}

export function setUnauthorizedListener(listener: (() => void) | null): void {
  unauthorizedListener = listener
}

async function performRefresh(): Promise<boolean> {
  const refreshToken = tokenStorage.getRefreshToken()
  if (!refreshToken) {
    return false
  }
  try {
    const res = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })
    const body = (await res.json()) as ApiEnvelope<TokenPair | null>
    if (body.code === ERRCODE_SUCCESS && body.data) {
      tokenStorage.setTokens(body.data)
      return true
    }
  } catch {
    // 刷新请求网络异常视为刷新失败
  }
  tokenStorage.clear()
  unauthorizedListener?.()
  return false
}

// 并发请求同时触发凭证过期时复用同一次刷新请求，防止旧 Refresh Token 轮换冲突
function ensureRefreshed(): Promise<boolean> {
  if (!refreshingPromise) {
    refreshingPromise = performRefresh().finally(() => {
      refreshingPromise = null
    })
  }
  return refreshingPromise
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  body?: unknown
  auth?: boolean
  skipRefresh?: boolean
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, auth = true, skipRefresh = false } = options

  const send = async (): Promise<ApiEnvelope<T>> => {
    const headers: Record<string, string> = {}
    const isFormData = typeof FormData !== 'undefined' && body instanceof FormData
    if (body !== undefined && !isFormData) {
      headers['Content-Type'] = 'application/json'
    }
    if (auth) {
      const token = tokenStorage.getAccessToken()
      if (token) {
        headers.Authorization = `Bearer ${token}`
      }
    }

    const res = await fetch(path, {
      method,
      headers,
      body: body === undefined ? undefined : isFormData ? body : JSON.stringify(body),
    })
    return (await res.json()) as ApiEnvelope<T>
  }

  notifyPendingRequests(1)
  try {
    let envelope = await send()

    if (envelope.code === ERRCODE_UNAUTHORIZED && auth && !skipRefresh && tokenStorage.getRefreshToken()) {
      const refreshed = await ensureRefreshed()
      if (refreshed) {
        envelope = await send()
      }
    }

    if (envelope.code !== ERRCODE_SUCCESS) {
      throw new ApiError(envelope.code, envelope.msg || '请求失败')
    }

    return envelope.data
  } finally {
    notifyPendingRequests(-1)
  }
}
