import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tokenStorage } from '../../api/client'
import { AuthProvider } from '../../context/AuthContext'
import { AccountPage } from './AccountPage'

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

const BASE_USER = {
  id: '2001',
  username: 'bili_tester',
  nickname: '初始昵称',
  email: null,
  phone: null,
  avatar_url: '/avatars/default.webp',
  bio: '',
  coin_balance: 10,
  follower_count: 0,
  following_count: 0,
  created_at: '2026-11-26T23:00:00Z',
}

describe('AccountPage 头像选择预览与点击保存后预签名直传 MinIO', () => {
  beforeEach(() => {
    localStorage.clear()
    tokenStorage.setTokens({ access_token: 'acc', refresh_token: 'ref' })
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:local-avatar-preview')
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('选择本地头像仅展示本地预览，点击保存后申请预签名直传 MinIO 并保存资料', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
      const url = String(input)
      const method = (init?.method || 'GET').toUpperCase()
      if (url === '/api/v1/users/me' && method === 'GET') {
        return jsonResponse({ code: 0, data: BASE_USER, msg: 'ok' })
      }
      if (url === '/api/v1/users/me/checkin' && method === 'GET') {
        return jsonResponse({
          code: 0,
          data: {
            checked_in_today: false,
            month: '2026-11',
            month_days: 30,
            streak_days: 0,
            checked_dates: [],
            coin_balance: 10,
          },
          msg: 'ok',
        })
      }
      if (url === '/api/v1/uploads/image-presign' && method === 'POST') {
        return jsonResponse({
          code: 0,
          data: {
            upload_url: 'http://127.0.0.1:8088/bilibili/tmp/avatars/2001/999.png?X-Amz-Signature=abc',
            object_key: 'tmp/avatars/2001/999.png',
            public_url: '/storage/tmp/avatars/2001/999.png',
            headers: { 'Content-Type': 'image/png' },
            expires_in: 300,
          },
          msg: 'ok',
        })
      }
      if (url.startsWith('http://127.0.0.1:8088/bilibili/tmp/avatars/') && method === 'PUT') {
        return new Response(null, { status: 200, headers: { ETag: '"etag-1"' } })
      }
      if (url === '/api/v1/users/me' && method === 'PUT') {
        return jsonResponse({
          code: 0,
          data: {
            ...BASE_USER,
            nickname: '保存后的昵称',
            avatar_url: '/storage/avatars/2001/999.png',
          },
          msg: 'ok',
        })
      }
      return jsonResponse({ code: 10004, data: null, msg: 'not found' })
    })

    render(
      <MemoryRouter>
        <AuthProvider>
          <AccountPage />
        </AuthProvider>
      </MemoryRouter>,
    )

    await screen.findByDisplayValue('初始昵称')
    expect(await screen.findByText(/2026-11 已累计签到 0 \/ 30 天/)).toBeInTheDocument()

    const file = new File(['png-bytes'], 'new-avatar.png', { type: 'image/png' })
    const fileInput = screen.getByLabelText('选择本地头像文件')
    fireEvent.change(fileInput, { target: { files: [file] } })

    expect(await screen.findByText(/已选择：new-avatar\.png（点击保存后上传生效）/)).toBeInTheDocument()
    const beforeSaveUrls = fetchSpy.mock.calls.map((c) => String(c[0]))
    expect(beforeSaveUrls).not.toContain('/api/v1/uploads/image-presign')

    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: '保存后的昵称' } })
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(screen.getByText('个人资料保存成功')).toBeInTheDocument()
    })

    const presignIdx = fetchSpy.mock.calls.findIndex(
      (c) => String(c[0]) === '/api/v1/uploads/image-presign',
    )
    const putIdx = fetchSpy.mock.calls.findIndex(
      (c) =>
        String(c[0]) ===
        'http://127.0.0.1:8088/bilibili/tmp/avatars/2001/999.png?X-Amz-Signature=abc',
    )
    const updateIdx = fetchSpy.mock.calls.findIndex(
      (c) => String(c[0]) === '/api/v1/users/me' && (c[1] as RequestInit | undefined)?.method === 'PUT',
    )
    expect(presignIdx).toBeGreaterThan(-1)
    expect(putIdx).toBeGreaterThan(presignIdx)
    expect(updateIdx).toBeGreaterThan(putIdx)
    const updateBody = JSON.parse((fetchSpy.mock.calls[updateIdx][1] as RequestInit).body as string)
    expect(updateBody.avatar_url).toBe('/storage/tmp/avatars/2001/999.png')
    expect(updateBody.nickname).toBe('保存后的昵称')
  })
})
