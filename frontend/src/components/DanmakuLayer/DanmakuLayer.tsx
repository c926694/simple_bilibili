import { useEffect, useRef, useState } from 'react'
import type { DanmakuItem } from '../../types/danmaku'
import './DanmakuLayer.css'

interface ActiveDanmaku {
  key: string
  item: DanmakuItem
  track: number
  durationSec: number
}

interface DanmakuLayerProps {
  partId: string
  items: DanmakuItem[]
  liveQueue: DanmakuItem[]
  onConsumeLive: () => void
  currentTimeMs: number
  playing: boolean
  playbackRate: number
  enabled: boolean
  opacity: number
}

const SCROLL_TRACK_COUNT = 8
const FIXED_TRACK_COUNT = 4
const BASE_SCROLL_DURATION_SEC = 8.5
const BASE_FIXED_DURATION_SEC = 4.2

export function DanmakuLayer({
  partId,
  items,
  liveQueue,
  onConsumeLive,
  currentTimeMs,
  playing,
  playbackRate,
  enabled,
  opacity,
}: DanmakuLayerProps) {
  const [activeList, setActiveList] = useState<ActiveDanmaku[]>([])

  const emittedIdsRef = useRef<Set<string>>(new Set())
  const lastTimeMsRef = useRef<number>(-1)
  const scrollTrackFreeAtRef = useRef<number[]>(Array(SCROLL_TRACK_COUNT).fill(0))
  const topTrackFreeAtRef = useRef<number[]>(Array(FIXED_TRACK_COUNT).fill(0))
  const bottomTrackFreeAtRef = useRef<number[]>(Array(FIXED_TRACK_COUNT).fill(0))

  const allocTrack = (tracks: number[], holdMs: number): number => {
    const now = Date.now()
    let bestIdx = 0
    let earliest = tracks[0] ?? 0
    for (let i = 0; i < tracks.length; i += 1) {
      if (tracks[i] <= now) {
        tracks[i] = now + holdMs
        return i
      }
      if (tracks[i] < earliest) {
        earliest = tracks[i]
        bestIdx = i
      }
    }
    tracks[bestIdx] = now + holdMs
    return bestIdx
  }

  const spawnDanmaku = (list: DanmakuItem[]) => {
    if (list.length === 0) {
      return
    }
    const safeRate = Math.max(0.5, playbackRate || 1)
    const nextActive: ActiveDanmaku[] = []

    for (const item of list) {
      let track = 0
      let durationSec = BASE_SCROLL_DURATION_SEC / safeRate
      if (item.mode === 2) {
        durationSec = BASE_FIXED_DURATION_SEC / safeRate
        track = allocTrack(topTrackFreeAtRef.current, durationSec * 1000)
      } else if (item.mode === 3) {
        durationSec = BASE_FIXED_DURATION_SEC / safeRate
        track = allocTrack(bottomTrackFreeAtRef.current, durationSec * 1000)
      } else {
        // 滚动弹幕按字数估算尾部离开右边界的占道时间，减少重叠
        const occupyMs = Math.min(durationSec * 450, 1200 + item.content.length * 85)
        track = allocTrack(scrollTrackFreeAtRef.current, occupyMs)
      }
      nextActive.push({
        key: `${item.id}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
        item,
        track,
        durationSec,
      })
    }

    setActiveList((prev) => [...prev.slice(-80), ...nextActive])
  }

  // 切换分 P 或关闭弹幕时重置状态
  useEffect(() => {
    setActiveList([])
    emittedIdsRef.current.clear()
    lastTimeMsRef.current = -1
    scrollTrackFreeAtRef.current = Array(SCROLL_TRACK_COUNT).fill(0)
    topTrackFreeAtRef.current = Array(FIXED_TRACK_COUNT).fill(0)
    bottomTrackFreeAtRef.current = Array(FIXED_TRACK_COUNT).fill(0)
  }, [partId, enabled])

  // 随播放器时间轴触发历史弹幕，拖动进度条时重置轨道
  useEffect(() => {
    if (!enabled) {
      return
    }

    const prevTime = lastTimeMsRef.current
    const isFirstSync = prevTime < 0
    const jumpedBackward = !isFirstSync && currentTimeMs < prevTime - 600
    const jumpedForward = !isFirstSync && currentTimeMs > prevTime + 2500

    if (jumpedBackward || jumpedForward) {
      setActiveList([])
      emittedIdsRef.current.clear()
      scrollTrackFreeAtRef.current = Array(SCROLL_TRACK_COUNT).fill(0)
      topTrackFreeAtRef.current = Array(FIXED_TRACK_COUNT).fill(0)
      bottomTrackFreeAtRef.current = Array(FIXED_TRACK_COUNT).fill(0)
      lastTimeMsRef.current = currentTimeMs
      return
    }

    if (!playing) {
      lastTimeMsRef.current = currentTimeMs
      return
    }

    const windowStart = isFirstSync ? Math.max(0, currentTimeMs - 200) : prevTime
    const windowEnd = currentTimeMs + 120
    const due: DanmakuItem[] = []

    for (const item of items) {
      if (item.part_id !== partId) {
        continue
      }
      if (emittedIdsRef.current.has(item.id)) {
        continue
      }
      if (item.time_offset_ms >= windowStart && item.time_offset_ms <= windowEnd) {
        emittedIdsRef.current.add(item.id)
        due.push(item)
      }
    }

    lastTimeMsRef.current = currentTimeMs
    if (due.length > 0) {
      spawnDanmaku(due)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentTimeMs, playing, items, partId, enabled, playbackRate])

  // 处理实时收到的或自己刚发送的弹幕，立刻展示在画面上
  useEffect(() => {
    if (liveQueue.length === 0) {
      return
    }
    if (enabled) {
      const immediate: DanmakuItem[] = []
      for (const item of liveQueue) {
        if (item.part_id !== partId) {
          continue
        }
        const nearNow = Math.abs(item.time_offset_ms - currentTimeMs) <= 2500
        if ((item.isSelf || nearNow) && !emittedIdsRef.current.has(item.id)) {
          emittedIdsRef.current.add(item.id)
          immediate.push(item)
        }
      }
      if (immediate.length > 0) {
        spawnDanmaku(immediate)
      }
    }
    onConsumeLive()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [liveQueue, enabled, partId, currentTimeMs])

  if (!enabled) {
    return null
  }

  return (
    <div
      className="danmaku-layer"
      style={{ opacity }}
      aria-label="视频弹幕层"
      data-testid="danmaku-layer"
    >
      {activeList.map((entry) => {
        const { item, track, durationSec } = entry
        const isFixedTop = item.mode === 2
        const isFixedBottom = item.mode === 3
        const modeClass = isFixedTop
          ? 'danmaku-item--top'
          : isFixedBottom
            ? 'danmaku-item--bottom'
            : 'danmaku-item--scroll'

        const posStyle = isFixedBottom
          ? { bottom: `${16 + track * 34}px` }
          : { top: `${12 + track * 34}px` }

        // 本人刚发出的实时弹幕即使播放器处于暂停状态也允许平滑展示
        const runState = playing || item.isSelf ? 'running' : 'paused'

        return (
          <span
            key={entry.key}
            className={`danmaku-item ${modeClass} ${item.isSelf ? 'danmaku-item--self' : ''}`}
            style={{
              ...posStyle,
              color: item.color || '#FFFFFF',
              fontSize: `${item.font_size || 25}px`,
              animationDuration: `${durationSec}s`,
              animationPlayState: runState,
            }}
            onAnimationEnd={() => {
              setActiveList((prev) => prev.filter((x) => x.key !== entry.key))
            }}
          >
            {item.content}
          </span>
        )
      })}
    </div>
  )
}
