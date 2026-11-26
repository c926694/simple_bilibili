import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiRequest, setUnauthorizedListener, tokenStorage } from './client'

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('apiRequest 统一三字段协议与自动刷新', () => {
  beforeEach(() => {
    localStorage.clear()
    setUnauthorizedListener(null)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('响应 code 为 0 时返回 data 内容', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      jsonResponse({ code: 0, data: { status: 'ok' }, msg: 'ok' }),
    )

    const result = await apiRequest<{ status: string }>('/healthz', { auth: false })
    expect(result).toEqual({ status: 'ok' })
  })

  it('响应 code 非 0 时抛出携带 code 和 msg 的 ApiError', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      jsonResponse({ code: 11004, data: null, msg: '账号或密码错误' }),
    )

    await expect(apiRequest('/api/v1/auth/login', { method: 'POST', auth: false })).rejects.toEqual(
      expect.objectContaining<Partial<ApiError>>({
        name: 'ApiError',
        code: 11004,
        message: '账号或密码错误',
      }),
    )
  })

  it('遇到 10002 时自动调用刷新接口并重放原请求', async () => {
    tokenStorage.setTokens({ access_token: 'old-access', refresh_token: 'old-refresh' })
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse({ code: 10002, data: null, msg: '未登录或凭证无效' }))
      .mockResolvedValueOnce(
        jsonResponse({
          code: 0,
          data: {
            access_token: 'new-access',
            refresh_token: 'new-refresh',
            access_expires_in: 900,
            refresh_expires_in: 604800,
          },
          msg: 'ok',
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({ code: 0, data: { id: '100', nickname: '小电视' }, msg: 'ok' }),
      )

    const profile = await apiRequest<{ id: string; nickname: string }>('/api/v1/users/me')

    expect(profile).toEqual({ id: '100', nickname: '小电视' })
    expect(tokenStorage.getAccessToken()).toBe('new-access')
    expect(tokenStorage.getRefreshToken()).toBe('new-refresh')
    expect(fetchSpy).toHaveBeenCalledTimes(3)
    const retryInit = fetchSpy.mock.calls[2][1] as RequestInit
    expect((retryInit.headers as Record<string, string>).Authorization).toBe('Bearer new-access')
  })

  it('刷新失败时清空本地凭证并触发未认证回调', async () => {
    tokenStorage.setTokens({ access_token: 'old-access', refresh_token: 'expired-refresh' })
    const onUnauthorized = vi.fn()
    setUnauthorizedListener(onUnauthorized)

    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse({ code: 10002, data: null, msg: '未登录或凭证无效' }))
      .mockResolvedValueOnce(jsonResponse({ code: 11008, data: null, msg: '刷新凭证无效或已过期' }))

    await expect(apiRequest('/api/v1/users/me')).rejects.toMatchObject({ code: 10002 })
    expect(tokenStorage.getAccessToken()).toBeNull()
    expect(tokenStorage.getRefreshToken()).toBeNull()
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })

  it('发送 FormData 上传时不覆写 Content-Type 请求头', async () => {
    tokenStorage.setTokens({ access_token: 'acc-token', refresh_token: 'ref-token' })
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(
        jsonResponse({ code: 0, data: { id: '1', avatar_url: '/storage/avatars/1/a.png' }, msg: 'ok' }),
      )

    const form = new FormData()
    form.append('file', new Blob(['img'], { type: 'image/png' }), 'a.png')

    const res = await apiRequest<{ id: string; avatar_url: string }>('/api/v1/users/me/avatar', {
      method: 'POST',
      body: form,
    })

    expect(res.avatar_url).toBe('/storage/avatars/1/a.png')
    const init = fetchSpy.mock.calls[0][1] as RequestInit
    expect(init.body).toBe(form)
    expect((init.headers as Record<string, string>)['Content-Type']).toBeUndefined()
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer acc-token')
  })
})
