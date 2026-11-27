import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { ApiError, tokenStorage } from '../../api/client'
import { buildVideoRoomWsUrl, DANMAKU_SEGMENT_MS, danmakuApi } from '../../api/danmaku'
import { videoApi } from '../../api/video'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { DanmakuBar } from '../../components/DanmakuBar/DanmakuBar'
import { DanmakuLayer } from '../../components/DanmakuLayer/DanmakuLayer'
import { HlsPlayer } from '../../components/HlsPlayer/HlsPlayer'
import { UserAvatar } from '../../components/UserAvatar/UserAvatar'
import { useAuth } from '../../context/AuthContext'
import type {
  DanmakuItem,
  DanmakuStyleConfig,
  VideoRoomWsMessage,
} from '../../types/danmaku'
import type { VideoDetail, VideoSummary } from '../../types/video'
import {
  formatCount,
  formatDateShort,
  formatDuration,
  getTranscodeStatusLabel,
  getVideoStatusMeta,
} from '../../utils/format'
import './VideoPage.css'

function mergeDanmakuItems(prev: DanmakuItem[], incoming: DanmakuItem[]): DanmakuItem[] {
  const byId = new Map<string, DanmakuItem>()
  for (const item of prev) {
    byId.set(item.id, item)
  }
  for (const item of incoming) {
    const existing = byId.get(item.id)
    byId.set(item.id, existing?.isSelf ? { ...item, isSelf: true } : item)
  }
  return Array.from(byId.values()).sort((a, b) => {
    if (a.time_offset_ms !== b.time_offset_ms) {
      return a.time_offset_ms - b.time_offset_ms
    }
    return a.id.localeCompare(b.id)
  })
}

