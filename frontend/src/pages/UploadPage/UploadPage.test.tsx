import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tokenStorage } from '../../api/client'
import { AuthProvider } from '../../context/AuthContext'
import { UploadPage } from './UploadPage'

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

const BASE_USER = {
  id: '2001',
  username: 'bili_tester',
  nickname: '测试作者',
  email: null,
  phone: null,
  avatar_url: '/avatars/default.webp',
  bio: '',
  coin_balance: 10,
  follower_count: 0,
  following_count: 0,
  created_at: '2026-11-26T23:00:00Z',
}

const MY_VIDEO = {
  id: '3001',
  bv_id: 'BV2X4TEST001',
  title: '既有测试视频',
  description: '简介',
  cover_url: '/storage/covers/2001/old.png',
  duration_sec: 30,
  status: 3,
  created_at: '2026-11-27T10:00:00Z',
  category: { id: 1, name: '动画' },
  tags: ['测试'],
  parts: [],
  stat: {
    play_count: 0,
    danmaku_count: 0,
    like_count: 0,
    coin_count: 0,
    favorite_count: 0,
    share_count: 0,
  },
}

describe('UploadPage 封面本地预览与提交时上传', () => {
  beforeEach(() => {
    localStorage.clear()
    tokenStorage.setTokens({ access_token: 'acc', refresh_token: 'ref' })
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:local-cover-preview')
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('投稿页选择封面后仅本地预览不发上传请求', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
      const url = String(input)
      const method = (init?.method || 'GET').toUpperCase()
      if (url === '/api/v1/users/me' && method === 'GET') {
        return jsonResponse({ code: 0, data: BASE_USER, msg: 'ok' })
      }
      if (url === '/api/v1/categories' && method === 'GET') {
        return jsonResponse({ code: 0, data: [{ id: 1, name: '动画', sort_order: 1 }], msg: 'ok' })
      }
      return jsonResponse({ code: 10004, data: null, msg: 'not found' })
    })

    render(
      <MemoryRouter initialEntries={['/upload']}>
        <AuthProvider>
          <UploadPage />
        </AuthProvider>
      </MemoryRouter>,
    )

    await screen.findByText('创作中心 · 视频投稿')
    const file = new File(['png-bytes'], 'cover.png', { type: 'image/png' })
    fireEvent.change(screen.getByLabelText('选择视频封面图片'), { target: { files: [file] } })

    expect(
      await screen.findByText(/已选择封面：cover\.png（点击立即投稿时上传生效）/),
    ).toBeInTheDocument()
    const calledUrls = fetchSpy.mock.calls.map((c) => String(c[0]))
    expect(calledUrls).not.toContain('/api/v1/uploads/image-presign')
  })

  it('稿件管理编辑封面选择后不上传，点击保存修改才直传临时路径并提交', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation(async (input, init) => {
      const url = String(input)
      const method = (init?.method || 'GET').toUpperCase()
      if (url === '/api/v1/users/me' && method === 'GET') {
        return jsonResponse({ code: 0, data: BASE_USER, msg: 'ok' })
      }
      if (url === '/api/v1/categories' && method === 'GET') {
        return jsonResponse({ code: 0, data: [{ id: 1, name: '动画', sort_order: 1 }], msg: 'ok' })
      }
      if (url.startsWith('/api/v1/videos/mine') && method === 'GET') {
        return jsonResponse({ code: 0, data: { items: [MY_VIDEO], total: 1 }, msg: 'ok' })
      }
      if (url === '/api/v1/uploads/image-presign' && method === 'POST') {
        return jsonResponse({
          code: 0,
          data: {
            upload_url:
              'http://127.0.0.1:8088/bilibili/tmp/covers/2001/888.png?X-Amz-Signature=xyz',
            object_key: 'tmp/covers/2001/888.png',
            public_url: '/storage/tmp/covers/2001/888.png',
            headers: { 'Content-Type': 'image/png' },
            expires_in: 300,
          },
          msg: 'ok',
        })
      }
      if (url.startsWith('http://127.0.0.1:8088/bilibili/tmp/covers/') && method === 'PUT') {
        return new Response(null, { status: 200, headers: { ETag: '"etag-cover"' } })
      }
      if (url === `/api/v1/videos/${MY_VIDEO.bv_id}` && method === 'PUT') {
        return jsonResponse({
          code: 0,
          data: { ...MY_VIDEO, cover_url: '/storage/covers/2001/888.png' },
          msg: 'ok',
        })
      }
      return jsonResponse({ code: 10004, data: null, msg: 'not found' })
    })

    render(
      <MemoryRouter initialEntries={['/upload?tab=manage']}>
        <AuthProvider>
          <UploadPage />
        </AuthProvider>
      </MemoryRouter>,
    )

    await screen.findByText('既有测试视频')
    fireEvent.click(screen.getByRole('button', { name: '编辑' }))

    const file = new File(['png-bytes'], 'next-cover.png', { type: 'image/png' })
    fireEvent.change(screen.getByLabelText('选择编辑视频封面图片'), {
      target: { files: [file] },
    })

    expect(
      await screen.findByText(/已选择新封面：next-cover\.png（点击保存修改生效）/),
    ).toBeInTheDocument()
    expect(fetchSpy.mock.calls.map((c) => String(c[0]))).not.toContain(
      '/api/v1/uploads/image-presign',
    )

    fireEvent.click(screen.getByRole('button', { name: '保存修改' }))

    await waitFor(() => {
      expect(screen.getByText('视频信息已更新')).toBeInTheDocument()
    })

    const presignIdx = fetchSpy.mock.calls.findIndex(
      (c) => String(c[0]) === '/api/v1/uploads/image-presign',
    )
    const putIdx = fetchSpy.mock.calls.findIndex(
      (c) =>
        String(c[0]) ===
        'http://127.0.0.1:8088/bilibili/tmp/covers/2001/888.png?X-Amz-Signature=xyz',
    )
    const updateIdx = fetchSpy.mock.calls.findIndex(
      (c) =>
        String(c[0]) === `/api/v1/videos/${MY_VIDEO.bv_id}` &&
        (c[1] as RequestInit | undefined)?.method === 'PUT',
    )
    expect(presignIdx).toBeGreaterThan(-1)
    expect(putIdx).toBeGreaterThan(presignIdx)
    expect(updateIdx).toBeGreaterThan(putIdx)
    const updateBody = JSON.parse((fetchSpy.mock.calls[updateIdx][1] as RequestInit).body as string)
    expect(updateBody.cover_url).toBe('/storage/tmp/covers/2001/888.png')
  })
})
