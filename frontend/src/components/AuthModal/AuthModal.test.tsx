import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider, useAuth } from '../../context/AuthContext'
import { AuthModal } from './AuthModal'

function ModalHost() {
  const { openAuthModal } = useAuth()
  return (
    <>
      <button type="button" onClick={() => openAuthModal('login')}>
        打开登录窗
      </button>
      <AuthModal />
    </>
  )
}

describe('AuthModal 登录注册弹窗交互', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('支持登录与注册页签切换及入口表单校验', async () => {
    render(
      <AuthProvider>
        <ModalHost />
      </AuthProvider>,
    )

    fireEvent.click(screen.getByText('打开登录窗'))
    expect(screen.getByRole('dialog', { name: '账号登录与注册' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '登录' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('请输入用户名、邮箱或手机号')

    fireEvent.click(screen.getByRole('button', { name: '注册账号' }))
    fireEvent.change(screen.getByPlaceholderText('3~32位字母、数字或下划线'), {
      target: { value: 'ab' },
    })
    fireEvent.click(screen.getByRole('button', { name: '立即注册' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('用户名需为 3~32 位字母、数字或下划线')
  })

  it('允许任意非空简单密码直接提交注册请求', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          code: 0,
          data: {
            access_token: 'acc',
            refresh_token: 'ref',
            access_expires_in: 900,
            refresh_expires_in: 604800,
            user: {
              id: '101',
              username: 'simple_user',
              nickname: 'simple_user',
              email: null,
              phone: null,
              avatar_url: '/avatars/default.webp',
              bio: '',
              coin_balance: 10,
              follower_count: 0,
              following_count: 0,
              created_at: '2026-11-26T23:00:00+08:00',
            },
          },
          msg: 'ok',
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )

    render(
      <AuthProvider>
        <ModalHost />
      </AuthProvider>,
    )

    fireEvent.click(screen.getByText('打开登录窗'))
    fireEvent.click(screen.getByRole('button', { name: '注册账号' }))
    fireEvent.change(screen.getByPlaceholderText('3~32位字母、数字或下划线'), {
      target: { value: 'simple_user' },
    })
    fireEvent.change(screen.getByPlaceholderText('请输入密码'), { target: { value: '1' } })
    fireEvent.change(screen.getByPlaceholderText('请再次输入密码'), { target: { value: '1' } })
    fireEvent.click(screen.getByRole('button', { name: '立即注册' }))

    await vi.waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledTimes(1)
    })
  })
})
