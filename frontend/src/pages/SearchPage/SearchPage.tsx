import { useEffect, useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ApiError } from '../../api/client'
import { videoApi } from '../../api/video'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { UserAvatar } from '../../components/UserAvatar/UserAvatar'
import { VideoCard } from '../../components/VideoCard/VideoCard'
import type { Category, VideoSearchResponse, VideoSortOption } from '../../types/video'
import './SearchPage.css'

const SORT_TABS: { value: VideoSortOption; label: string }[] = [
  { value: 'hot', label: '综合排序' },
  { value: 'play', label: '最多播放' },
  { value: 'latest', label: '最新发布' },
  { value: 'danmaku', label: '最多弹幕' },
]

const PAGE_SIZE = 20

export function SearchPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const keyword = (searchParams.get('q') || '').trim()
  const sort = (searchParams.get('sort') as VideoSortOption) || 'hot'
  const categoryId = searchParams.get('category') ? Number(searchParams.get('category')) : undefined
  const page = Math.max(1, Number(searchParams.get('page') || '1'))

  const [inputValue, setInputValue] = useState(keyword)
  const [categories, setCategories] = useState<Category[]>([])
  const [result, setResult] = useState<VideoSearchResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => {
    setInputValue(keyword)
  }, [keyword])

  useEffect(() => {
    videoApi
      .listCategories()
      .then(setCategories)
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!keyword) {
      setResult(null)
      setErrorMsg('')
      return
    }
    let cancelled = false
    setLoading(true)
    setErrorMsg('')
    videoApi
      .searchVideos({
        keyword,
        category_id: categoryId,
        sort,
        page,
        page_size: PAGE_SIZE,
      })
      .then((data) => {
        if (!cancelled) {
          setResult(data)
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setResult(null)
          setErrorMsg(err instanceof ApiError ? err.message : '搜索请求失败')
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
  }, [keyword, categoryId, sort, page])

  const updateParams = (patch: Record<string, string | undefined>) => {
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

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    const trimmed = inputValue.trim()
    if (!trimmed) {
      return
    }
    updateParams({ q: trimmed, page: '1' })
  }

  const totalPages = result ? Math.max(1, Math.ceil(result.videos.total / PAGE_SIZE)) : 1

  return (
    <div className="search-page">
      <BiliHeader />

      <main className="search-page__container">
        <form className="search-hero-form" onSubmit={handleSubmit}>
          <input
            type="search"
            className="search-hero-form__input"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            placeholder="输入视频标题或 UP 主昵称进行搜索"
            aria-label="搜索关键词"
          />
          <button type="submit" className="search-hero-form__btn">
            搜索
          </button>
        </form>

        <div className="search-filters">
          <div className="search-filter-row">
            {SORT_TABS.map((tab) => (
              <button
                key={tab.value}
                type="button"
                className={`search-pill ${sort === tab.value ? 'search-pill--active' : ''}`}
                onClick={() => updateParams({ sort: tab.value, page: '1' })}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="search-filter-row">
            <button
              type="button"
              className={`search-pill ${!categoryId ? 'search-pill--active' : ''}`}
              onClick={() => updateParams({ category: undefined, page: '1' })}
            >
              全部分区
            </button>
            {categories.map((cat) => (
              <button
                key={cat.id}
                type="button"
                className={`search-pill ${categoryId === cat.id ? 'search-pill--active' : ''}`}
                onClick={() => updateParams({ category: String(cat.id), page: '1' })}
              >
                {cat.name}
              </button>
            ))}
          </div>
        </div>

        {!keyword ? (
          <div className="search-empty">请输入关键词开始搜索视频或 UP 主</div>
        ) : loading ? (
          <div className="search-videos-grid" aria-label={`正在搜索${keyword}`}>
            {Array.from({ length: 10 }).map((_, idx) => (
              <div key={idx} className="search-skeleton-card" aria-hidden="true">
                <div className="bili-skeleton search-skeleton-card__thumb" />
                <div className="bili-skeleton search-skeleton-card__line" />
                <div className="bili-skeleton search-skeleton-card__line search-skeleton-card__line--short" />
              </div>
            ))}
          </div>
        ) : errorMsg ? (
          <div className="search-empty search-empty--error">{errorMsg}</div>
        ) : result ? (
          <>
            {result.users.items.length > 0 && (
              <section className="search-section">
                <h2 className="search-section__title">
                  相关 UP 主（{result.users.total}）
                </h2>
                <div className="search-users-grid">
                  {result.users.items.map((up) => (
                    <Link key={up.id} to={`/space/${up.id}`} className="search-user-card">
                      <UserAvatar src={up.avatar_url} nickname={up.nickname} size={52} />
                      <div className="search-user-card__info">
                        <div className="search-user-card__name">{up.nickname}</div>
                        <div className="search-user-card__bio">
                          {up.bio || `粉丝 ${up.follower_count} · 关注 ${up.following_count}`}
                        </div>
                      </div>
                    </Link>
                  ))}
                </div>
              </section>
            )}

            <section className="search-section">
              <h2 className="search-section__title">
                相关视频（{result.videos.total}）
              </h2>
              {result.videos.items.length === 0 ? (
                <div className="search-empty">没有找到与“{keyword}”相关的视频</div>
              ) : (
                <>
                  <div className="search-videos-grid">
                    {result.videos.items.map((item) => (
                      <VideoCard key={item.bv_id} video={item} />
                    ))}
                  </div>

                  {totalPages > 1 && (
                    <div className="search-pagination">
                      <button
                        type="button"
                        disabled={page <= 1}
                        onClick={() => updateParams({ page: String(page - 1) })}
                      >
                        上一页
                      </button>
                      <span>
                        第 {page} / {totalPages} 页
                      </span>
                      <button
                        type="button"
                        disabled={page >= totalPages}
                        onClick={() => updateParams({ page: String(page + 1) })}
                      >
                        下一页
                      </button>
                    </div>
                  )}
                </>
              )}
            </section>
          </>
        ) : null}
      </main>
    </div>
  )
}
