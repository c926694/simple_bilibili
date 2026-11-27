import { useState, type FormEvent } from 'react'
import type {
  DanmakuFontSize,
  DanmakuMode,
  DanmakuStyleConfig,
} from '../../types/danmaku'
import './DanmakuBar.css'

const COLOR_PRESETS = [
  '#FFFFFF',
  '#FE0302',
  '#FF7204',
  '#FFAA02',
  '#FFD302',
  '#00CD00',
  '#00AEEC',
  '#FB7299',
]

const FONT_SIZE_OPTIONS: { value: DanmakuFontSize; label: string }[] = [
  { value: 18, label: '小' },
  { value: 25, label: '标准' },
  { value: 36, label: '大' },
]

const MODE_OPTIONS: { value: DanmakuMode; label: string }[] = [
  { value: 1, label: '滚动' },
  { value: 2, label: '顶部' },
  { value: 3, label: '底部' },
]

interface DanmakuBarProps {
  onlineCount: number
  loadedCount: number
  enabled: boolean
  onToggleEnabled: () => void
  styleConfig: DanmakuStyleConfig
  onChangeStyle: (next: DanmakuStyleConfig) => void
  isLoggedIn: boolean
  disabled?: boolean
  sending?: boolean
  errorMsg?: string
  onRequireLogin: () => void
  onSend: (content: string) => Promise<boolean>
}

export function DanmakuBar({
  onlineCount,
  loadedCount,
  enabled,
  onToggleEnabled,
  styleConfig,
  onChangeStyle,
  isLoggedIn,
  disabled = false,
  sending = false,
  errorMsg = '',
  onRequireLogin,
  onSend,
}: DanmakuBarProps) {
  const [content, setContent] = useState('')
  const [panelOpen, setPanelOpen] = useState(false)

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    if (!isLoggedIn) {
      onRequireLogin()
      return
    }
    const trimmed = content.trim()
    if (!trimmed || sending || disabled) {
      return
    }
    const ok = await onSend(trimmed)
    if (ok) {
      setContent('')
    }
  }

  return (
    <div className="danmaku-bar">
      <div className="danmaku-bar__stats">
        <span className="danmaku-bar__online" data-testid="online-count">
          <strong>{Math.max(1, onlineCount)}</strong> 人正在看
        </span>
        <span className="danmaku-bar__sep">·</span>
        <span>已装填 {loadedCount} 条弹幕</span>
      </div>

      <div className="danmaku-bar__controls">
        <button
          type="button"
          className={`danmaku-switch-btn ${enabled ? 'danmaku-switch-btn--on' : ''}`}
          onClick={onToggleEnabled}
          aria-pressed={enabled}
          aria-label={enabled ? '关闭弹幕' : '开启弹幕'}
        >
          弹幕 {enabled ? '开' : '关'}
        </button>

        <div className="danmaku-style-anchor">
          <button
            type="button"
            className="danmaku-style-btn"
            onClick={() => setPanelOpen((prev) => !prev)}
            aria-label="弹幕样式设置"
          >
            样式
          </button>

          {panelOpen && (
            <div className="danmaku-style-popover" role="dialog" aria-label="弹幕样式面板">
              <div className="danmaku-style-group">
                <span className="danmaku-style-label">字号</span>
                <div className="danmaku-style-options">
                  {FONT_SIZE_OPTIONS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`danmaku-chip ${styleConfig.fontSize === opt.value ? 'danmaku-chip--active' : ''}`}
                      onClick={() => onChangeStyle({ ...styleConfig, fontSize: opt.value })}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>

              <div className="danmaku-style-group">
                <span className="danmaku-style-label">模式</span>
                <div className="danmaku-style-options">
                  {MODE_OPTIONS.map((opt) => (
                    <button
                      key={opt.value}
                      type="button"
                      className={`danmaku-chip ${styleConfig.mode === opt.value ? 'danmaku-chip--active' : ''}`}
                      onClick={() => onChangeStyle({ ...styleConfig, mode: opt.value })}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>

              <div className="danmaku-style-group">
                <span className="danmaku-style-label">颜色</span>
                <div className="danmaku-color-grid">
                  {COLOR_PRESETS.map((hex) => (
                    <button
                      key={hex}
                      type="button"
                      className={`danmaku-color-swatch ${styleConfig.color.toUpperCase() === hex ? 'danmaku-color-swatch--active' : ''}`}
                      style={{ backgroundColor: hex }}
                      aria-label={`颜色 ${hex}`}
                      onClick={() => onChangeStyle({ ...styleConfig, color: hex })}
                    />
                  ))}
                </div>
              </div>

              <div className="danmaku-style-group">
                <span className="danmaku-style-label">
                  不透明度 {Math.round(styleConfig.opacity * 100)}%
                </span>
                <input
                  type="range"
                  min={0.2}
                  max={1}
                  step={0.1}
                  value={styleConfig.opacity}
                  onChange={(e) =>
                    onChangeStyle({ ...styleConfig, opacity: Number(e.target.value) })
                  }
                  aria-label="弹幕不透明度"
                />
              </div>
            </div>
          )}
        </div>

        <form className="danmaku-send-form" onSubmit={handleSubmit}>
          <input
            type="text"
            className="danmaku-send-form__input"
            value={content}
            maxLength={100}
            disabled={disabled || sending}
            placeholder={isLoggedIn ? '发个友善的弹幕见证当下' : '请先登录后发送弹幕'}
            aria-label="输入弹幕内容"
            onFocus={() => {
              if (!isLoggedIn) {
                onRequireLogin()
              }
            }}
            onChange={(e) => setContent(e.target.value)}
          />
          <button
            type="submit"
            className="danmaku-send-form__btn"
            disabled={disabled || sending}
          >
            {sending ? '发送中' : '发送'}
          </button>
        </form>
      </div>

      {errorMsg && (
        <div className="danmaku-bar__error" role="alert">
          {errorMsg}
        </div>
      )}
    </div>
  )
}
