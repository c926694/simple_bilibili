import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { ApiError } from '../../api/client'
import { interactionApi } from '../../api/interaction'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { useAuth } from '../../context/AuthContext'
import type { WatchHistoryEntry } from '../../types/interaction'
import { formatDateShort, formatDuration } from '../../utils/format'
import './HistoryPage.css'

export function HistoryPage() {
  const { user, initializing, openAuthModal } = useAuth()
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [items, setItems] = useState<WatchHistoryEntry[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [notice, setNotice] = useState('')

  const loadHistory = useCallback(async () => {
    if (!user) {
      setLoading(false)
      return
    }
    setLoading(true)
    setNotice('')
    try {
      const res = await interactionApi.listWatchHistory({
        keyword: keyword || undefined,
        page,
        page_size: 20,
      })
      setItems(res.items)
      setTotal(res.total)
    } catch (err) {
      setNotice(err instanceof ApiError ? err.message : '加载观看历史失败')
    } finally {
      setLoading(false)
    }
  }, [user, keyword, page])

  useEffect(() => {
    void loadHistory()
  }, [loadHistory])

  const handleSearch = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setKeyword(keywordInput.trim())
  }

  const handleDeleteOne = async (bvid: string) => {
    try {
      await interactionApi.deleteWatchHistoryItem(bvid)
      setItems((prev) => prev.filter((item) => item.video.bv_id !== bvid))
      setTotal((prev) => Math.max(0, prev - 1))
    } catch (err) {
      setNotice(err instanceof ApiError ? err.message : '删除历史记录失败')
    }
  }

  const handleClearAll = async () => {
    if (!window.confirm('确认清空全部观看历史记录吗？')) {
      return
    }
    try {
      await interactionApi.clearWatchHistory()
      setItems([])
      setTotal(0)
      setPage(1)
    } catch (err) {
      setNotice(err instanceof ApiError ? err.message : '清空历史记录失败')
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / 20))

  return (
    <div className="history-page">
      <BiliHeader />

      <main className="history-container">
        {initializing ? (
          <div className="history-card history-empty">正在加载观看历史...</div>
        ) : !user ? (
          <div className="history-card history-empty">
            <h2>登录后查看观看历史</h2>
            <p>支持跨设备断点续播、按标题检索与一键清空。</p>
            <button
              type="button"
              className="history-primary-btn"
              onClick={() => openAuthModal('login')}
            >
              立即登录
            </button>
          </div>
        ) : (
          <section className="history-card">
            <div className="history-header">
              <div className="history-header__left">
                <h1>观看历史</h1>
                <span>共 {total} 条记录</span>
              </div>
              <div className="history-header__right">
                <form className="history-search" onSubmit={handleSearch}>
                  <input
                    type="search"
                    value={keywordInput}
                    onChange={(e) => setKeywordInput(e.target.value)}
                    placeholder="搜索观看过的视频标题"
                    aria-label="搜索历史记录"
                  />
                  <button type="submit">搜索</button>
                </form>
                <button
                  type="button"
                  className="history-clear-btn"
                  disabled={items.length === 0}
                  onClick={() => void handleClearAll()}
                >
                  清空历史
                </button>
              </div>
            </div>

            {notice && <div className="history-notice">{notice}</div>}

            {loading ? (
              <div className="history-list">
                {Array.from({ length: 4 }).map((_, idx) => (
                  <div key={idx} className="history-item history-item--skeleton">
                    <div className="bili-skeleton history-item__cover" />
                    <div className="history-item__body">
                      <div className="bili-skeleton history-skel-line" />
                      <div className="bili-skeleton history-skel-line history-skel-line--short" />
                    </div>
                  </div>
                ))}
              </div>
            ) : items.length === 0 ? (
              <div className="history-empty">
                <p>{keyword ? `没有找到包含“${keyword}”的观看记录` : '你还没有观看过任何视频'}</p>
                <Link to="/" className="history-primary-btn">
                  去首页逛逛
                </Link>
              </div>
            ) : (
              <>
                <div className="history-list">
                  {items.map((entry) => {
                    const duration = Math.max(1, entry.video.duration_sec || 1)
                    const percent = Math.min(100, Math.round((entry.progress_sec / duration) * 100))
                    const resumeHref = `/video/${entry.video.bv_id}?p=${entry.part_index || 1}&t=${entry.progress_sec}`
                    return (
                      <article key={entry.video.bv_id} className="history-item">
                        <Link to={resumeHref} className="history-item__cover">
                          {entry.video.cover_url ? (
                            <img src={entry.video.cover_url} alt={entry.video.title} loading="lazy" />
                          ) : (
                            <span>无封面</span>
                          )}
                          <div className="history-item__progress-track">
                            <div
                              className="history-item__progress-fill"
                              style={{ width: `${percent}%` }}
                            />
                          </div>
                        </Link>

                        <div className="history-item__body">
                          <Link to={resumeHref} className="history-item__title">
                            {entry.video.title}
                          </Link>
                          <div className="history-item__meta">
                            <span>
                              看到 P{entry.part_index || 1} {entry.part_title} ·{' '}
                              {formatDuration(entry.progress_sec)} /{' '}
                              {formatDuration(entry.video.duration_sec)}
                            </span>
                            <span>观看于 {formatDateShort(entry.watched_at)}</span>
                          </div>
                          <div className="history-item__author">
                            <Link to={`/space/${entry.video.author.id}`}>
                              UP：{entry.video.author.nickname}
                            </Link>
                            {entry.video.category && <span>· {entry.video.category.name}</span>}
                          </div>
                        </div>

                        <div className="history-item__actions">
                          <Link to={resumeHref} className="history-resume-link">
                            继续观看
                          </Link>
                          <button
                            type="button"
                            className="history-del-btn"
                            onClick={() => void handleDeleteOne(entry.video.bv_id)}
                          >
                            删除
                          </button>
                        </div>
                      </article>
                    )
                  })}
                </div>

                {totalPages > 1 && (
                  <div className="history-pagination">
                    <button
                      type="button"
                      disabled={page <= 1}
                      onClick={() => setPage((p) => p - 1)}
                    >
                      上一页
                    </button>
                    <span>
                      第 {page} / {totalPages} 页
                    </span>
                    <button
                      type="button"
                      disabled={page >= totalPages}
                      onClick={() => setPage((p) => p + 1)}
                    >
                      下一页
                    </button>
                  </div>
                )}
              </>
            )}
          </section>
        )}
      </main>
    </div>
  )
}
