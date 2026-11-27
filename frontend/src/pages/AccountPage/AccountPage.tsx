import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../api/client'
import { uploadApi } from '../../api/upload'
import { userApi } from '../../api/user'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { CheckinPanel } from '../../components/CheckinPanel/CheckinPanel'
import { PRESET_AVATARS, UserAvatar } from '../../components/UserAvatar/UserAvatar'
import { useAuth } from '../../context/AuthContext'
import './AccountPage.css'

type AccountTab = 'profile' | 'security'
const MAX_AVATAR_BYTES = 5 * 1024 * 1024
const ALLOWED_AVATAR_TYPES = ['image/jpeg', 'image/png', 'image/webp', 'image/gif']

export function AccountPage() {
  const { user, initializing, openAuthModal, setUser, logout } = useAuth()
  const [activeTab, setActiveTab] = useState<AccountTab>('profile')
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  const [nickname, setNickname] = useState('')
  const [bio, setBio] = useState('')
  const [avatarUrl, setAvatarUrl] = useState('')
  const [pendingAvatarFile, setPendingAvatarFile] = useState<File | null>(null)
  const [pendingAvatarPreview, setPendingAvatarPreview] = useState<string | null>(null)
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [savingProfile, setSavingProfile] = useState(false)
  const [profileNotice, setProfileNotice] = useState<{ type: 'success' | 'error'; text: string } | null>(
    null,
  )

  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmNewPassword, setConfirmNewPassword] = useState('')
  const [savingPassword, setSavingPassword] = useState(false)
  const [passwordNotice, setPasswordNotice] = useState<{
    type: 'success' | 'error'
    text: string
  } | null>(null)

  useEffect(() => {
    if (!user) {
      return
    }
    setNickname(user.nickname)
    setBio(user.bio)
    setAvatarUrl(user.avatar_url)
    setEmail(user.email ?? '')
    setPhone(user.phone ?? '')
  }, [user])

  useEffect(() => {
    return () => {
      if (pendingAvatarPreview) {
        URL.revokeObjectURL(pendingAvatarPreview)
      }
    }
  }, [pendingAvatarPreview])

  const clearPendingAvatar = () => {
    setPendingAvatarFile(null)
    setPendingAvatarPreview((prev) => {
      if (prev) {
        URL.revokeObjectURL(prev)
      }
      return null
    })
  }

  const handleAvatarFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) {
      return
    }
    setProfileNotice(null)
    if (!ALLOWED_AVATAR_TYPES.includes(file.type)) {
      setProfileNotice({ type: 'error', text: '仅支持 JPG、PNG、WebP 或 GIF 格式图片' })
      return
    }
    if (file.size > MAX_AVATAR_BYTES) {
      setProfileNotice({ type: 'error', text: '头像文件大小不能超过 5MB' })
      return
    }

    const previewUrl = URL.createObjectURL(file)
    setPendingAvatarFile(file)
    setPendingAvatarPreview((prev) => {
      if (prev) {
        URL.revokeObjectURL(prev)
      }
      return previewUrl
    })
  }

  const handleSelectPresetAvatar = (url: string) => {
    clearPendingAvatar()
    setAvatarUrl(url)
  }

  const handleAvatarUrlInputChange = (value: string) => {
    clearPendingAvatar()
    setAvatarUrl(value)
  }

  const handleProfileSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setProfileNotice(null)

    const trimmedNickname = nickname.trim()
    if (!trimmedNickname || [...trimmedNickname].length > 64) {
      setProfileNotice({ type: 'error', text: '昵称长度需在 1~64 个字符之间' })
      return
    }
    if (bio.trim().length > 200) {
      setProfileNotice({ type: 'error', text: '个人签名不能超过 200 个字符' })
      return
    }

    setSavingProfile(true)
    try {
      let finalAvatarUrl = avatarUrl.trim()
      if (pendingAvatarFile) {
        const uploaded = await uploadApi.uploadImageDirect('avatar', pendingAvatarFile)
        finalAvatarUrl = uploaded.public_url
      }

      const updated = await userApi.updateMe({
        nickname: trimmedNickname,
        bio: bio.trim(),
        avatar_url: finalAvatarUrl,
        email: email.trim(),
        phone: phone.trim(),
      })
      clearPendingAvatar()
      setAvatarUrl(updated.avatar_url)
      setUser(updated)
      setProfileNotice({ type: 'success', text: '个人资料保存成功' })
    } catch (err) {
      setProfileNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '保存失败，请稍后重试',
      })
    } finally {
      setSavingProfile(false)
    }
  }

  const handlePasswordSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setPasswordNotice(null)

    if (!oldPassword) {
      setPasswordNotice({ type: 'error', text: '请输入当前密码' })
      return
    }
    if (!newPassword) {
      setPasswordNotice({ type: 'error', text: '请输入新密码' })
      return
    }
    if (newPassword === oldPassword) {
      setPasswordNotice({ type: 'error', text: '新密码不能与原密码相同' })
      return
    }
    if (newPassword !== confirmNewPassword) {
      setPasswordNotice({ type: 'error', text: '两次输入的新密码不一致' })
      return
    }

    setSavingPassword(true)
    try {
      await userApi.updatePassword({
        old_password: oldPassword,
        new_password: newPassword,
      })
      setOldPassword('')
      setNewPassword('')
      setConfirmNewPassword('')
      await logout()
      openAuthModal('login')
    } catch (err) {
      setPasswordNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '密码修改失败，请稍后重试',
      })
    } finally {
      setSavingPassword(false)
    }
  }

  return (
    <div className="account-page">
      <BiliHeader />

      {initializing ? (
        <div className="account-guest">
          <p>正在加载账号信息...</p>
        </div>
      ) : !user ? (
        <div className="account-guest">
          <h2>请先登录账号</h2>
          <p>登录后可在个人中心管理昵称、头像、绑定信息与账号密码。</p>
          <button
            type="button"
            className="account-submit-btn"
            onClick={() => openAuthModal('login')}
          >
            立即登录
          </button>
        </div>
      ) : (
        <div className="account-layout">
          <aside className="account-sidebar">
            <div className="account-sidebar__title">个人中心</div>
            <nav className="account-nav" aria-label="个人中心菜单">
              <button
                type="button"
                className={`account-nav__item ${
                  activeTab === 'profile' ? 'account-nav__item--active' : ''
                }`}
                onClick={() => setActiveTab('profile')}
              >
                我的信息
              </button>
              <button
                type="button"
                className={`account-nav__item ${
                  activeTab === 'security' ? 'account-nav__item--active' : ''
                }`}
                onClick={() => setActiveTab('security')}
              >
                账号安全
              </button>
              <Link to={`/space/${user.id}`} className="account-nav__item">
                查看个人空间 ↗
              </Link>
            </nav>
          </aside>

          <section className="account-main">
            <div className="account-main__head">
              <span className="account-main__bar" />
              <h1 className="account-main__title">
                {activeTab === 'profile' ? '我的信息' : '账号安全 · 修改密码'}
              </h1>
            </div>

            {activeTab === 'profile' ? (
              <>
                <div className="account-summary">
                  <UserAvatar
                    src={pendingAvatarPreview ?? avatarUrl}
                    nickname={nickname || user.nickname}
                    size={64}
                  />
                  <div className="account-summary__info">
                    <div className="account-summary__name">{user.nickname}</div>
                    <div className="account-summary__meta">
                      <span>UID：{user.id}</span>
                      <span>用户名：{user.username}</span>
                      <span>硬币余额：{user.coin_balance}</span>
                    </div>
                  </div>
                </div>

                <CheckinPanel />

                <form className="account-form" onSubmit={handleProfileSubmit} noValidate>
                  <div className="account-field">
                    <label className="account-field__label" htmlFor="profile-nickname">
                      昵称
                    </label>
                    <div className="account-field__control">
                      <input
                        id="profile-nickname"
                        type="text"
                        className="account-input"
                        value={nickname}
                        onChange={(e) => setNickname(e.target.value)}
                        maxLength={64}
                      />
                    </div>
                  </div>

                  <div className="account-field">
                    <label className="account-field__label" htmlFor="profile-avatar">
                      头像
                    </label>
                    <div className="account-field__control">
                      <div className="avatar-upload-row">
                        <input
                          ref={fileInputRef}
                          type="file"
                          accept="image/jpeg,image/png,image/webp,image/gif"
                          onChange={handleAvatarFileChange}
                          style={{ display: 'none' }}
                          aria-label="选择本地头像文件"
                        />
                        <button
                          type="button"
                          className="avatar-upload-btn"
                          onClick={() => fileInputRef.current?.click()}
                          disabled={savingProfile}
                        >
                          选择本地头像
                        </button>
                        <span className="avatar-upload-hint">支持 JPG / PNG / WebP / GIF，最大 5MB</span>
                      </div>
                      {pendingAvatarFile && (
                        <div className="avatar-pending-tag" role="status">
                          <span>已选择：{pendingAvatarFile.name}（点击保存后上传生效）</span>
                          <button
                            type="button"
                            className="avatar-cancel-btn"
                            onClick={clearPendingAvatar}
                          >
                            取消选择
                          </button>
                        </div>
                      )}
                      <div className="avatar-presets" aria-label="预设头像">
                        {PRESET_AVATARS.map((item) => (
                          <button
                            key={item.url}
                            type="button"
                            title={item.label}
                            className={`avatar-preset-btn ${
                              !pendingAvatarFile && avatarUrl === item.url
                                ? 'avatar-preset-btn--selected'
                                : ''
                            }`}
                            onClick={() => handleSelectPresetAvatar(item.url)}
                          >
                            <UserAvatar src={item.url} nickname={item.label} size={42} />
                          </button>
                        ))}
                      </div>
                      <input
                        id="profile-avatar"
                        type="text"
                        className="account-input"
                        placeholder="输入头像 URL（支持 /storage/...、/avatars/... 或 http(s)://...）"
                        value={avatarUrl}
                        onChange={(e) => handleAvatarUrlInputChange(e.target.value)}
                        maxLength={512}
                      />
                    </div>
                  </div>

                  <div className="account-field">
                    <label className="account-field__label" htmlFor="profile-bio">
                      我的签名
                    </label>
                    <div className="account-field__control">
                      <textarea
                        id="profile-bio"
                        className="account-textarea"
                        placeholder="设置您的签名，展示个人风格"
                        value={bio}
                        onChange={(e) => setBio(e.target.value)}
                        maxLength={200}
                      />
                    </div>
                  </div>

                  <div className="account-field">
                    <label className="account-field__label" htmlFor="profile-email">
                      绑定邮箱
                    </label>
                    <div className="account-field__control">
                      <input
                        id="profile-email"
                        type="email"
                        className="account-input"
                        placeholder="留空则不绑定邮箱"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                      />
                    </div>
                  </div>

                  <div className="account-field">
                    <label className="account-field__label" htmlFor="profile-phone">
                      绑定手机号
                    </label>
                    <div className="account-field__control">
                      <input
                        id="profile-phone"
                        type="tel"
                        className="account-input"
                        placeholder="留空则不绑定手机号"
                        value={phone}
                        onChange={(e) => setPhone(e.target.value)}
                      />
                    </div>
                  </div>

                  {profileNotice && (
                    <div
                      className={`account-feedback ${
                        profileNotice.type === 'success'
                          ? 'account-feedback--success'
                          : 'account-feedback--error'
                      }`}
                      role="status"
                    >
                      {profileNotice.text}
                    </div>
                  )}

                  <div className="account-submit-row">
                    <button
                      type="submit"
                      className="account-submit-btn"
                      disabled={savingProfile}
                    >
                      {savingProfile ? '保存中...' : '保存'}
                    </button>
                  </div>
                </form>
              </>
            ) : (
              <form className="account-form" onSubmit={handlePasswordSubmit} noValidate>
                <div className="account-field">
                  <label className="account-field__label" htmlFor="old-password">
                    原密码
                  </label>
                  <div className="account-field__control">
                    <input
                      id="old-password"
                      type="password"
                      className="account-input"
                      value={oldPassword}
                      onChange={(e) => setOldPassword(e.target.value)}
                      autoComplete="current-password"
                    />
                  </div>
                </div>

                <div className="account-field">
                  <label className="account-field__label" htmlFor="new-password">
                    新密码
                  </label>
                  <div className="account-field__control">
                    <input
                      id="new-password"
                      type="password"
                      className="account-input"
                      placeholder="请输入新密码"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      autoComplete="new-password"
                    />
                  </div>
                </div>

                <div className="account-field">
                  <label className="account-field__label" htmlFor="confirm-new-password">
                    确认新密码
                  </label>
                  <div className="account-field__control">
                    <input
                      id="confirm-new-password"
                      type="password"
                      className="account-input"
                      value={confirmNewPassword}
                      onChange={(e) => setConfirmNewPassword(e.target.value)}
                      autoComplete="new-password"
                    />
                  </div>
                </div>

                {passwordNotice && (
                  <div
                    className={`account-feedback ${
                      passwordNotice.type === 'success'
                        ? 'account-feedback--success'
                        : 'account-feedback--error'
                    }`}
                    role="alert"
                  >
                    {passwordNotice.text}
                  </div>
                )}

                <div className="account-submit-row">
                  <button
                    type="submit"
                    className="account-submit-btn"
                    disabled={savingPassword}
                  >
                    {savingPassword ? '提交中...' : '确认修改密码'}
                  </button>
                </div>
              </form>
            )}
          </section>
        </div>
      )}
    </div>
  )
}
