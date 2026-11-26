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
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(jsonResponse({ code: 0, data: BASE_USER, msg: 'ok' }))
      .mockResolvedValueOnce(
        jsonResponse({
          code: 0,
          data: {
            upload_url: 'http://127.0.0.1:8088/bilibili/avatars/2001/999.png?X-Amz-Signature=abc',
            object_key: 'avatars/2001/999.png',
            public_url: '/storage/avatars/2001/999.png',
            headers: { 'Content-Type': 'image/png' },
            expires_in: 300,
          },
          msg: 'ok',
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 200, headers: { ETag: '"etag-1"' } }))
      .mockResolvedValueOnce(
        jsonResponse({
          code: 0,
          data: {
            ...BASE_USER,
            nickname: '保存后的昵称',
            avatar_url: '/storage/avatars/2001/999.png',
          },
          msg: 'ok',
        }),
      )

    render(
      <MemoryRouter>
        <AuthProvider>
          <AccountPage />
        </AuthProvider>
      </MemoryRouter>,
    )

    await screen.findByDisplayValue('初始昵称')
    expect(fetchSpy).toHaveBeenCalledTimes(1)

    const file = new File(['png-bytes'], 'new-avatar.png', { type: 'image/png' })
    const fileInput = screen.getByLabelText('选择本地头像文件')
    fireEvent.change(fileInput, { target: { files: [file] } })

    expect(await screen.findByText(/已选择：new-avatar\.png（点击保存后上传生效）/)).toBeInTheDocument()
    expect(fetchSpy).toHaveBeenCalledTimes(1)

    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: '保存后的昵称' } })
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(screen.getByText('个人资料保存成功')).toBeInTheDocument()
    })

    expect(fetchSpy).toHaveBeenCalledTimes(4)
    expect(fetchSpy.mock.calls[1][0]).toBe('/api/v1/uploads/image-presign')
    expect(fetchSpy.mock.calls[2][0]).toBe(
      'http://127.0.0.1:8088/bilibili/avatars/2001/999.png?X-Amz-Signature=abc',
    )
    expect((fetchSpy.mock.calls[2][1] as RequestInit).method).toBe('PUT')
    expect(fetchSpy.mock.calls[3][0]).toBe('/api/v1/users/me')
    const updateBody = JSON.parse((fetchSpy.mock.calls[3][1] as RequestInit).body as string)
    expect(updateBody.avatar_url).toBe('/storage/avatars/2001/999.png')
    expect(updateBody.nickname).toBe('保存后的昵称')
  })
})
