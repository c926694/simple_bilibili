import { useCallback, useEffect, useState } from 'react'
import { interactionApi } from '../../api/interaction'
import { useAuth } from '../../context/AuthContext'
import type { CheckinStatus } from '../../types/interaction'
import './CheckinPanel.css'

export function CheckinPanel() {
  const { user, setUser } = useAuth()
  const [status, setStatus] = useState<CheckinStatus | null>(null)
  const [loading, setLoading] = useState(true)

  const loadStatus = useCallback(async () => {
    if (!user) {
      return
    }
    setLoading(true)
    try {
      const data = await interactionApi.getCheckinStatus()
      setStatus(data)
      if (data.coin_balance !== user.coin_balance) {
        setUser({ ...user, coin_balance: data.coin_balance })
      }
    } catch {
      setStatus(null)
    } finally {
      setLoading(false)
    }
  }, [user?.id])

  useEffect(() => {
    void loadStatus()
  }, [loadStatus])

  if (!user) {
    return null
  }

  const totalDays = status?.month_days || 30
  const checkedDates = status?.checked_dates ?? []
  const checkedSet = new Set(checkedDates)

  return (
    <section className="checkin-card" aria-label="每日硬币签到">
      <div className="checkin-card__head">
        <div>
          <h2 className="checkin-card__title">每日登录自动签到</h2>
          <p className="checkin-card__sub">
            {status
              ? `${status.month} 已累计签到 ${checkedDates.length} / ${totalDays} 天 · 连续签到 ${status.streak_days} 天 · 当前硬币 ${status.coin_balance} 枚`
              : '每日首次登录自动获得 1 枚硬币'}
          </p>
        </div>

        <span className="checkin-card__badge">
          {status?.checked_in_today ? '今日已自动签到 +1 硬币' : '每日登录自动签到'}
        </span>
      </div>

      {status?.just_checked_in && (
        <div className="checkin-card__notice" role="status">
          欢迎回来！今日已自动完成签到，硬币 +1
        </div>
      )}

      {!loading && status && (
        <div className="checkin-calendar" aria-label="本月签到日历">
          {Array.from({ length: totalDays }, (_, idx) => {
            const day = idx + 1
            const checked = checkedSet.has(day)
            return (
              <span
                key={day}
                className={`checkin-day ${checked ? 'checkin-day--checked' : ''}`}
                title={checked ? `${status.month}-${String(day).padStart(2, '0')} 已签到` : `${day} 日`}
              >
                {day}
              </span>
            )
          })}
        </div>
      )}
    </section>
  )
}
