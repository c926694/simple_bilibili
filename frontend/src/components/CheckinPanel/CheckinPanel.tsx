import { useCallback, useEffect, useState } from 'react'
import { ApiError } from '../../api/client'
import { interactionApi } from '../../api/interaction'
import { useAuth } from '../../context/AuthContext'
import type { CheckinStatus } from '../../types/interaction'
import './CheckinPanel.css'

function daysInMonth(monthStr: string): number {
  const [y, m] = monthStr.split('-').map(Number)
  if (!y || !m) {
    return 30
  }
  return new Date(y, m, 0).getDate()
}

export function CheckinPanel() {
  const { user, setUser } = useAuth()
  const [status, setStatus] = useState<CheckinStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')

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

  const handleCheckin = async () => {
    if (!user || busy) {
      return
    }
    setBusy(true)
    setNotice('')
    try {
      const res = await interactionApi.checkin()
      setStatus(res)
      setUser({ ...user, coin_balance: res.coin_balance })
      setNotice(`签到成功，获得 +${res.reward_coins} 枚代币！`)
    } catch (err) {
      setNotice(err instanceof ApiError ? err.message : '签到失败，请稍后重试')
    } finally {
      setBusy(false)
    }
  }

  if (!user) {
    return null
  }

  const totalDays = status?.month_days || daysInMonth(status?.month ?? '')
  const checkedDates = status?.checked_dates ?? []
  const checkedSet = new Set(checkedDates)

  return (
    <section className="checkin-card" aria-label="每日签到">
      <div className="checkin-card__head">
        <div>
          <h2 className="checkin-card__title">每日签到领代币</h2>
          <p className="checkin-card__sub">
            {status
              ? `${status.month} 已累计签到 ${checkedDates.length} / ${totalDays} 天 · 连续签到 ${status.streak_days} 天 · 当前代币 ${status.coin_balance} 枚`
              : '每日签到可领取 1 枚代币'}
          </p>
        </div>

        <button
          type="button"
          className={`checkin-card__btn ${status?.checked_in_today ? 'checkin-card__btn--done' : ''}`}
          disabled={loading || busy || Boolean(status?.checked_in_today)}
          onClick={() => void handleCheckin()}
        >
          {status?.checked_in_today ? '今日已签到' : busy ? '签到中...' : '立即签到 +1 代币'}
        </button>
      </div>

      {notice && (
        <div className="checkin-card__notice" role="status">
          {notice}
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
