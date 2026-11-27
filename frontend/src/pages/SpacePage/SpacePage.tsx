import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { ApiError } from '../../api/client'
import { interactionApi } from '../../api/interaction'
import { userApi } from '../../api/user'
import { videoApi } from '../../api/video'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { UserAvatar } from '../../components/UserAvatar/UserAvatar'
import { VideoCard } from '../../components/VideoCard/VideoCard'
import { useAuth } from '../../context/AuthContext'
import type { FavoriteFolder } from '../../types/interaction'
import type { PublicUserProfile } from '../../types/user'
import type { VideoSummary } from '../../types/video'
import './SpacePage.css'

type SpaceTab = 'videos' | 'favorites'

export function SpacePage() {
  const { id } = useParams<{ id: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const { user } = useAuth()

  const activeTab: SpaceTab = searchParams.get('tab') === 'favorites' ? 'favorites' : 'videos'

  const [profile, setProfile] = useState<PublicUserProfile | null>(null)
  const [videos, setVideos] = useState<VideoSummary[]>([])
  const [videoTotal, setVideoTotal] = useState(0)
  const [videosLoading, setVideosLoading] = useState(false)
  const [loading, setLoading] = useState(true)
  const [errorMsg, setErrorMsg] = useState('')

  // 收藏夹状态
  const [folders, setFolders] = useState<FavoriteFolder[]>([])
  const [foldersLoading, setFoldersLoading] = useState(false)
  const [selectedFolderId, setSelectedFolderId] = useState<string>('')
  const [folderVideos, setFolderVideos] = useState<VideoSummary[]>([])
  const [folderVideosLoading, setFolderVideosLoading] = useState(false)
  const [folderNotice, setFolderNotice] = useState('')

  const [newTitle, setNewTitle] = useState('')
  const [newPublic, setNewPublic] = useState(true)
  const [editingFolderId, setEditingFolderId] = useState('')
  const [editTitle, setEditTitle] = useState('')
  const [editPublic, setEditPublic] = useState(true)

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

  useEffect(() => {
    if (!id || !profile) {
      return
    }
    let cancelled = false
    setVideosLoading(true)
    videoApi
      .listUserVideos(id, { sort: 'latest', page: 1, page_size: 20 })
      .then((res) => {
        if (!cancelled) {
          setVideos(res.items)
          setVideoTotal(res.total)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setVideos([])
          setVideoTotal(0)
        }
      })
      .finally(() => {
        if (!cancelled) {
          setVideosLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [id, profile])

  const loadFolders = useCallback(async () => {
    if (!id || !profile) {
      return
    }
    setFoldersLoading(true)
    setFolderNotice('')
    try {
      const list = await interactionApi.listFavoriteFolders(isSelf ? undefined : id)
      setFolders(list)
      setSelectedFolderId((prev) =>
        list.some((item) => item.id === prev) ? prev : (list[0]?.id ?? ''),
      )
    } catch (err) {
      setFolders([])
      setFolderNotice(err instanceof ApiError ? err.message : '加载收藏夹失败')
    } finally {
      setFoldersLoading(false)
    }
  }, [id, profile, isSelf])

  useEffect(() => {
    if (activeTab === 'favorites' && profile) {
      void loadFolders()
    }
  }, [activeTab, profile, loadFolders])

  useEffect(() => {
    if (activeTab !== 'favorites' || !selectedFolderId) {
      setFolderVideos([])
      return
    }
    let cancelled = false
    setFolderVideosLoading(true)
    interactionApi
      .listFavoriteFolderVideos(selectedFolderId, { page: 1, page_size: 24 })
      .then((res) => {
        if (!cancelled) {
          setFolderVideos(res.items)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFolderVideos([])
        }
      })
      .finally(() => {
        if (!cancelled) {
          setFolderVideosLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [activeTab, selectedFolderId])

  const switchTab = (tab: SpaceTab) => {
    if (tab === 'favorites') {
      setSearchParams({ tab: 'favorites' })
    } else {
      setSearchParams({})
    }
  }

  const handleCreateFolder = async (event: FormEvent) => {
    event.preventDefault()
    const trimmed = newTitle.trim()
    if (!trimmed) {
      return
    }
    setFolderNotice('')
    try {
      const created = await interactionApi.createFavoriteFolder(trimmed, newPublic)
      setNewTitle('')
      setFolders((prev) => [...prev, created])
      setSelectedFolderId(created.id)
    } catch (err) {
      setFolderNotice(err instanceof ApiError ? err.message : '创建收藏夹失败')
    }
  }

  const handleSaveEditFolder = async (event: FormEvent) => {
    event.preventDefault()
    if (!editingFolderId || !editTitle.trim()) {
      return
    }
    try {
      const updated = await interactionApi.updateFavoriteFolder(
        editingFolderId,
        editTitle.trim(),
        editPublic,
      )
      setFolders((prev) => prev.map((f) => (f.id === updated.id ? updated : f)))
      setEditingFolderId('')
    } catch (err) {
      setFolderNotice(err instanceof ApiError ? err.message : '修改收藏夹失败')
    }
  }

  const handleDeleteFolder = async (folder: FavoriteFolder) => {
    if (!window.confirm(`确认删除收藏夹「${folder.title}」吗？`)) {
      return
    }
    try {
      await interactionApi.deleteFavoriteFolder(folder.id)
      await loadFolders()
    } catch (err) {
      setFolderNotice(err instanceof ApiError ? err.message : '删除收藏夹失败')
    }
  }

  const activeFolder = folders.find((f) => f.id === selectedFolderId) ?? null

  return (
    <div className="space-page">
      <BiliHeader />

      <main className="space-container">
        {loading ? (
          <div className="space-skeleton" aria-label="正在加载个人空间">
            <div className="bili-skeleton space-skeleton__banner" />
            <div className="bili-skeleton space-skeleton__tabs" />
            <div className="space-body">
              <div className="space-card">
                <div className="bili-skeleton space-skeleton__heading" />
                <div className="space-videos-grid">
                  {Array.from({ length: 6 }).map((_, idx) => (
                    <div key={idx}>
                      <div className="bili-skeleton space-skeleton__thumb" />
                      <div className="bili-skeleton space-skeleton__line" />
                    </div>
                  ))}
                </div>
              </div>
              <div className="bili-skeleton space-skeleton__aside" />
            </div>
          </div>
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
                <div className="space-banner__actions">
                  <Link to="/upload?tab=manage" className="space-banner__btn space-banner__btn--ghost">
                    稿件管理
                  </Link>
                  <Link to="/account" className="space-banner__btn">
                    编辑资料
                  </Link>
                </div>
              )}
            </section>

            <section className="space-tabs-bar">
              <div className="space-tabs">
                <button
                  type="button"
                  className={`space-tab ${activeTab === 'videos' ? 'space-tab--active' : ''}`}
                  onClick={() => switchTab('videos')}
                >
                  投稿 ({videoTotal})
                </button>
                <button
                  type="button"
                  className={`space-tab ${activeTab === 'favorites' ? 'space-tab--active' : ''}`}
                  onClick={() => switchTab('favorites')}
                >
                  收藏
                </button>
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

            {activeTab === 'videos' ? (
              <section className="space-body">
                <div className="space-card">
                  <h2 className="space-card__title">TA 的视频投稿（{videoTotal}）</h2>
                  {videosLoading ? (
                    <div className="space-empty">正在加载视频列表...</div>
                  ) : videos.length === 0 ? (
                    <div className="space-empty">暂无已发布的投稿视频</div>
                  ) : (
                    <div className="space-videos-grid">
                      {videos.map((item) => (
                        <VideoCard key={item.bv_id} video={item} />
                      ))}
                    </div>
                  )}
                </div>

                <aside className="space-card">
                  <h2 className="space-card__title">个人资料</h2>
                  <div className="space-meta-list">
                    <div>UID：{profile.id}</div>
                    <div>已发布投稿：{videoTotal} 个</div>
                    <div>注册时间：{profile.created_at.slice(0, 10)}</div>
                  </div>
                </aside>
              </section>
            ) : (
              <section className="space-fav-layout">
                <aside className="space-card space-fav-sidebar">
                  <h2 className="space-card__title">
                    {isSelf ? '我的收藏夹' : 'TA 的公开收藏夹'}
                  </h2>

                  {isSelf && (
                    <form className="space-fav-create" onSubmit={handleCreateFolder}>
                      <input
                        type="text"
                        value={newTitle}
                        maxLength={64}
                        onChange={(e) => setNewTitle(e.target.value)}
                        placeholder="新建收藏夹名称"
                        aria-label="新建收藏夹名称"
                      />
                      <div className="space-fav-create__row">
                        <label>
                          <input
                            type="checkbox"
                            checked={newPublic}
                            onChange={(e) => setNewPublic(e.target.checked)}
                          />
                          公开可见
                        </label>
                        <button type="submit">创建</button>
                      </div>
                    </form>
                  )}

                  {folderNotice && <div className="space-fav-notice">{folderNotice}</div>}

                  {foldersLoading ? (
                    <div className="space-empty">加载中...</div>
                  ) : folders.length === 0 ? (
                    <div className="space-empty">暂无收藏夹</div>
                  ) : (
                    <ul className="space-fav-list">
                      {folders.map((folder) => {
                        const active = folder.id === selectedFolderId
                        const isEditing = editingFolderId === folder.id
                        return (
                          <li key={folder.id} className="space-fav-item-wrap">
                            <button
                              type="button"
                              className={`space-fav-item ${active ? 'space-fav-item--active' : ''}`}
                              onClick={() => setSelectedFolderId(folder.id)}
                            >
                              <span className="space-fav-item__name">
                                {folder.title}
                                {!folder.is_public && <em>私密</em>}
                              </span>
                              <span className="space-fav-item__count">{folder.item_count}</span>
                            </button>

                            {isSelf && (
                              <div className="space-fav-item__ops">
                                <button
                                  type="button"
                                  onClick={() => {
                                    setEditingFolderId(isEditing ? '' : folder.id)
                                    setEditTitle(folder.title)
                                    setEditPublic(folder.is_public)
                                  }}
                                >
                                  {isEditing ? '取消' : '编辑'}
                                </button>
                                {!folder.is_default && (
                                  <button
                                    type="button"
                                    className="space-fav-del"
                                    onClick={() => void handleDeleteFolder(folder)}
                                  >
                                    删除
                                  </button>
                                )}
                              </div>
                            )}

                            {isEditing && (
                              <form className="space-fav-edit" onSubmit={handleSaveEditFolder}>
                                <input
                                  type="text"
                                  value={editTitle}
                                  maxLength={64}
                                  onChange={(e) => setEditTitle(e.target.value)}
                                />
                                <label>
                                  <input
                                    type="checkbox"
                                    checked={editPublic}
                                    onChange={(e) => setEditPublic(e.target.checked)}
                                  />
                                  公开
                                </label>
                                <button type="submit">保存</button>
                              </form>
                            )}
                          </li>
                        )
                      })}
                    </ul>
                  )}
                </aside>

                <div className="space-card">
                  <h2 className="space-card__title">
                    {activeFolder
                      ? `${activeFolder.title}（${activeFolder.item_count}）`
                      : '收藏视频'}
                  </h2>
                  {folderVideosLoading ? (
                    <div className="space-empty">正在加载收藏视频...</div>
                  ) : folderVideos.length === 0 ? (
                    <div className="space-empty">该收藏夹下还没有视频</div>
                  ) : (
                    <div className="space-videos-grid">
                      {folderVideos.map((item) => (
                        <VideoCard key={item.bv_id} video={item} />
                      ))}
                    </div>
                  )}
                </div>
              </section>
            )}
          </>
        )}
      </main>
    </div>
  )
}
