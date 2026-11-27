import { useEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { subscribePendingRequests } from '../../api/client'
import './RouteProgressBar.css'

const MIN_VISIBLE_MS = 240
const FADE_OUT_MS = 220

export function RouteProgressBar() {
  const location = useLocation()
  const [visible, setVisible] = useState(false)
  const [fading, setFading] = useState(false)
  const [progress, setProgress] = useState(0)

  const pendingRef = useRef(0)
  const startedAtRef = useRef(0)
  const mountedRef = useRef(false)
  const trickleTimerRef = useRef<number | null>(null)
  const finishTimerRef = useRef<number | null>(null)
  const hideTimerRef = useRef<number | null>(null)

  const clearTimers = () => {
    if (trickleTimerRef.current !== null) {
      window.clearInterval(trickleTimerRef.current)
      trickleTimerRef.current = null
    }
    if (finishTimerRef.current !== null) {
      window.clearTimeout(finishTimerRef.current)
      finishTimerRef.current = null
    }
    if (hideTimerRef.current !== null) {
      window.clearTimeout(hideTimerRef.current)
      hideTimerRef.current = null
    }
  }

  const startBar = () => {
    clearTimers()
    startedAtRef.current = Date.now()
    setFading(false)
    setVisible(true)
    setProgress(18)

    window.requestAnimationFrame(() => {
      setProgress((prev) => (prev < 64 ? 64 : prev))
    })

    trickleTimerRef.current = window.setInterval(() => {
      setProgress((prev) => {
        if (prev >= 90) {
          return prev
        }
        return Math.min(90, prev + Math.max(1.5, (90 - prev) * 0.16))
      })
    }, 120)
  }

  const completeBar = () => {
    const elapsed = Date.now() - startedAtRef.current
    const waitMs = Math.max(0, MIN_VISIBLE_MS - elapsed)

    if (finishTimerRef.current !== null) {
      window.clearTimeout(finishTimerRef.current)
    }
    finishTimerRef.current = window.setTimeout(() => {
      if (pendingRef.current > 0) {
        return
      }
      clearTimers()
      setProgress(100)
      hideTimerRef.current = window.setTimeout(() => {
        setFading(true)
        hideTimerRef.current = window.setTimeout(() => {
          setVisible(false)
          setFading(false)
          setProgress(0)
        }, FADE_OUT_MS)
      }, 140)
    }, waitMs)
  }

  // 路由切换时启动过渡进度条
  useEffect(() => {
    if (!mountedRef.current) {
      mountedRef.current = true
      return
    }
    startBar()
    if (pendingRef.current === 0) {
      completeBar()
    }
    return () => {
      clearTimers()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname, location.search])

  // 联动接口在途请求，确保请求结束后进度条才滑至 100%
  useEffect(() => {
    return subscribePendingRequests((count) => {
      const prev = pendingRef.current
      pendingRef.current = count
      if (count > 0 && prev === 0 && mountedRef.current) {
        startBar()
      } else if (count === 0 && prev > 0) {
        completeBar()
      }
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  if (!visible) {
    return null
  }

  return (
    <div
      className={`route-progress-bar ${fading ? 'route-progress-bar--fading' : ''}`}
      role="progressbar"
      aria-label="页面加载进度"
      aria-valuenow={Math.round(progress)}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div
        className="route-progress-bar__fill"
        style={{ width: `${progress}%` }}
      />
    </div>
  )
}
