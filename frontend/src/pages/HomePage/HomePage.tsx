import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { videoApi } from '../../api/video'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { VideoCard } from '../../components/VideoCard/VideoCard'
import { useAuth } from '../../context/AuthContext'
import type { Category, VideoSortOption, VideoSummary } from '../../types/video'
import './HomePage.css'

const SIDE_CHANNELS = ['专栏', '活动', '社区中心', '直播', '课堂', '新歌热榜']

const SORT_OPTIONS: { value: VideoSortOption; label: string }[] = [
  { value: 'latest', label: '最新发布' },
  { value: 'hot', label: '全站热门' },
  { value: 'play', label: '最多播放' },
  { value: 'danmaku', label: '最多弹幕' },
]

const PAGE_SIZE = 16

export function HomePage() {
  const { user, openAuthModal } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()

  const activeCategoryId = searchParams.get('category')
    ? Number(searchParams.get('category'))
    : undefined
  const activeSort = (searchParams.get('sort') as VideoSortOption) || 'latest'
  const page = Math.max(1, Number(searchParams.get('page') || '1'))

  const [categories, setCategories] = useState<Category[]>([])
  const [videos, setVideos] = useState<VideoSummary[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    videoApi
      .listCategories()
      .then(setCategories)
      .catch(() => {})
  }, [])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setErrorMsg('')
    videoApi
      .listVideos({
        category_id: activeCategoryId,
        sort: activeSort,
        page,
        page_size: PAGE_SIZE,
      })
      .then((res) => {
        if (!cancelled) {
          setVideos(res.items)
          setTotal(res.total)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setErrorMsg('加载视频列表失败，请稍后刷新重试')
          setVideos([])
          setTotal(0)
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
  }, [activeCategoryId, activeSort, page])

  const updateQuery = (patch: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams)
    for (const [k, v] of Object.entries(patch)) {
      if (!v) {
        next.delete(k)
      } else {
        next.set(k, v)
      }
    }
    setSearchParams(next)
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const activeCategory = categories.find((c) => c.id === activeCategoryId)

  return (
    <div className="home-page">
      <BiliHeader withBanner />

      <main className="home-container">
        <section className="channel-bar" aria-label="频道分区导航">
          <div className="channel-bar__icons">
            <Link to={user ? `/space/${user.id}` : '/'} className="channel-icon-item">
              <span className="channel-icon-circle channel-icon-circle--dynamic">
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M12 4a8 8 0 108 8"
                    stroke="currentColor"
                    strokeWidth="2.2"
                    strokeLinecap="round"
                  />
                  <circle cx="17.5" cy="6.5" r="2.5" fill="currentColor" />
                </svg>
              </span>
              <span>动态</span>
            </Link>
            <button
              type="button"
              className="channel-icon-item"
              onClick={() => updateQuery({ category: undefined, sort: 'hot', page: '1' })}
            >
              <span className="channel-icon-circle channel-icon-circle--hot">
                <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M12.5 3.5c.8 3.2-1.8 4.9-1.8 7.3 0 1.4 1 2.4 2.3 2.4 1.8 0 3-1.6 3-3.7 2.2 1.8 3 4.3 3 6.5a7 7 0 11-14 0c0-3.4 2-6.1 4.8-8.2-.2 1.6.4 2.7 1.3 3.2-.5-2.8.4-5.4 1.4-7.5z"
                    fill="currentColor"
                  />
                </svg>
              </span>
              <span>热门</span>
            </button>
          </div>

          <div className="channel-bar__tags">
            <button
              type="button"
              className={`channel-tag ${!activeCategoryId ? 'channel-tag--active' : ''}`}
              onClick={() => updateQuery({ category: undefined, page: '1' })}
            >
              全部
            </button>
            {categories.map((cat) => (
              <button
                key={cat.id}
                type="button"
                className={`channel-tag ${activeCategoryId === cat.id ? 'channel-tag--active' : ''}`}
                onClick={() => updateQuery({ category: String(cat.id), page: '1' })}
              >
                {cat.name}
              </button>
            ))}
          </div>

          <div className="channel-bar__right">
            {SIDE_CHANNELS.map((item) => (
              <Link key={item} to="/" className="channel-side-link">
                <span>{item}</span>
              </Link>
            ))}
          </div>
        </section>

        <section className="home-sort-bar" aria-label="排序切换">
          <div className="home-sort-bar__left">
            <h2 className="home-sort-bar__heading">
              {activeCategory ? `${activeCategory.name}分区` : '全站视频'}
            </h2>
            <span className="home-sort-bar__count">共 {total} 个已发布视频</span>
          </div>
          <div className="home-sort-bar__tabs">
            {SORT_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                type="button"
                className={`home-sort-btn ${activeSort === opt.value ? 'home-sort-btn--active' : ''}`}
                onClick={() => updateQuery({ sort: opt.value, page: '1' })}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </section>

        <section className="feed-grid" aria-label="首页视频区">
          <div className="hero-carousel">
            <div className="hero-carousel__visual">
              <span className="hero-carousel__badge">
                {user ? `欢迎回来 · ${user.nickname}` : 'simple_bilibili · 视频社区'}
              </span>
              <div>
                <h1 className="hero-carousel__title">
                  {user ? `你好，${user.nickname}` : '支持多清晰度 HLS 与分片直传'}
                </h1>
                <p className="hero-carousel__desc">
                  {user
                    ? `当前账号 UID ${user.id}，可用代币余额 ${user.coin_balance} 枚。你可以立刻前往创作中心上传视频，FFmpeg 自动转码为多清晰度 HLS 流畅播放。`
                    : '登录即可体验 8MB 分片并发直传、SHA-256 秒传、断点续传、异步多清晰度 HLS 转码与投稿管理。'}
                </p>
              </div>
            </div>

            <div className="hero-carousel__footer">
              <span>{user ? `已登录账号：${user.username}` : '注册即赠 10 枚初始代币'}</span>
              <div className="hero-carousel__actions">
                {user ? (
                  <>
                    <Link to="/upload" className="hero-cta hero-cta--primary">
                      立即投稿
                    </Link>
                    <Link to="/upload?tab=manage" className="hero-cta hero-cta--ghost">
                      稿件管理
                    </Link>
                  </>
                ) : (
                  <>
                    <button
                      type="button"
                      className="hero-cta hero-cta--primary"
                      onClick={() => openAuthModal('login')}
                    >
                      立即登录
                    </button>
                    <button
                      type="button"
                      className="hero-cta hero-cta--ghost"
                      onClick={() => openAuthModal('register')}
                    >
                      注册账号
                    </button>
                  </>
                )}
              </div>
            </div>
          </div>

          {loading ? (
            Array.from({ length: 6 }).map((_, idx) => (
              <div key={idx} className="video-slot video-slot--skeleton" aria-hidden="true">
                <div className="video-slot__cover bili-skeleton" />
                <div className="video-slot__line bili-skeleton" />
                <div className="video-slot__line video-slot__line--short bili-skeleton" />
              </div>
            ))
          ) : errorMsg ? (
            <div className="home-empty-card">{errorMsg}</div>
          ) : videos.length === 0 ? (
            <div className="home-empty-card">
              <div className="home-empty-card__title">
                {activeCategory ? `「${activeCategory.name}」分区暂时还没有视频` : '还没有已发布的视频'}
              </div>
              <p className="home-empty-card__desc">
                上传一段视频，体验分片直传与 FFmpeg 自动转码 HLS 播放吧。
              </p>
              <button
                type="button"
                className="home-empty-card__btn"
                onClick={() => (user ? (window.location.href = '/upload') : openAuthModal('login'))}
              >
                {user ? '去投稿第一个视频' : '登录后投稿'}
              </button>
            </div>
          ) : (
            videos.map((item) => <VideoCard key={item.bv_id} video={item} />)
          )}
        </section>

        {totalPages > 1 && (
          <div className="home-pagination">
            <button
              type="button"
              disabled={page <= 1}
              onClick={() => updateQuery({ page: String(page - 1) })}
            >
              上一页
            </button>
            <span>
              第 {page} / {totalPages} 页
            </span>
            <button
              type="button"
              disabled={page >= totalPages}
              onClick={() => updateQuery({ page: String(page + 1) })}
            >
              下一页
            </button>
          </div>
        )}
      </main>
    </div>
  )
}
