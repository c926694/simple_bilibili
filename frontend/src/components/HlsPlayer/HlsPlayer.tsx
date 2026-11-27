import Hls from 'hls.js'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import type { VideoPart } from '../../types/video'
import { formatDuration } from '../../utils/format'
import './HlsPlayer.css'

interface QualityOption {
  key: '1080p' | '720p' | '480p'
  label: string
  url: string
}

const SPEED_OPTIONS = [0.5, 0.75, 1, 1.25, 1.5, 2]

interface HlsPlayerProps {
  part: VideoPart | null
  poster?: string
  canRetry?: boolean
  retrying?: boolean
  onRetryTranscode?: () => void
  onTimeChange?: (timeMs: number) => void
  onPlayStateChange?: (playing: boolean) => void
  onRateChange?: (rate: number) => void
  overlay?: ReactNode
  footer?: ReactNode
}

export function HlsPlayer({
  part,
  poster,
  canRetry = false,
  retrying = false,
  onRetryTranscode,
  onTimeChange,
  onPlayStateChange,
  onRateChange,
  overlay,
  footer,
}: HlsPlayerProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const hlsRef = useRef<Hls | null>(null)
  const pendingResumeRef = useRef<{ time: number; shouldPlay: boolean } | null>(null)

  const qualities = useMemo<QualityOption[]>(() => {
    if (!part) {
      return []
    }
    const list: QualityOption[] = []
    if (part.hls_1080p_url) {
      list.push({ key: '1080p', label: '1080P 高清', url: part.hls_1080p_url })
    }
    if (part.hls_720p_url) {
      list.push({ key: '720p', label: '720P 高清', url: part.hls_720p_url })
    }
    if (part.hls_480p_url) {
      list.push({ key: '480p', label: '480P 清晰', url: part.hls_480p_url })
    }
    return list
  }, [part])

  const [selectedQualityKey, setSelectedQualityKey] = useState<QualityOption['key'] | ''>('')
  const [playbackRate, setPlaybackRate] = useState(1)
  const [playing, setPlaying] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(0.85)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [playerError, setPlayerError] = useState('')

  const activeQuality = useMemo(() => {
    if (qualities.length === 0) {
      return null
    }
    return qualities.find((q) => q.key === selectedQualityKey) ?? qualities[0]
  }, [qualities, selectedQualityKey])

  // 切换分 P 时重置播放状态并默认选中最高可用清晰度
  useEffect(() => {
    setSelectedQualityKey(qualities[0]?.key ?? '')
    setCurrentTime(0)
    setDuration(part?.duration_sec ?? 0)
    setPlaying(false)
    onTimeChange?.(0)
    onPlayStateChange?.(false)
    setPlayerError('')
    pendingResumeRef.current = null
  }, [part?.id, part?.duration_sec, qualities, onTimeChange, onPlayStateChange])

  // 加载当前清晰度的 HLS 流
  useEffect(() => {
    const videoEl = videoRef.current
    if (!videoEl || !activeQuality) {
      return
    }

    setPlayerError('')
    const resumeState = pendingResumeRef.current
    pendingResumeRef.current = null

    if (hlsRef.current) {
      hlsRef.current.destroy()
      hlsRef.current = null
    }

    const applyResume = () => {
      videoEl.playbackRate = playbackRate
      videoEl.volume = volume
      videoEl.muted = muted
      if (resumeState) {
        if (resumeState.time > 0) {
          videoEl.currentTime = resumeState.time
        }
        if (resumeState.shouldPlay) {
          void videoEl.play().catch(() => {})
        }
      }
    }

    if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
      })
      hlsRef.current = hls
      hls.loadSource(activeQuality.url)
      hls.attachMedia(videoEl)
      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        applyResume()
      })
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) {
          if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
            hls.startLoad()
          } else if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
            hls.recoverMediaError()
          } else {
            setPlayerError('视频流加载失败，请刷新页面或切换清晰度重试')
            hls.destroy()
          }
        }
      })
    } else if (videoEl.canPlayType('application/vnd.apple.mpegurl')) {
      videoEl.src = activeQuality.url
      const onLoaded = () => applyResume()
      videoEl.addEventListener('loadedmetadata', onLoaded, { once: true })
    } else {
      setPlayerError('当前浏览器不支持 HLS 视频播放')
    }

    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy()
        hlsRef.current = null
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeQuality?.url])

  useEffect(() => {
    const handleFsChange = () => {
      setIsFullscreen(Boolean(document.fullscreenElement))
    }
    document.addEventListener('fullscreenchange', handleFsChange)
    return () => document.removeEventListener('fullscreenchange', handleFsChange)
  }, [])

  const handleTogglePlay = () => {
    const videoEl = videoRef.current
    if (!videoEl) {
      return
    }
    if (videoEl.paused) {
      void videoEl.play().catch(() => {})
    } else {
      videoEl.pause()
    }
  }

  const handleSeek = (value: number) => {
    const videoEl = videoRef.current
    if (!videoEl) {
      return
    }
    videoEl.currentTime = value
    setCurrentTime(value)
    onTimeChange?.(Math.round(value * 1000))
  }

  const handleVolumeChange = (nextVolume: number) => {
    const videoEl = videoRef.current
    setVolume(nextVolume)
    const nextMuted = nextVolume === 0
    setMuted(nextMuted)
    if (videoEl) {
      videoEl.volume = nextVolume
      videoEl.muted = nextMuted
    }
  }

  const handleToggleMute = () => {
    const videoEl = videoRef.current
    const nextMuted = !muted
    setMuted(nextMuted)
    if (videoEl) {
      videoEl.muted = nextMuted
    }
  }

  const handleSpeedChange = (rate: number) => {
    setPlaybackRate(rate)
    onRateChange?.(rate)
    if (videoRef.current) {
      videoRef.current.playbackRate = rate
    }
  }

  const handleQualityChange = (key: QualityOption['key']) => {
    if (key === activeQuality?.key) {
      return
    }
    const videoEl = videoRef.current
    if (videoEl) {
      pendingResumeRef.current = {
        time: videoEl.currentTime,
        shouldPlay: !videoEl.paused,
      }
    }
    setSelectedQualityKey(key)
  }

  const handleToggleFullscreen = async () => {
    const container = containerRef.current
    if (!container) {
      return
    }
    if (document.fullscreenElement) {
      await document.exitFullscreen().catch(() => {})
    } else {
      await container.requestFullscreen?.().catch(() => {})
    }
  }

  if (!part) {
    return (
      <div className="hls-player hls-player--empty">
        <div className="hls-player__state">暂无可播放的视频分 P</div>
      </div>
    )
  }

  if (part.transcode_status !== 2 || !activeQuality) {
    const isFailed = part.transcode_status === 3
    return (
      <div className="hls-player hls-player--empty">
        <div className="hls-player__state">
          <div className="hls-player__state-title">
            {isFailed ? '该分 P 转码失败' : '视频正在后台转码中...'}
          </div>
          <p className="hls-player__state-desc">
            {isFailed
              ? '原视频解析或转码异常，作者可点击重新转码或检查源文件格式。'
              : 'FFmpeg 正在生成多清晰度 HLS 切片，转码完成后即可流畅播放。'}
          </p>
          {isFailed && canRetry && onRetryTranscode && (
            <button
              type="button"
              className="hls-player__retry-btn"
              onClick={onRetryTranscode}
              disabled={retrying}
            >
              {retrying ? '正在重新提交...' : '重新转码'}
            </button>
          )}
        </div>
      </div>
    )
  }

  const maxDuration = duration > 0 ? duration : part.duration_sec || 1

  return (
    <div ref={containerRef} className="hls-player-shell">
      <div className="hls-player">
        <div className="hls-player__stage" onClick={handleTogglePlay}>
          <video
            ref={videoRef}
            className="hls-player__video"
            poster={poster || undefined}
            playsInline
            onPlay={() => {
              setPlaying(true)
              onPlayStateChange?.(true)
            }}
            onPause={() => {
              setPlaying(false)
              onPlayStateChange?.(false)
            }}
            onEnded={() => {
              setPlaying(false)
              onPlayStateChange?.(false)
            }}
            onTimeUpdate={(e) => {
              const sec = e.currentTarget.currentTime
              setCurrentTime(sec)
              onTimeChange?.(Math.round(sec * 1000))
            }}
            onLoadedMetadata={(e) => {
              if (Number.isFinite(e.currentTarget.duration)) {
                setDuration(e.currentTarget.duration)
              }
            }}
          />
          {overlay}
          {!playing && !playerError && (
            <button
              type="button"
              className="hls-player__center-play"
              aria-label="播放视频"
              onClick={(e) => {
                e.stopPropagation()
                handleTogglePlay()
              }}
            >
              <svg width="28" height="28" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M8 5.5v13l11-6.5-11-6.5z" />
              </svg>
            </button>
          )}
          {playerError && <div className="hls-player__error">{playerError}</div>}
        </div>

        <div className="hls-player__controls">
          <div className="hls-player__progress-row">
            <input
              type="range"
              className="hls-player__seek"
              min={0}
              max={maxDuration}
              step={0.1}
              value={Math.min(currentTime, maxDuration)}
              onChange={(e) => handleSeek(Number(e.target.value))}
              aria-label="播放进度"
            />
          </div>

          <div className="hls-player__bar">
            <div className="hls-player__bar-left">
              <button
                type="button"
                className="hls-player__ctrl-btn"
                onClick={handleTogglePlay}
                aria-label={playing ? '暂停' : '播放'}
              >
                {playing ? '暂停' : '播放'}
              </button>
              <span className="hls-player__time">
                {formatDuration(currentTime)} / {formatDuration(maxDuration)}
              </span>
            </div>

            <div className="hls-player__bar-right">
              <label className="hls-player__select-wrap">
                <span className="hls-player__sr-only">清晰度</span>
                <select
                  className="hls-player__select"
                  value={activeQuality.key}
                  onChange={(e) => handleQualityChange(e.target.value as QualityOption['key'])}
                  aria-label="切换清晰度"
                >
                  {qualities.map((q) => (
                    <option key={q.key} value={q.key}>
                      {q.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="hls-player__select-wrap">
                <span className="hls-player__sr-only">倍速</span>
                <select
                  className="hls-player__select"
                  value={playbackRate}
                  onChange={(e) => handleSpeedChange(Number(e.target.value))}
                  aria-label="切换倍速"
                >
                  {SPEED_OPTIONS.map((rate) => (
                    <option key={rate} value={rate}>
                      {rate === 1 ? '倍速 1.0x' : `${rate}x`}
                    </option>
                  ))}
                </select>
              </label>

              <button
                type="button"
                className="hls-player__ctrl-btn"
                onClick={handleToggleMute}
                aria-label={muted ? '取消静音' : '静音'}
              >
                {muted || volume === 0 ? '静音' : '音量'}
              </button>
              <input
                type="range"
                className="hls-player__volume"
                min={0}
                max={1}
                step={0.05}
                value={muted ? 0 : volume}
                onChange={(e) => handleVolumeChange(Number(e.target.value))}
                aria-label="音量滑块"
              />

              <button
                type="button"
                className="hls-player__ctrl-btn"
                onClick={() => void handleToggleFullscreen()}
                aria-label={isFullscreen ? '退出全屏' : '全屏'}
              >
                {isFullscreen ? '退出全屏' : '全屏'}
              </button>
            </div>
          </div>
        </div>
      </div>
      {footer}
    </div>
  )
}
