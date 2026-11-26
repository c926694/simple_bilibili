import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
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
})
