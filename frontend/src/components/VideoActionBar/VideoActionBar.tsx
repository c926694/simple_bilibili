import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ApiError } from '../../api/client'
import { interactionApi } from '../../api/interaction'
import type { FavoriteFolder, VideoInteractionState } from '../../types/interaction'
import type { VideoStat } from '../../types/video'
import { formatCount } from '../../utils/format'
import './VideoActionBar.css'

const TRIPLE_HOLD_MS = 1500

interface VideoActionBarProps {
  bvid: string
  stat: VideoStat
  interaction: VideoInteractionState
  isAuthor: boolean
  isLoggedIn: boolean
  onRequireLogin: () => void
  onInteractionChange: (next: VideoInteractionState) => void
  onStatDelta: (delta: Partial<VideoStat>) => void
  onCoinBalanceChange: (balance: number) => void
}

export function VideoActionBar({
  bvid,
  stat,
  interaction,
  isAuthor,
  isLoggedIn,
  onRequireLogin,
  onInteractionChange,
  onStatDelta,
  onCoinBalanceChange,
}: VideoActionBarProps) {
  const [busy, setBusy] = useState(false)
  const [holdingTriple, setHoldingTriple] = useState(false)
  const [toast, setToast] = useState<{ type: 'info' | 'warn' | 'error'; text: string } | null>(null)

  // 投币弹窗状态
  const [coinModalOpen, setCoinModalOpen] = useState(false)
  const [selectedCoinCount, setSelectedCoinCount] = useState<1 | 2>(1)

  // 收藏夹弹窗状态
  const [favModalOpen, setFavModalOpen] = useState(false)
  const [folders, setFolders] = useState<FavoriteFolder[]>([])
  const [foldersLoading, setFoldersLoading] = useState(false)
  const [checkedFolderIds, setCheckedFolderIds] = useState<string[]>([])
  const [newFolderTitle, setNewFolderTitle] = useState('')
  const [newFolderPublic, setNewFolderPublic] = useState(true)
  const [creatingFolder, setCreatingFolder] = useState(false)

  const holdTimerRef = useRef<number | null>(null)
  const tripleTriggeredRef = useRef(false)
  const toastTimerRef = useRef<number | null>(null)

  const showToast = (type: 'info' | 'warn' | 'error', text: string) => {
    setToast({ type, text })
    if (toastTimerRef.current !== null) {
      window.clearTimeout(toastTimerRef.current)
    }
    toastTimerRef.current = window.setTimeout(() => {
      setToast(null)
    }, 3600)
  }

  useEffect(() => {
    return () => {
      if (holdTimerRef.current !== null) {
        window.clearTimeout(holdTimerRef.current)
      }
      if (toastTimerRef.current !== null) {
        window.clearTimeout(toastTimerRef.current)
      }
    }
  }, [])

  const handleToggleLike = async () => {
    if (!isLoggedIn) {
      onRequireLogin()
      return
    }
    if (busy) {
      return
    }
    setBusy(true)
    try {
      const res = interaction.liked
        ? await interactionApi.unlikeVideo(bvid)
        : await interactionApi.likeVideo(bvid)
      onInteractionChange({ ...interaction, liked: res.liked })
      if (res.delta !== 0) {
        onStatDelta({ like_count: res.delta })
      }
    } catch (err) {
      showToast('error', err instanceof ApiError ? err.message : '操作失败，请稍后重试')
    } finally {
      setBusy(false)
    }
  }

  const triggerTriple = async () => {
    if (!isLoggedIn) {
      onRequireLogin()
      return
    }
    if (busy) {
      return
    }
    setBusy(true)
    try {
      const res = await interactionApi.tripleVideo(bvid)
      onInteractionChange({
        ...interaction,
        liked: res.liked,
        coin_count: res.coin_count,
        favorited: res.favorited,
        favorite_folder_ids: res.favorite_folder_ids,
        coin_balance: res.coin_balance,
      })
      onCoinBalanceChange(res.coin_balance)
      onStatDelta({
        like_count: res.like_delta,
        coin_count: res.added_coins,
        favorite_count: res.favorite_delta,
      })
      if (res.coin_skipped_reason === 'insufficient_coins') {
        showToast('warn', '代币余额不足，已保持点赞和收藏状态，本次未投币')
      } else if (res.coin_skipped_reason === 'self_video') {
        showToast('info', '已保持点赞和收藏状态（不能给自己的视频投币）')
      } else {
        showToast('info', '一键三连成功！感谢支持')
      }
    } catch (err) {
      showToast('error', err instanceof ApiError ? err.message : '一键三连失败')
    } finally {
      setBusy(false)
    }
  }

  const handleLikePointerDown = () => {
    if (!isLoggedIn || busy) {
      return
    }
    tripleTriggeredRef.current = false
    setHoldingTriple(true)
    holdTimerRef.current = window.setTimeout(() => {
      tripleTriggeredRef.current = true
      setHoldingTriple(false)
      void triggerTriple()
    }, TRIPLE_HOLD_MS)
  }

  const cancelTripleHold = () => {
    setHoldingTriple(false)
    if (holdTimerRef.current !== null) {
      window.clearTimeout(holdTimerRef.current)
      holdTimerRef.current = null
    }
  }

  const handleLikeClick = () => {
    if (tripleTriggeredRef.current) {
      tripleTriggeredRef.current = false
      return
    }
    void handleToggleLike()
  }

  const openCoinModal = () => {
    if (!isLoggedIn) {
      onRequireLogin()
      return
    }
    if (isAuthor) {
      showToast('warn', '不能给自己的视频投币')
      return
    }
    if (interaction.coin_count >= 2) {
      showToast('info', '你已经为这个视频投满 2 枚代币啦')
      return
    }
    setSelectedCoinCount(1)
    setCoinModalOpen(true)
  }

  const handleConfirmCoin = async () => {
    if (busy) {
      return
    }
    if (interaction.coin_balance < selectedCoinCount) {
      showToast('warn', `代币余额不足（当前仅剩 ${interaction.coin_balance} 枚），可前往每日签到获取代币`)
      return
    }
    setBusy(true)
    try {
      const res = await interactionApi.coinVideo(bvid, selectedCoinCount)
      onInteractionChange({
        ...interaction,
        coin_count: res.coin_count,
        coin_balance: res.coin_balance,
      })
      onCoinBalanceChange(res.coin_balance)
      if (res.added_coins > 0) {
        onStatDelta({ coin_count: res.added_coins })
      }
      setCoinModalOpen(false)
      showToast('info', `成功投出 ${res.added_coins} 枚代币`)
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : '投币失败'
      showToast(err instanceof ApiError && err.code === 40001 ? 'warn' : 'error', msg)
    } finally {
      setBusy(false)
    }
  }

  const openFavoriteModal = async () => {
    if (!isLoggedIn) {
      onRequireLogin()
      return
    }
    setFavModalOpen(true)
    setCheckedFolderIds(interaction.favorite_folder_ids)
    setFoldersLoading(true)
    try {
      const list = await interactionApi.listFavoriteFolders()
      setFolders(list)
    } catch (err) {
      showToast('error', err instanceof ApiError ? err.message : '加载收藏夹失败')
    } finally {
      setFoldersLoading(false)
    }
  }

  const handleCreateFolder = async (event: FormEvent) => {
    event.preventDefault()
    const trimmed = newFolderTitle.trim()
    if (!trimmed || creatingFolder) {
      return
    }
    setCreatingFolder(true)
    try {
      const created = await interactionApi.createFavoriteFolder(trimmed, newFolderPublic)
      setFolders((prev) => [...prev, created])
      setCheckedFolderIds((prev) => (prev.includes(created.id) ? prev : [...prev, created.id]))
      setNewFolderTitle('')
    } catch (err) {
      showToast('error', err instanceof ApiError ? err.message : '创建收藏夹失败')
    } finally {
      setCreatingFolder(false)
    }
  }

  const handleConfirmFavorites = async () => {
    if (busy) {
      return
    }
    setBusy(true)
    try {
      const res = await interactionApi.updateVideoFavorites(bvid, checkedFolderIds)
      onInteractionChange({
        ...interaction,
        favorited: res.favorited,
        favorite_folder_ids: res.favorite_folder_ids,
      })
      if (res.delta !== 0) {
        onStatDelta({ favorite_count: res.delta })
      }
      setFavModalOpen(false)
      showToast('info', res.favorited ? '收藏夹更新成功' : '已取消收藏')
    } catch (err) {
      showToast('error', err instanceof ApiError ? err.message : '更新收藏失败')
    } finally {
      setBusy(false)
    }
  }

  const handleShare = async () => {
    const url = window.location.href
    try {
      await navigator.clipboard?.writeText(url)
    } catch {
      // 忽略剪贴板权限限制
    }
    try {
      const res = await interactionApi.shareVideo(bvid)
      if (res.counted) {
        onStatDelta({ share_count: 1 })
      }
      showToast('info', '视频链接已复制，快分享给好友吧')
    } catch {
      showToast('info', '视频链接已复制')
    }
  }

  const remainCoinCap = Math.max(0, 2 - interaction.coin_count)

  return (
    <div className="video-action-bar">
      <div className="video-action-bar__group">
        <button
          type="button"
          className={`action-btn ${interaction.liked ? 'action-btn--active' : ''} ${holdingTriple ? 'action-btn--holding' : ''}`}
          onPointerDown={handleLikePointerDown}
          onPointerUp={cancelTripleHold}
          onPointerLeave={cancelTripleHold}
          onPointerCancel={cancelTripleHold}
          onClick={handleLikeClick}
          disabled={busy}
          title="单击点赞，长按 1.5 秒一键三连"
          aria-label="点赞或长按一键三连"
        >
          <span className="action-btn__icon">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M2 10h4v10H2V10zm7 10h8.2c.9 0 1.7-.6 1.9-1.5l1.6-6.2c.3-1.2-.6-2.3-1.8-2.3H14l.8-3.8.1-.5c0-.6-.2-1.1-.6-1.5L13 3l-5.4 5.4c-.4.4-.6.9-.6 1.4V18c0 1.1.9 2 2 2z" />
            </svg>
          </span>
          <span className="action-btn__count">{formatCount(stat.like_count)}</span>
          {holdingTriple && <span className="action-btn__ring" />}
        </button>

        <button
          type="button"
          className={`action-btn ${interaction.coin_count > 0 ? 'action-btn--active' : ''} ${holdingTriple ? 'action-btn--triple-sync' : ''}`}
          onClick={openCoinModal}
          disabled={busy}
          aria-label="投币"
        >
          <span className="action-btn__icon">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="2" />
              <path d="M9 9h6M12 9v7M9.5 13h5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
            </svg>
          </span>
          <span className="action-btn__count">{formatCount(stat.coin_count)}</span>
        </button>

        <button
          type="button"
          className={`action-btn ${interaction.favorited ? 'action-btn--active' : ''} ${holdingTriple ? 'action-btn--triple-sync' : ''}`}
          onClick={() => void openFavoriteModal()}
          disabled={busy}
          aria-label="收藏"
        >
          <span className="action-btn__icon">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M12 2.5l2.9 6 6.6.9-4.8 4.6 1.2 6.5L12 17.4 6.1 20.5l1.2-6.5-4.8-4.6 6.6-.9L12 2.5z" />
            </svg>
          </span>
          <span className="action-btn__count">{formatCount(stat.favorite_count)}</span>
        </button>

        <button
          type="button"
          className="action-btn"
          onClick={() => void handleShare()}
          aria-label="转发"
        >
          <span className="action-btn__icon">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M15 5l5 5-5 5M20 10H10a6 6 0 00-6 6v3"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </span>
          <span className="action-btn__count">{formatCount(stat.share_count)}</span>
        </button>

        <span className="action-bar__hint">长按点赞可触发一键三连</span>
      </div>

      {toast && (
        <div
          className={`action-toast action-toast--${toast.type}`}
          role="alert"
          data-testid="interaction-toast"
        >
          {toast.text}
        </div>
      )}

      {coinModalOpen && (
        <div className="interaction-modal-mask" onClick={() => setCoinModalOpen(false)}>
          <div
            className="interaction-modal"
            role="dialog"
            aria-label="给 UP 主投币"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="interaction-modal__head">
              <h3>给 UP 主投上枚代币</h3>
              <button type="button" onClick={() => setCoinModalOpen(false)} aria-label="关闭弹窗">
                ×
              </button>
            </div>
            <p className="interaction-modal__sub">
              当前可用代币：<strong>{interaction.coin_balance}</strong> 枚（本视频还可投{' '}
              {remainCoinCap} 枚）
            </p>
            <div className="coin-choice-row">
              <button
                type="button"
                className={`coin-choice-card ${selectedCoinCount === 1 ? 'coin-choice-card--active' : ''}`}
                onClick={() => setSelectedCoinCount(1)}
              >
                <span className="coin-choice-card__num">1 枚</span>
                <span className="coin-choice-card__desc">投 1 枚代币支持</span>
              </button>
              <button
                type="button"
                className={`coin-choice-card ${selectedCoinCount === 2 ? 'coin-choice-card--active' : ''}`}
                disabled={remainCoinCap < 2}
                onClick={() => setSelectedCoinCount(2)}
              >
                <span className="coin-choice-card__num">2 枚</span>
                <span className="coin-choice-card__desc">投 2 枚代币大力支持</span>
              </button>
            </div>
            <div className="interaction-modal__foot">
              <button
                type="button"
                className="interaction-btn-secondary"
                onClick={() => setCoinModalOpen(false)}
              >
                取消
              </button>
              <button
                type="button"
                className="interaction-btn-primary"
                disabled={busy}
                onClick={() => void handleConfirmCoin()}
              >
                确认投币
              </button>
            </div>
          </div>
        </div>
      )}

      {favModalOpen && (
        <div className="interaction-modal-mask" onClick={() => setFavModalOpen(false)}>
          <div
            className="interaction-modal"
            role="dialog"
            aria-label="添加到收藏夹"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="interaction-modal__head">
              <h3>添加到收藏夹</h3>
              <button type="button" onClick={() => setFavModalOpen(false)} aria-label="关闭弹窗">
                ×
              </button>
            </div>
            {foldersLoading ? (
              <div className="interaction-modal__empty">正在加载收藏夹...</div>
            ) : (
              <ul className="fav-folder-check-list">
                {folders.map((folder) => {
                  const checked = checkedFolderIds.includes(folder.id)
                  return (
                    <li key={folder.id}>
                      <label className="fav-folder-check-row">
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setCheckedFolderIds((prev) => [...prev, folder.id])
                            } else {
                              setCheckedFolderIds((prev) => prev.filter((id) => id !== folder.id))
                            }
                          }}
                        />
                        <span className="fav-folder-check-row__title">
                          {folder.title}
                          {folder.is_default && <em>默认</em>}
                        </span>
                        <span className="fav-folder-check-row__meta">
                          {folder.is_public ? '公开' : '私密'} · {folder.item_count}
                        </span>
                      </label>
                    </li>
                  )
                })}
              </ul>
            )}

            <form className="fav-new-form" onSubmit={handleCreateFolder}>
              <input
                type="text"
                value={newFolderTitle}
                maxLength={64}
                onChange={(e) => setNewFolderTitle(e.target.value)}
                placeholder="新建收藏夹名称"
                aria-label="新建收藏夹名称"
              />
              <label className="fav-new-form__pub">
                <input
                  type="checkbox"
                  checked={newFolderPublic}
                  onChange={(e) => setNewFolderPublic(e.target.checked)}
                />
                公开
              </label>
              <button type="submit" className="interaction-btn-secondary" disabled={creatingFolder}>
                新建
              </button>
            </form>

            <div className="interaction-modal__foot">
              <button
                type="button"
                className="interaction-btn-secondary"
                onClick={() => setFavModalOpen(false)}
              >
                取消
              </button>
              <button
                type="button"
                className="interaction-btn-primary"
                disabled={busy}
                onClick={() => void handleConfirmFavorites()}
              >
                确定
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
