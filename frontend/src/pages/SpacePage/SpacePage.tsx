import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError } from '../../api/client'
import { userApi } from '../../api/user'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { UserAvatar } from '../../components/UserAvatar/UserAvatar'
import { useAuth } from '../../context/AuthContext'
import type { PublicUserProfile } from '../../types/user'
import './SpacePage.css'

export function SpacePage() {
  const { id } = useParams<{ id: string }>()
  const { user } = useAuth()
  const [profile, setProfile] = useState<PublicUserProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    if (!id) {
      setErrorMsg('缺少用户 ID')
      setLoading(false)
      return
    }
    let cancelled = false
    setLoading(true)
    setErrorMsg('')
    userApi
      .getPublicUser(id)
      .then((data) => {
        if (!cancelled) {
          setProfile(data)
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setProfile(null)
          setErrorMsg(err instanceof ApiError ? err.message : '加载用户主页失败')
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [id])

  const isSelf = Boolean(user && profile && user.id === profile.id)

  return (
    <div className="space-page">
      <BiliHeader />

      <main className="space-container">
        {loading ? (
          <div className="space-card space-empty">正在加载个人空间...</div>
        ) : errorMsg || !profile ? (
          <div className="space-card space-empty">{errorMsg || '用户不存在'}</div>
        ) : (
          <>
            <section className="space-banner">
              <div className="space-banner__user">
                <UserAvatar
                  src={profile.avatar_url}
                  nickname={profile.nickname}
                  size={74}
                  className="space-banner__avatar"
                />
                <div>
                  <div className="space-banner__name-row">
                    <h1 className="space-banner__name">{profile.nickname}</h1>
                    <span className="space-banner__uid">UID {profile.id}</span>
                  </div>
                  <p className="space-banner__bio">
                    {profile.bio || '这个人很懒，什么都没有写'}
                  </p>
                </div>
              </div>

              {isSelf && (
                <Link to="/account" className="space-banner__btn">
                  编辑资料
                </Link>
              )}
            </section>

            <section className="space-tabs-bar">
              <div className="space-tabs">
                <span className="space-tab space-tab--active">主页</span>
                <span className="space-tab">动态</span>
                <span className="space-tab">投稿</span>
                <span className="space-tab">收藏</span>
              </div>

              <div className="space-stats">
                <div className="space-stat">
                  <span className="space-stat__label">关注数</span>
                  <span className="space-stat__num">{profile.following_count}</span>
                </div>
                <div className="space-stat">
                  <span className="space-stat__label">粉丝数</span>
                  <span className="space-stat__num">{profile.follower_count}</span>
                </div>
              </div>
            </section>

            <section className="space-body">
              <div className="space-card">
                <h2 className="space-card__title">TA 的视频投稿</h2>
                <div className="space-empty">暂无投稿视频</div>
              </div>

              <aside className="space-card">
                <h2 className="space-card__title">个人资料</h2>
                <div className="space-meta-list">
                  <div>UID：{profile.id}</div>
                  <div>注册时间：{profile.created_at.slice(0, 10)}</div>
                </div>
              </aside>
            </section>
          </>
        )}
      </main>
    </div>
  )
}
