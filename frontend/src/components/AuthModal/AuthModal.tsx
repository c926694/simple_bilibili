import { useEffect, useState, type FormEvent } from 'react'
import { ApiError } from '../../api/client'
import { useAuth, type AuthModalTab } from '../../context/AuthContext'
import './AuthModal.css'

const USERNAME_PATTERN = /^[a-zA-Z0-9_]{3,32}$/

export function AuthModal() {
  const { isAuthModalOpen, authModalTab, closeAuthModal, login, register } = useAuth()
  const [tab, setTab] = useState<AuthModalTab>(authModalTab)
  const [passwordFocused, setPasswordFocused] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  const [account, setAccount] = useState('')
  const [loginPassword, setLoginPassword] = useState('')

  const [username, setUsername] = useState('')
  const [nickname, setNickname] = useState('')
  const [registerPassword, setRegisterPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')

  useEffect(() => {
    setTab(authModalTab)
    setErrorMsg('')
  }, [authModalTab, isAuthModalOpen])

  if (!isAuthModalOpen) {
    return null
  }

  const switchTab = (next: AuthModalTab) => {
    setTab(next)
    setErrorMsg('')
  }

  const handleLoginSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setErrorMsg('')
    const trimmedAccount = account.trim()
    if (!trimmedAccount) {
      setErrorMsg('请输入用户名、邮箱或手机号')
      return
    }
    if (!loginPassword) {
      setErrorMsg('请输入登录密码')
      return
    }

    setSubmitting(true)
    try {
      await login({ account: trimmedAccount, password: loginPassword })
      setLoginPassword('')
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorMsg(err.message)
      } else {
        setErrorMsg('网络异常，请稍后再试')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleRegisterSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setErrorMsg('')
    const trimmedUsername = username.trim()
    if (!USERNAME_PATTERN.test(trimmedUsername)) {
      setErrorMsg('用户名需为 3~32 位字母、数字或下划线')
      return
    }
    if (registerPassword.length < 6 || registerPassword.length > 64) {
      setErrorMsg('密码长度需在 6~64 位之间')
      return
    }
    if (registerPassword !== confirmPassword) {
      setErrorMsg('两次输入的密码不一致')
      return
    }

    setSubmitting(true)
    try {
      await register({
        username: trimmedUsername,
        password: registerPassword,
        nickname: nickname.trim() || undefined,
        email: email.trim() || undefined,
        phone: phone.trim() || undefined,
      })
      setRegisterPassword('')
      setConfirmPassword('')
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorMsg(err.message)
      } else {
        setErrorMsg('网络异常，请稍后再试')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-modal-mask" role="dialog" aria-modal="true" aria-label="账号登录与注册">
      <div className="auth-modal">
        <button
          type="button"
          className="auth-modal__close"
          onClick={closeAuthModal}
          aria-label="关闭弹窗"
        >
          ×
        </button>

        <div className="auth-modal__mascot" aria-hidden="true">
          <svg width="76" height="58" viewBox="0 0 76 58" fill="none">
            <path
              d="M24 6L32 14M52 6L44 14"
              stroke="#00AEEC"
              strokeWidth="3.5"
              strokeLinecap="round"
            />
            <rect
              x="10"
              y="14"
              width="56"
              height="38"
              rx="10"
              fill="#F6F7F8"
              stroke="#00AEEC"
              strokeWidth="3"
            />
            {passwordFocused ? (
              <>
                <path
                  d="M22 33L31 33M45 33L54 33"
                  stroke="#FB7299"
                  strokeWidth="3.2"
                  strokeLinecap="round"
                />
                <circle cx="24" cy="40" r="3" fill="#FB7299" fillOpacity="0.45" />
                <circle cx="52" cy="40" r="3" fill="#FB7299" fillOpacity="0.45" />
              </>
            ) : (
              <>
                <circle cx="28" cy="31" r="3.5" fill="#18191C" />
                <circle cx="48" cy="31" r="3.5" fill="#18191C" />
                <path
                  d="M32 40Q38 45 44 40"
                  stroke="#FB7299"
                  strokeWidth="2.8"
                  strokeLinecap="round"
                />
              </>
            )}
          </svg>
        </div>

        <div className="auth-modal__tabs">
          <button
            type="button"
            className={`auth-modal__tab ${tab === 'login' ? 'auth-modal__tab--active' : ''}`}
            onClick={() => switchTab('login')}
          >
            密码登录
          </button>
          <span className="auth-modal__divider" />
          <button
            type="button"
            className={`auth-modal__tab ${tab === 'register' ? 'auth-modal__tab--active' : ''}`}
            onClick={() => switchTab('register')}
          >
            注册账号
          </button>
        </div>

        {tab === 'login' ? (
          <form className="auth-form" onSubmit={handleLoginSubmit} noValidate>
            <div className="auth-form__box">
              <label className="auth-form__row">
                <span className="auth-form__label">账号</span>
                <input
                  type="text"
                  className="auth-form__input"
                  placeholder="请输入用户名 / 邮箱 / 手机号"
                  value={account}
                  onChange={(e) => setAccount(e.target.value)}
                  autoComplete="username"
                />
              </label>
              <label className="auth-form__row">
                <span className="auth-form__label">密码</span>
                <input
                  type="password"
                  className="auth-form__input"
                  placeholder="请输入密码"
                  value={loginPassword}
                  onChange={(e) => setLoginPassword(e.target.value)}
                  onFocus={() => setPasswordFocused(true)}
                  onBlur={() => setPasswordFocused(false)}
                  autoComplete="current-password"
                />
              </label>
            </div>

            <div className="auth-form__error" role="alert">
              {errorMsg}
            </div>

            <div className="auth-form__actions">
              <button
                type="button"
                className="auth-btn auth-btn--secondary"
                onClick={() => switchTab('register')}
                disabled={submitting}
              >
                注册
              </button>
              <button type="submit" className="auth-btn auth-btn--primary" disabled={submitting}>
                {submitting ? '登录中...' : '登录'}
              </button>
            </div>
          </form>
        ) : (
          <form className="auth-form" onSubmit={handleRegisterSubmit} noValidate>
            <div className="auth-form__box">
              <label className="auth-form__row">
                <span className="auth-form__label">用户名</span>
                <input
                  type="text"
                  className="auth-form__input"
                  placeholder="3~32位字母、数字或下划线"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="username"
                />
              </label>
              <label className="auth-form__row">
                <span className="auth-form__label">昵称</span>
                <input
                  type="text"
                  className="auth-form__input"
                  placeholder="选填，留空默认使用用户名"
                  value={nickname}
                  onChange={(e) => setNickname(e.target.value)}
                />
              </label>
              <label className="auth-form__row">
                <span className="auth-form__label">密码</span>
                <input
                  type="password"
                  className="auth-form__input"
                  placeholder="6~64位密码"
                  value={registerPassword}
                  onChange={(e) => setRegisterPassword(e.target.value)}
                  onFocus={() => setPasswordFocused(true)}
                  onBlur={() => setPasswordFocused(false)}
                  autoComplete="new-password"
                />
              </label>
              <label className="auth-form__row">
                <span className="auth-form__label">确认密码</span>
                <input
                  type="password"
                  className="auth-form__input"
                  placeholder="请再次输入密码"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  onFocus={() => setPasswordFocused(true)}
                  onBlur={() => setPasswordFocused(false)}
                  autoComplete="new-password"
                />
              </label>
              <label className="auth-form__row">
                <span className="auth-form__label">邮箱</span>
                <input
                  type="email"
                  className="auth-form__input"
                  placeholder="选填，用于账号登录"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  autoComplete="email"
                />
              </label>
              <label className="auth-form__row">
                <span className="auth-form__label">手机号</span>
                <input
                  type="tel"
                  className="auth-form__input"
                  placeholder="选填，11位手机号"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  autoComplete="tel"
                />
              </label>
            </div>

            <div className="auth-form__error" role="alert">
              {errorMsg}
            </div>

            <div className="auth-form__actions">
              <button
                type="button"
                className="auth-btn auth-btn--secondary"
                onClick={() => switchTab('login')}
                disabled={submitting}
              >
                返回登录
              </button>
              <button type="submit" className="auth-btn auth-btn--primary" disabled={submitting}>
                {submitting ? '注册中...' : '立即注册'}
              </button>
            </div>
          </form>
        )}

        <div className="auth-modal__tip">注册即赠 10 枚初始代币，用于视频投币互动体验</div>
      </div>
    </div>
  )
}