export function VideoPage() {
  const { bvid } = useParams<{ bvid: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const { user, openAuthModal } = useAuth()

  const [video, setVideo] = useState<VideoDetail | null>(null)
  const [related, setRelated] = useState<VideoSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [errorMsg, setErrorMsg] = useState('')
  const [retrying, setRetrying] = useState(false)
  const [descExpanded, setDescExpanded] = useState(false)

  // 播放器状态与弹幕状态
  const [playerTimeMs, setPlayerTimeMs] = useState(0)
  const [playerPlaying, setPlayerPlaying] = useState(false)
  const [playerRate, setPlayerRate] = useState(1)

  const [danmakuEnabled, setDanmakuEnabled] = useState(true)
  const [danmakuStyle, setDanmakuStyle] = useState<DanmakuStyleConfig>({
    mode: 1,
    fontSize: 25,
    color: '#FFFFFF',
    opacity: 0.9,
  })
  const [onlineCount, setOnlineCount] = useState(1)
  const [danmakuItems, setDanmakuItems] = useState<DanmakuItem[]>([])
  const [liveQueue, setLiveQueue] = useState<DanmakuItem[]>([])
  const [danmakuSending, setDanmakuSending] = useState(false)
  const [danmakuError, setDanmakuError] = useState('')

  const loadedSegmentsRef = useRef<Set<number>>(new Set())
  const segmentDurationRef = useRef<number>(DANMAKU_SEGMENT_MS)
  const activePartIdRef = useRef<string>('')

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

  useEffect(() => {
    activePartIdRef.current = activePart?.id ?? ''
  }, [activePart?.id])

  const loadSegment = useCallback(
    async (targetBvid: string, partId: string, segment: number) => {
      if (loadedSegmentsRef.current.has(segment)) {
        return
      }
      loadedSegmentsRef.current.add(segment)
      try {
        const res = await danmakuApi.listSegment(targetBvid, partId, segment)
        if (activePartIdRef.current !== partId) {
          return
        }
        if (res.segment_duration_ms > 0) {
          segmentDurationRef.current = res.segment_duration_ms
        }
        if (res.online_count > 0) {
          setOnlineCount(res.online_count)
        }
        if (res.items.length > 0) {
          setDanmakuItems((prev) => mergeDanmakuItems(prev, res.items))
        }
      } catch {
        loadedSegmentsRef.current.delete(segment)
      }
    },
    [],
  )

  // 切换视频或分 P 时清空弹幕池并拉取第 0 段历史弹幕
  useEffect(() => {
    loadedSegmentsRef.current.clear()
    setDanmakuItems([])
    setLiveQueue([])
    setDanmakuError('')
    if (!video || video.status !== 3 || !activePart) {
      return
    }
    void loadSegment(video.bv_id, activePart.id, 0)
  }, [video?.bv_id, video?.status, activePart?.id, loadSegment])

  // 播放进度推进或拖动进度条时按需拉取当前分段并预拉取下一分段
  useEffect(() => {
    if (!video || video.status !== 3 || !activePart) {
      return
    }
    const segMs = segmentDurationRef.current || DANMAKU_SEGMENT_MS
    const currentSegment = Math.floor(Math.max(0, playerTimeMs) / segMs)
    void loadSegment(video.bv_id, activePart.id, currentSegment)

    const remainInSegment = (currentSegment + 1) * segMs - playerTimeMs
    const totalMs = (activePart.duration_sec || 0) * 1000
    if (remainInSegment <= 20000 && (currentSegment + 1) * segMs < totalMs) {
      void loadSegment(video.bv_id, activePart.id, currentSegment + 1)
    }
  }, [video?.bv_id, video?.status, activePart?.id, activePart?.duration_sec, playerTimeMs, loadSegment])

  // 建立视频房间 WebSocket 长连接（含心跳与断线重连）
  useEffect(() => {
    if (!video || video.status !== 3) {
      return
    }
    let closedByEffect = false
    let socket: WebSocket | null = null
    let pingTimer: number | null = null
    let reconnectTimer: number | null = null
    let attempt = 0

    const clearTimers = () => {
      if (pingTimer !== null) {
        window.clearInterval(pingTimer)
        pingTimer = null
      }
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
    }

    const connect = () => {
      if (closedByEffect) {
        return
      }
      const token = tokenStorage.getAccessToken()
      const protocols = token ? ['bilibili', `bearer.${token}`] : undefined
      const ws = protocols
        ? new WebSocket(buildVideoRoomWsUrl(video.bv_id), protocols)
        : new WebSocket(buildVideoRoomWsUrl(video.bv_id))
      socket = ws

      ws.onopen = () => {
        attempt = 0
        pingTimer = window.setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'ping' }))
          }
        }, 25000)
      }

      ws.onmessage = (event) => {
        try {
          const payload = JSON.parse(String(event.data)) as VideoRoomWsMessage
          if (payload.type === 'online_count') {
            setOnlineCount(Math.max(1, Number(payload.data.count || 1)))
          } else if (payload.type === 'danmaku' && payload.data) {
            const incoming = payload.data
            if (incoming.part_id === activePartIdRef.current) {
              setDanmakuItems((prev) => mergeDanmakuItems(prev, [incoming]))
              setLiveQueue((prev) => [...prev, incoming])
            }
          }
        } catch {
          // 忽略无法解析的帧
        }
      }

      ws.onclose = () => {
        clearTimers()
        if (!closedByEffect) {
          attempt += 1
          const delayMs = Math.min(10000, 1000 * attempt)
          reconnectTimer = window.setTimeout(connect, delayMs)
        }
      }
    }

    connect()

    return () => {
      closedByEffect = true
      clearTimers()
      if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
        socket.close()
      }
    }
  }, [video?.bv_id, video?.status, user?.id])

  const handleConsumeLive = useCallback(() => {
    setLiveQueue([])
  }, [])

  const handleSendDanmaku = async (content: string): Promise<boolean> => {
    if (!video || !activePart) {
      return false
    }
    setDanmakuError('')
    setDanmakuSending(true)
    try {
      const created = await danmakuApi.send(video.bv_id, {
        part_id: activePart.id,
        content,
        time_offset_ms: Math.max(0, Math.round(playerTimeMs)),
        mode: danmakuStyle.mode,
        font_size: danmakuStyle.fontSize,
        color: danmakuStyle.color,
      })
      const selfItem: DanmakuItem = { ...created, isSelf: true }
      setDanmakuItems((prev) => mergeDanmakuItems(prev, [selfItem]))
      setLiveQueue((prev) => [...prev, selfItem])
      setVideo((prev) =>
        prev
          ? {
              ...prev,
              stat: {
                ...prev.stat,
                danmaku_count: (prev.stat?.danmaku_count ?? 0) + 1,
              },
            }
          : prev,
      )
      return true
    } catch (err) {
      setDanmakuError(err instanceof ApiError ? err.message : '发送弹幕失败，请稍后重试')
      return false
    } finally {
      setDanmakuSending(false)
    }
  }

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
          <div className="video-page__layout" aria-label="正在加载视频内容">
            <section className="video-page__main">
              <div className="bili-skeleton video-skeleton__title" />
              <div className="bili-skeleton video-skeleton__meta" />
              <div className="bili-skeleton video-skeleton__player" />
              <div className="bili-skeleton video-skeleton__bar" />
              <div className="bili-skeleton video-skeleton__desc" />
            </section>
            <aside className="video-page__side">
              <div className="bili-skeleton video-skeleton__author" />
              <div className="bili-skeleton video-skeleton__parts" />
              <div className="bili-skeleton video-skeleton__related" />
            </aside>
          </div>
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
                onTimeChange={setPlayerTimeMs}
                onPlayStateChange={setPlayerPlaying}
                onRateChange={setPlayerRate}
                overlay={
                  activePart && video.status === 3 ? (
                    <DanmakuLayer
                      partId={activePart.id}
                      items={danmakuItems}
                      liveQueue={liveQueue}
                      onConsumeLive={handleConsumeLive}
                      currentTimeMs={playerTimeMs}
                      playing={playerPlaying}
                      playbackRate={playerRate}
                      enabled={danmakuEnabled}
                      opacity={danmakuStyle.opacity}
                    />
                  ) : null
                }
                footer={
                  video.status === 3 && activePart ? (
                    <DanmakuBar
                      onlineCount={onlineCount}
                      loadedCount={danmakuItems.length}
                      enabled={danmakuEnabled}
                      onToggleEnabled={() => setDanmakuEnabled((prev) => !prev)}
                      styleConfig={danmakuStyle}
                      onChangeStyle={setDanmakuStyle}
                      isLoggedIn={Boolean(user)}
                      disabled={activePart.transcode_status !== 2}
                      sending={danmakuSending}
                      errorMsg={danmakuError}
                      onRequireLogin={() => openAuthModal('login')}
                      onSend={handleSendDanmaku}
                    />
                  ) : null
                }
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
