import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { ApiError } from '../../api/client'
import { videoApi } from '../../api/video'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { HlsPlayer } from '../../components/HlsPlayer/HlsPlayer'
import { UserAvatar } from '../../components/UserAvatar/UserAvatar'
import { useAuth } from '../../context/AuthContext'
import type { VideoDetail, VideoSummary } from '../../types/video'
import {
  formatCount,
  formatDateShort,
  formatDuration,
  getTranscodeStatusLabel,
  getVideoStatusMeta,
} from '../../utils/format'
import './VideoPage.css'

export function VideoPage() {
  const { bvid } = useParams<{ bvid: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const { user } = useAuth()

  const [video, setVideo] = useState<VideoDetail | null>(null)
  const [related, setRelated] = useState<VideoSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [errorMsg, setErrorMsg] = useState('')
  const [retrying, setRetrying] = useState(false)
  const [descExpanded, setDescExpanded] = useState(false)

  const loadDetail = useCallback(async () => {
    if (!bvid) {
      setErrorMsg('缺少视频编码')
      setLoading(false)
      return
    }
    try {
      const detail = await videoApi.getVideoDetail(bvid)
      setVideo(detail)
      setErrorMsg('')
    } catch (err) {
      setVideo(null)
      setErrorMsg(err instanceof ApiError ? err.message : '加载视频详情失败')
    } finally {
      setLoading(false)
    }
  }, [bvid])

  useEffect(() => {
    setLoading(true)
    void loadDetail()
  }, [loadDetail])

  // 若本人查看转码中的视频，每 3 秒轮询直到转码结束
  useEffect(() => {
    if (!video || video.status !== 1) {
      return
    }
    const timer = window.setInterval(() => {
      void loadDetail()
    }, 3000)
    return () => window.clearInterval(timer)
  }, [video, loadDetail])

  // 加载同分区相关视频
  useEffect(() => {
    if (!video) {
      return
    }
    let cancelled = false
    videoApi
      .listVideos({
        category_id: video.category?.id,
        sort: 'latest',
        page: 1,
        page_size: 8,
      })
      .then((res) => {
        if (!cancelled) {
          setRelated(res.items.filter((item) => item.bv_id !== video.bv_id).slice(0, 6))
        }
      })
      .catch(() => {
        if (!cancelled) {
          setRelated([])
        }
      })
    return () => {
      cancelled = true
    }
  }, [video?.bv_id, video?.category?.id])

  const requestedPartIndex = Number(searchParams.get('p') || '1')
  const activePart = useMemo(() => {
    if (!video || !video.parts || video.parts.length === 0) {
      return null
    }
    return video.parts.find((item) => item.part_index === requestedPartIndex) ?? video.parts[0]
  }, [video, requestedPartIndex])

  const isAuthor = Boolean(user && video && user.id === video.author.id)

  const handleSelectPart = (partIndex: number) => {
    setSearchParams({ p: String(partIndex) })
  }

  const handleRetryTranscode = async () => {
    if (!video || !activePart) {
      return
    }
    setRetrying(true)
    try {
      await videoApi.retryPartTranscode(video.bv_id, activePart.id)
      await loadDetail()
    } catch (err) {
      setErrorMsg(err instanceof ApiError ? err.message : '重新触发转码失败')
    } finally {
      setRetrying(false)
    }
  }

  return (
    <div className="video-page">
      <BiliHeader />

      <main className="video-page__container">
        {loading ? (
          <div className="video-page__state-card">正在加载视频内容...</div>
        ) : errorMsg || !video ? (
          <div className="video-page__state-card">
            <h2>{errorMsg || '视频不存在或已下架'}</h2>
            <Link to="/" className="video-page__back-btn">
              返回首页
            </Link>
          </div>
        ) : (
          <div className="video-page__layout">
            <section className="video-page__main">
              <div className="video-page__header">
                <div className="video-page__title-row">
                  <h1 className="video-page__title">{video.title}</h1>
                  {video.status !== 3 && (
                    <span
                      className={`video-status-pill video-status-pill--${getVideoStatusMeta(video.status).tone}`}
                    >
                      {getVideoStatusMeta(video.status).label}
                    </span>
                  )}
                </div>
                <div className="video-page__meta-bar">
                  <span>播放 {formatCount(video.stat.play_count)}</span>
                  <span>弹幕 {formatCount(video.stat.danmaku_count)}</span>
                  <span>{formatDateShort(video.published_at || video.created_at)}</span>
                  <span className="video-page__bvid">{video.bv_id}</span>
                  {video.category && (
                    <Link
                      to={`/?category=${video.category.id}`}
                      className="video-page__category-link"
                    >
                      {video.category.name}
                    </Link>
                  )}
                </div>
              </div>

              <HlsPlayer
                part={activePart}
                poster={video.cover_url}
                canRetry={isAuthor}
                retrying={retrying}
                onRetryTranscode={() => void handleRetryTranscode()}
              />

              <div className="video-page__toolbar">
                <div className="video-page__toolbar-left">
                  <span className="video-action-item">
                    点赞 <strong>{formatCount(video.stat.like_count)}</strong>
                  </span>
                  <span className="video-action-item">
                    投币 <strong>{formatCount(video.stat.coin_count)}</strong>
                  </span>
                  <span className="video-action-item">
                    收藏 <strong>{formatCount(video.stat.favorite_count)}</strong>
                  </span>
                  <span className="video-action-item">
                    转发 <strong>{formatCount(video.stat.share_count)}</strong>
                  </span>
                </div>
                {isAuthor && (
                  <Link to={`/upload?tab=manage&edit=${video.bv_id}`} className="video-edit-link">
                    编辑稿件
                  </Link>
                )}
              </div>

              <div className="video-page__desc-card">
                <p
                  className={`video-page__desc ${descExpanded ? 'video-page__desc--expanded' : ''}`}
                >
                  {video.description || '作者还没有填写视频简介。'}
                </p>
                {video.description && video.description.length > 140 && (
                  <button
                    type="button"
                    className="video-page__desc-toggle"
                    onClick={() => setDescExpanded((prev) => !prev)}
                  >
                    {descExpanded ? '收起简介' : '展开更多'}
                  </button>
                )}

                {video.tags && video.tags.length > 0 && (
                  <div className="video-page__tags">
                    {video.tags.map((tag) => (
                      <Link
                        key={tag}
                        to={`/search?q=${encodeURIComponent(tag)}`}
                        className="video-tag-chip"
                      >
                        {tag}
                      </Link>
                    ))}
                  </div>
                )}
              </div>
            </section>

            <aside className="video-page__side">
              <div className="video-author-card">
                <Link to={`/space/${video.author.id}`} className="video-author-card__avatar">
                  <UserAvatar
                    src={video.author.avatar_url}
                    nickname={video.author.nickname}
                    size={48}
                  />
                </Link>
                <div className="video-author-card__info">
                  <Link to={`/space/${video.author.id}`} className="video-author-card__name">
                    {video.author.nickname}
                  </Link>
                  <p className="video-author-card__bio">
                    {video.author.bio || `UP 主 UID ${video.author.id}`}
                  </p>
                  <Link to={`/space/${video.author.id}`} className="video-author-card__btn">
                    访问空间
                  </Link>
                </div>
              </div>

              {video.parts.length > 0 && (
                <div className="video-parts-card">
                  <div className="video-parts-card__head">
                    <h2>视频选集</h2>
                    <span>
                      ({activePart?.part_index ?? 1}/{video.parts.length})
                    </span>
                  </div>
                  <ul className="video-parts-list">
                    {video.parts.map((item) => {
                      const active = activePart?.id === item.id
                      return (
                        <li key={item.id}>
                          <button
                            type="button"
                            className={`video-part-item ${active ? 'video-part-item--active' : ''}`}
                            onClick={() => handleSelectPart(item.part_index)}
                          >
                            <span className="video-part-item__index">P{item.part_index}</span>
                            <span className="video-part-item__title">{item.title}</span>
                            <span className="video-part-item__right">
                              {item.transcode_status === 2
                                ? formatDuration(item.duration_sec)
                                : getTranscodeStatusLabel(item.transcode_status)}
                            </span>
                          </button>
                        </li>
                      )
                    })}
                  </ul>
                </div>
              )}

              <div className="video-related-card">
                <h2 className="video-related-card__title">同分区推荐</h2>
                {related.length === 0 ? (
                  <div className="video-related-empty">暂无更多相关视频</div>
                ) : (
                  <div className="video-related-list">
                    {related.map((item) => (
                      <Link
                        key={item.bv_id}
                        to={`/video/${item.bv_id}`}
                        className="video-related-item"
                      >
                        <div className="video-related-item__cover">
                          {item.cover_url ? (
                            <img src={item.cover_url} alt={item.title} loading="lazy" />
                          ) : (
                            <span className="video-related-item__placeholder">无封面</span>
                          )}
                          <span className="video-related-item__dur">
                            {formatDuration(item.duration_sec)}
                          </span>
                        </div>
                        <div className="video-related-item__info">
                          <div className="video-related-item__title">{item.title}</div>
                          <div className="video-related-item__meta">{item.author.nickname}</div>
                          <div className="video-related-item__meta">
                            播放 {formatCount(item.stat.play_count)} · 弹幕{' '}
                            {formatCount(item.stat.danmaku_count)}
                          </div>
                        </div>
                      </Link>
                    ))}
                  </div>
                )}
              </div>
            </aside>
          </div>
        )}
      </main>
    </div>
  )
}
