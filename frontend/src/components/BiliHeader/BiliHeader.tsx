import { useEffect, useState, type FormEvent } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { interactionApi } from '../../api/interaction'
import { useAuth } from '../../context/AuthContext'
import { UserAvatar } from '../UserAvatar/UserAvatar'
import './BiliHeader.css'

const MAIN_NAV_LINKS = ['首页', '番剧', '直播', '游戏中心', '会员购', '漫画', '赛事']

interface BiliHeaderProps {
  withBanner?: boolean
}

export function BiliHeader({ withBanner = false }: BiliHeaderProps) {
  const { user, setUser, openAuthModal, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [scrolled, setScrolled] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [checkedInToday, setCheckedInToday] = useState(false)

  useEffect(() => {
    if (!user) {
      setCheckedInToday(false)
      return
    }
    let cancelled = false
    interactionApi
      .getCheckinStatus()
      .then((res) => {
        if (!cancelled) {
          setCheckedInToday(res.checked_in_today)
          if (res.coin_balance !== user.coin_balance) {
            setUser({ ...user, coin_balance: res.coin_balance })
          }
        }
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.id])

  useEffect(() => {
    const q = new URLSearchParams(location.search).get('q')
    if (location.pathname === '/search' && q !== null) {
      setKeyword(q)
    }
  }, [location.pathname, location.search])

  useEffect(() => {
    if (!withBanner) {
      setScrolled(true)
      return
    }
    const handleScroll = () => {
      setScrolled(window.scrollY > 72)
    }
    handleScroll()
    window.addEventListener('scroll', handleScroll, { passive: true })
    return () => window.removeEventListener('scroll', handleScroll)
  }, [withBanner])

  const isSolid = !withBanner || scrolled

  const handleSearch = (event: FormEvent) => {
    event.preventDefault()
    const trimmed = keyword.trim()
    if (!trimmed) {
      navigate('/search')
      return
    }
    navigate(`/search?q=${encodeURIComponent(trimmed)}`)
  }

  const handleLogout = async () => {
    await logout()
    navigate('/')
  }

  return (
    <header className="bili-header-wrap">
      <div className={`bili-topbar ${isSolid ? 'bili-topbar--solid' : 'bili-topbar--transparent'}`}>
        <div className="bili-topbar__left">
          <Link to="/" className="bili-brand">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M7.5 3.5L10 6.5M16.5 3.5L14 6.5"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
              />
              <rect x="3" y="6.5" width="18" height="13.5" rx="4" stroke="currentColor" strokeWidth="2" />
              <circle cx="9" cy="13" r="1.4" fill="currentColor" />
              <circle cx="15" cy="13" r="1.4" fill="currentColor" />
            </svg>
            <span>bilibili</span>
          </Link>

          <ul className="bili-nav-links">
            {MAIN_NAV_LINKS.map((item) => (
              <li key={item}>
                <Link to="/" className="bili-nav-link">
                  {item}
                </Link>
              </li>
            ))}
          </ul>
        </div>

        <form className="bili-search" onSubmit={handleSearch} role="search">
          <input
            type="search"
            className="bili-search__input"
            placeholder="搜索你感兴趣的视频或 UP 主"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            aria-label="搜索视频或 UP 主"
          />
          <button type="submit" className="bili-search__btn" aria-label="搜索">
            <svg width="17" height="17" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="2" />
              <path d="M16.5 16.5L21 21" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </button>
        </form>

        <div className="bili-topbar__right">
          <div className="bili-user-entry">
            {user ? (
              <>
                <Link
                  to={`/space/${user.id}`}
                  className="bili-avatar-trigger"
                  aria-label="个人空间"
                >
                  <UserAvatar src={user.avatar_url} nickname={user.nickname} size={36} />
                </Link>
                <div className="bili-popover" role="region" aria-label="个人菜单">
                  <div className="bili-profile-card__head">
                    <UserAvatar src={user.avatar_url} nickname={user.nickname} size={60} />
                    <div className="bili-profile-card__name">{user.nickname}</div>
                    <div className="bili-profile-card__meta">
                      <span>UID: {user.id}</span>
                      <span className="bili-profile-card__coin">硬币 {user.coin_balance}</span>
                    </div>
                    {checkedInToday && (
                      <span className="bili-checkin-pill bili-checkin-pill--done">
                        今日已自动签到 +1 硬币
                      </span>
                    )}
                  </div>
                  <div className="bili-profile-card__stats">
                    <Link to={`/space/${user.id}`}>
                      <div className="bili-profile-card__stat-num">{user.following_count}</div>
                      <div className="bili-profile-card__stat-label">关注</div>
                    </Link>
                    <Link to={`/space/${user.id}`}>
                      <div className="bili-profile-card__stat-num">{user.follower_count}</div>
                      <div className="bili-profile-card__stat-label">粉丝</div>
                    </Link>
                    <Link to={`/space/${user.id}`}>
                      <div className="bili-profile-card__stat-num">0</div>
                      <div className="bili-profile-card__stat-label">动态</div>
                    </Link>
                  </div>
                  <div className="bili-profile-card__menu">
                    <Link to="/account" className="bili-profile-card__item">
                      <span>个人中心</span>
                      <span>›</span>
                    </Link>
                    <Link to="/upload?tab=manage" className="bili-profile-card__item">
                      <span>稿件管理</span>
                      <span>›</span>
                    </Link>
                    <Link to={`/space/${user.id}`} className="bili-profile-card__item">
                      <span>我的主页</span>
                      <span>›</span>
                    </Link>
                    <div className="bili-profile-card__logout">
                      <button
                        type="button"
                        className="bili-profile-card__item"
                        onClick={handleLogout}
                      >
                        <span>退出登录</span>
                      </button>
                    </div>
                  </div>
                </div>
              </>
            ) : (
              <>
                <button
                  type="button"
                  className="bili-login-circle"
                  onClick={() => openAuthModal('login')}
                >
                  登录
                </button>
                <div className="bili-popover" role="region" aria-label="登录提示">
                  <div className="bili-guest-card__title">登录后你可以：</div>
                  <div className="bili-guest-card__perks">
                    <div className="bili-guest-card__perk">
                      <span className="bili-guest-card__dot" />
                      <span>免费看高清视频</span>
                    </div>
                    <div className="bili-guest-card__perk">
                      <span className="bili-guest-card__dot" />
                      <span>多端同步播放记录</span>
                    </div>
                    <div className="bili-guest-card__perk">
                      <span className="bili-guest-card__dot" />
                      <span>发表弹幕和评论</span>
                    </div>
                    <div className="bili-guest-card__perk">
                      <span className="bili-guest-card__dot" />
                      <span>点赞投币一键三连</span>
                    </div>
                  </div>
                  <button
                    type="button"
                    className="bili-guest-card__btn"
                    onClick={() => openAuthModal('login')}
                  >
                    立即登录
                  </button>
                  <div className="bili-guest-card__footer">
                    首次使用？{' '}
                    <button
                      type="button"
                      className="bili-guest-card__link"
                      onClick={() => openAuthModal('register')}
                    >
                      点我注册
                    </button>
                  </div>
                </div>
              </>
            )}
          </div>

          <button
            type="button"
            className="bili-icon-entry bili-icon-entry--secondary"
            onClick={() => (user ? navigate(`/space/${user.id}?tab=favorites`) : openAuthModal('login'))}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M12 3l2.4 5 5.6.8-4 3.9 1 5.5L12 15.6 7 18.2l1-5.5-4-3.9 5.6-.8L12 3z"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinejoin="round"
              />
            </svg>
            <span>收藏</span>
          </button>

          <button
            type="button"
            className="bili-icon-entry bili-icon-entry--secondary"
            onClick={() => (user ? navigate('/history') : openAuthModal('login'))}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="12" cy="12" r="8.5" stroke="currentColor" strokeWidth="1.8" />
              <path d="M12 7.5v5l3.5 2" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
            </svg>
            <span>历史</span>
          </button>

          <button
            type="button"
            className="bili-upload-btn"
            onClick={() => (user ? navigate('/upload') : openAuthModal('login'))}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M12 15V4m0 0l-4 4m4-4l4 4M4 17v2a2 2 0 002 2h12a2 2 0 002-2v-2"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
            <span>投稿</span>
          </button>
        </div>
      </div>

      {withBanner ? (
        <div className="bili-banner">
          <svg
            className="bili-banner__decor"
            viewBox="0 0 1440 155"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <circle cx="240" cy="40" r="90" fill="#ffffff" fillOpacity="0.16" />
            <circle cx="1160" cy="24" r="110" fill="#ffffff" fillOpacity="0.18" />
            <path
              d="M0 128 C260 88, 520 152, 820 118 C1080 90, 1280 138, 1440 112 L1440 155 L0 155 Z"
              fill="#ffffff"
              fillOpacity="0.22"
            />
          </svg>
          <div className="bili-banner__inner">
            <Link to="/" className="bili-banner__logo">
              <svg width="38" height="38" viewBox="0 0 40 40" fill="none" aria-hidden="true">
                <path
                  d="M12.5 6.5L16.8 11.2M27.5 6.5L23.2 11.2"
                  stroke="#FFFFFF"
                  strokeWidth="3"
                  strokeLinecap="round"
                />
                <rect x="5" y="11" width="30" height="22" rx="6.5" stroke="#FFFFFF" strokeWidth="3" />
                <circle cx="15" cy="21.5" r="2.2" fill="#FFFFFF" />
                <circle cx="25" cy="21.5" r="2.2" fill="#FFFFFF" />
              </svg>
              <span>bilibili</span>
            </Link>
          </div>
        </div>
      ) : (
        <div className="bili-header-spacer" />
      )}
    </header>
  )
}
