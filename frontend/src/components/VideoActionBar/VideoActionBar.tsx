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

function ChargeRing() {
  return (
    <svg className="bili-triple-ring" viewBox="0 0 44 44" aria-hidden="true">
      <circle className="bili-triple-ring__bg" cx="22" cy="22" r="19" />
      <circle className="bili-triple-ring__fg" cx="22" cy="22" r="19" />
    </svg>
  )
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
  const [tripleBurst, setTripleBurst] = useState(false)
  const [toast, setToast] = useState<{ type: 'info' | 'warn' | 'error'; text: string } | null>(null)

  const [coinModalOpen, setCoinModalOpen] = useState(false)
  const [selectedCoinCount, setSelectedCoinCount] = useState<1 | 2>(1)

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
  const burstTimerRef = useRef<number | null>(null)

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
      if (burstTimerRef.current !== null) {
        window.clearTimeout(burstTimerRef.current)
      }
    }
  }, [])

  const playTripleBurst = () => {
    setTripleBurst(true)
    if (burstTimerRef.current !== null) {
      window.clearTimeout(burstTimerRef.current)
    }
    burstTimerRef.current = window.setTimeout(() => {
      setTripleBurst(false)
    }, 750)
  }

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
      playTripleBurst()
      if (res.coin_skipped_reason === 'insufficient_coins') {
        showToast('warn', '硬币余额不足，已保持点赞和收藏状态，本次未投币')
      } else if (res.coin_skipped_reason === 'self_video') {
        showToast('info', '已保持点赞和收藏状态（不能给自己的视频投币）')
      } else {
        showToast('info', '一键三连成功！感谢你的支持')
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
      showToast('info', '你已经为这个视频投满 2 枚硬币啦')
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
      showToast('warn', `硬币余额不足（当前仅剩 ${interaction.coin_balance} 枚硬币），每日登录自动签到可领取 1 枚硬币`)
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
      showToast('info', `成功投出 ${res.added_coins} 枚硬币`)
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
      showToast('info', res.favorited ? '收藏成功' : '已取消收藏')
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
          className={`bili-action-item ${interaction.liked ? 'bili-action-item--active' : ''} ${holdingTriple ? 'bili-action-item--charging' : ''} ${tripleBurst ? 'bili-action-item--burst' : ''}`}
          onPointerDown={handleLikePointerDown}
          onPointerUp={cancelTripleHold}
          onPointerLeave={cancelTripleHold}
          onPointerCancel={cancelTripleHold}
          onClick={handleLikeClick}
          disabled={busy}
          title="单击点赞，长按 1.5 秒一键三连"
          aria-label="点赞或长按一键三连"
        >
          <span className="bili-action-item__icon-box">
            {holdingTriple && <ChargeRing />}
            <svg width="28" height="28" viewBox="0 0 28 28" fill="currentColor" aria-hidden="true">
              <path d="M9.6 11.4L13.2 4.3c.5-1 1.7-1.4 2.7-.9.9.4 1.4 1.4 1.2 2.4l-.8 4.4h5.3c1.6 0 2.8 1.5 2.4 3.1l-1.9 7.8c-.3 1.3-1.5 2.2-2.8 2.2H10.5c-.8 0-1.5-.7-1.5-1.5v-9.4c0-.4.2-.8.6-1zM4 12.2c0-.9.7-1.6 1.6-1.6h1.2c.4 0 .8.4.8.8v11.1c0 .4-.4.8-.8.8H5.6c-.9 0-1.6-.7-1.6-1.6v-9.5z" />
            </svg>
          </span>
          <span className="bili-action-item__num">{formatCount(stat.like_count)}</span>
        </button>

        <button
          type="button"
          className={`bili-action-item ${interaction.coin_count > 0 ? 'bili-action-item--active' : ''} ${holdingTriple ? 'bili-action-item--charging' : ''} ${tripleBurst ? 'bili-action-item--burst' : ''}`}
          onClick={openCoinModal}
          disabled={busy}
          title="投币支持 UP 主"
          aria-label="投币"
        >
          <span className="bili-action-item__icon-box">
            {holdingTriple && <ChargeRing />}
            <svg width="28" height="28" viewBox="0 0 28 28" fill="none" aria-hidden="true">
              <circle cx="14" cy="14" r="11" stroke="currentColor" strokeWidth="2.2" />
              <path
                d="M10.2 9.4h7.6M14 9.4v9.6M9.6 13.6h8.8v4.2c0 .7-.5 1.2-1.2 1.2h-.8M9.6 13.6v5.2"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </span>
          <span className="bili-action-item__num">{formatCount(stat.coin_count)}</span>
        </button>

        <button
          type="button"
          className={`bili-action-item ${interaction.favorited ? 'bili-action-item--active' : ''} ${holdingTriple ? 'bili-action-item--charging' : ''} ${tripleBurst ? 'bili-action-item--burst' : ''}`}
          onClick={() => void openFavoriteModal()}
          disabled={busy}
          title="收藏到收藏夹"
          aria-label="收藏"
        >
          <span className="bili-action-item__icon-box">
            {holdingTriple && <ChargeRing />}
            <svg width="28" height="28" viewBox="0 0 28 28" fill="currentColor" aria-hidden="true">
              <path d="M14 3.4l3.1 6.3 6.9 1-5 4.9 1.2 6.9L14 19.2l-6.2 3.3 1.2-6.9-5-4.9 6.9-1L14 3.4z" />
            </svg>
          </span>
          <span className="bili-action-item__num">{formatCount(stat.favorite_count)}</span>
        </button>

        <button
          type="button"
          className="bili-action-item"
          onClick={() => void handleShare()}
          title="复制链接分享"
          aria-label="转发"
        >
          <span className="bili-action-item__icon-box">
            <svg width="28" height="28" viewBox="0 0 28 28" fill="currentColor" aria-hidden="true">
              <path d="M16.4 6.1l7.2 6.5c.6.5.6 1.4 0 1.9l-7.2 6.5c-.8.7-2.1.2-2.1-.9v-3.5c-4.5.1-7.5 1.5-9.8 4.4-.5.6-1.5.2-1.4-.6.9-5.9 4.5-9.8 11.2-10.3V7c0-1.1 1.3-1.6 2.1-.9z" />
            </svg>
          </span>
          <span className="bili-action-item__num">{formatCount(stat.share_count)}</span>
        </button>

        <span className="bili-action-bar__tip">长按点赞可触发一键三连</span>
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
            className="bili-coin-modal"
            role="dialog"
            aria-label="给 UP 主投币"
            onClick={(e) => e.stopPropagation()}
          >
            <button
              type="button"
              className="bili-modal-close"
              onClick={() => setCoinModalOpen(false)}
              aria-label="关闭弹窗"
            >
              ×
            </button>
            <div className="bili-coin-modal__title">
              给 UP 主投上 <strong>{selectedCoinCount}</strong> 枚硬币
            </div>

            <div className="bili-coin-cards">
              <button
                type="button"
                className={`bili-coin-card ${selectedCoinCount === 1 ? 'bili-coin-card--selected' : ''}`}
                onClick={() => setSelectedCoinCount(1)}
              >
                <span className="bili-coin-card__badge">1 硬币</span>
                <div className="bili-coin-card__art">
                  <span className="bili-coin-disc">币</span>
                </div>
                <span className="bili-coin-card__label">给 UP 主投 1 枚硬币</span>
              </button>

              <button
                type="button"
                className={`bili-coin-card ${selectedCoinCount === 2 ? 'bili-coin-card--selected' : ''}`}
                disabled={remainCoinCap < 2}
                onClick={() => setSelectedCoinCount(2)}
              >
                <span className="bili-coin-card__badge">2 硬币</span>
                <div className="bili-coin-card__art">
                  <span className="bili-coin-disc">币</span>
                  <span className="bili-coin-disc bili-coin-disc--second">币</span>
                </div>
                <span className="bili-coin-card__label">给 UP 主投 2 枚硬币</span>
              </button>
            </div>

            <div className="bili-coin-modal__balance">
              我的硬币余额：<strong>{interaction.coin_balance}</strong> 枚（本视频还可投{' '}
              {remainCoinCap} 枚）
            </div>

            <div className="bili-coin-modal__foot">
              <button
                type="button"
                className="bili-modal-confirm-btn"
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
            className="bili-fav-modal"
            role="dialog"
            aria-label="添加到收藏夹"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="bili-fav-modal__head">
              <h3>添加到收藏夹</h3>
              <button type="button" onClick={() => setFavModalOpen(false)} aria-label="关闭弹窗">
                ×
              </button>
            </div>

            <div className="bili-fav-modal__body">
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
                                setCheckedFolderIds((prev) =>
                                  prev.filter((id) => id !== folder.id),
                                )
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
                  placeholder="+ 新建收藏夹"
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
                <button
                  type="submit"
                  className="interaction-btn-secondary"
                  disabled={creatingFolder}
                >
                  新建
                </button>
              </form>
            </div>

            <div className="bili-fav-modal__foot">
              <button
                type="button"
                className="bili-modal-confirm-btn"
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
