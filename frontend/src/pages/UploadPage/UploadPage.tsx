import { useRef, useState, type ChangeEvent, type DragEvent } from 'react'
import { ApiError } from '../../api/client'
import { uploadApi } from '../../api/upload'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { useAuth } from '../../context/AuthContext'
import { VideoMultipartUploader, type VideoUploadProgress } from '../../utils/videoUploader'
import './UploadPage.css'

const ALLOWED_VIDEO_EXTS = ['.mp4', '.webm', '.mov', '.mkv']
const ALLOWED_COVER_TYPES = ['image/jpeg', 'image/png', 'image/webp', 'image/gif']
const MAX_COVER_BYTES = 5 * 1024 * 1024

function formatBytes(bytes: number): string {
  if (bytes <= 0) {
    return '0 B'
  }
  const units = ['B', 'KB', 'MB', 'GB']
  let idx = 0
  let value = bytes
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(idx === 0 ? 0 : 2)} ${units[idx]}`
}

export function UploadPage() {
  const { user, initializing, openAuthModal } = useAuth()
  const videoInputRef = useRef<HTMLInputElement | null>(null)
  const coverInputRef = useRef<HTMLInputElement | null>(null)
  const uploaderRef = useRef<VideoMultipartUploader | null>(null)

  const [dragging, setDragging] = useState(false)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [progress, setProgress] = useState<VideoUploadProgress | null>(null)
  const [videoError, setVideoError] = useState('')

  const [coverUrl, setCoverUrl] = useState('')
  const [coverUploading, setCoverUploading] = useState(false)
  const [coverNotice, setCoverNotice] = useState<{ type: 'success' | 'error'; text: string } | null>(
    null,
  )

  const startVideoUpload = async (file: File) => {
    const lowerName = file.name.toLowerCase()
    const validExt = ALLOWED_VIDEO_EXTS.some((ext) => lowerName.endsWith(ext))
    if (!validExt) {
      setVideoError('仅支持 MP4、WebM、MOV、MKV 格式视频文件')
      return
    }
    if (file.size <= 0) {
      setVideoError('视频文件为空，请重新选择')
      return
    }

    setVideoError('')
    setSelectedFile(file)
    const uploader = new VideoMultipartUploader(file, (next) => {
      setProgress(next)
    })
    uploaderRef.current = uploader

    try {
      await uploader.start()
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : '视频上传失败，请稍后重试'
      setVideoError(msg)
      setProgress((prev) => (prev ? { ...prev, phase: 'error', message: msg } : null))
    }
  }

  const handleVideoInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (file) {
      void startVideoUpload(file)
    }
  }

  const handleDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setDragging(false)
    const file = event.dataTransfer.files?.[0]
    if (file) {
      void startVideoUpload(file)
    }
  }

  const handlePause = () => {
    uploaderRef.current?.pause()
  }

  const handleResume = async () => {
    setVideoError('')
    try {
      await uploaderRef.current?.resume()
    } catch (err) {
      setVideoError(err instanceof Error ? err.message : '恢复上传失败')
    }
  }

  const handleAbort = async () => {
    await uploaderRef.current?.abort()
    uploaderRef.current = null
    setSelectedFile(null)
    setProgress(null)
    setVideoError('')
  }

  const handleCoverFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) {
      return
    }
    setCoverNotice(null)
    if (!ALLOWED_COVER_TYPES.includes(file.type)) {
      setCoverNotice({ type: 'error', text: '封面仅支持 JPG、PNG、WebP 或 GIF 图片' })
      return
    }
    if (file.size > MAX_COVER_BYTES) {
      setCoverNotice({ type: 'error', text: '封面图片大小不能超过 5MB' })
      return
    }

    setCoverUploading(true)
    try {
      const res = await uploadApi.uploadImageDirect('cover', file)
      setCoverUrl(res.public_url)
      setCoverNotice({ type: 'success', text: `封面直传成功：${res.public_url}` })
    } catch (err) {
      setCoverNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '封面上传失败',
      })
    } finally {
      setCoverUploading(false)
    }
  }

  const displayPercent =
    progress?.phase === 'hashing' ? progress.hashPercent : (progress?.uploadPercent ?? 0)

  return (
    <div className="upload-page">
      <BiliHeader />

      {initializing ? (
        <div className="upload-guest">
          <p>正在加载创作中心...</p>
        </div>
      ) : !user ? (
        <div className="upload-guest">
          <h2>登录后开始视频投稿</h2>
          <p>支持 8MB 分片并发直传 MinIO、SHA-256 秒传、断点续传与异步转码队列。</p>
          <button
            type="button"
            className="upload-primary-btn"
            onClick={() => openAuthModal('login')}
          >
            立即登录
          </button>
        </div>
      ) : (
        <main className="upload-container">
          <section className="upload-card">
            <div className="upload-header">
              <h1 className="upload-header__title">
                <span className="upload-header__bar" />
                <span>创作中心 · 视频投稿上传</span>
              </h1>
              <span className="upload-header__sub">分片直传 MinIO · 秒传 · 断点续传</span>
            </div>

            <input
              ref={videoInputRef}
              type="file"
              accept=".mp4,.webm,.mov,.mkv,video/mp4,video/webm,video/quicktime,video/x-matroska"
              onChange={handleVideoInputChange}
              style={{ display: 'none' }}
              aria-label="选择上传视频文件"
            />

            {!selectedFile || !progress ? (
              <div
                className={`upload-dropzone ${dragging ? 'upload-dropzone--active' : ''}`}
                onClick={() => videoInputRef.current?.click()}
                onDragOver={(e) => {
                  e.preventDefault()
                  setDragging(true)
                }}
                onDragLeave={() => setDragging(false)}
                onDrop={handleDrop}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    videoInputRef.current?.click()
                  }
                }}
              >
                <div className="upload-dropzone__icon">
                  <svg width="48" height="48" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path
                      d="M12 16V5m0 0l-4.5 4.5M12 5l4.5 4.5M4 17v1.5A2.5 2.5 0 006.5 21h11a2.5 2.5 0 002.5-2.5V17"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </div>
                <div className="upload-dropzone__title">拖拽视频到此处或点击选择文件</div>
                <div className="upload-dropzone__hint">
                  支持 MP4 / WebM / MOV / MKV，最大 4GB，后台自动分片直传至对象存储
                </div>
                <span className="upload-primary-btn">上传视频</span>
              </div>
            ) : (
              <div className="upload-task-panel">
                <div className="upload-task-top">
                  <div className="upload-task-name">
                    {selectedFile.name}（{formatBytes(selectedFile.size)}）
                  </div>
                  <div className="upload-task-badges">
                    {progress.instant && <span className="upload-badge upload-badge--pink">秒传命中</span>}
                    {progress.resumed && !progress.instant && (
                      <span className="upload-badge upload-badge--blue">断点续传</span>
                    )}
                    {progress.phase === 'completed' ? (
                      <span className="upload-badge upload-badge--green">上传完成</span>
                    ) : (
                      <span className="upload-badge upload-badge--blue">{displayPercent}%</span>
                    )}
                  </div>
                </div>

                <div className="upload-progress-track">
                  <div
                    className="upload-progress-fill"
                    style={{ width: `${displayPercent}%` }}
                  />
                </div>

                <div className="upload-task-meta">
                  <span>{progress.message}</span>
                  <span>
                    {formatBytes(progress.uploadedBytes)} / {formatBytes(progress.totalBytes)}
                    {progress.speedBytesPerSec > 0
                      ? ` · ${formatBytes(progress.speedBytesPerSec)}/s`
                      : ''}
                  </span>
                </div>

                <div className="upload-task-actions">
                  {progress.phase === 'uploading' && (
                    <button type="button" className="upload-secondary-btn" onClick={handlePause}>
                      暂停上传
                    </button>
                  )}
                  {progress.phase === 'paused' && (
                    <button type="button" className="upload-primary-btn" onClick={handleResume}>
                      继续上传
                    </button>
                  )}
                  {progress.phase !== 'completed' && (
                    <button type="button" className="upload-danger-btn" onClick={handleAbort}>
                      取消任务
                    </button>
                  )}
                  {(progress.phase === 'completed' || progress.phase === 'error') && (
                    <button
                      type="button"
                      className="upload-secondary-btn"
                      onClick={() => videoInputRef.current?.click()}
                    >
                      继续上传新视频
                    </button>
                  )}
                </div>

                {progress.phase === 'completed' && (
                  <div className="upload-result-box" role="status">
                    <div>任务 ID：{progress.taskId}</div>
                    <div>对象路径：{progress.objectKey}</div>
                    <div>转码任务已投递至 RocketMQ（video_transcode_topic）</div>
                  </div>
                )}
              </div>
            )}

            {videoError && (
              <div className="upload-error-box" role="alert">
                {videoError}
              </div>
            )}

            <div className="upload-cover-section">
              <h2 className="upload-cover-section__title">视频封面直传（MinIO 公开读前缀 covers/）</h2>
              <div className="upload-cover-row">
                <div className="upload-cover-preview">
                  {coverUrl ? <img src={coverUrl} alt="视频封面预览" /> : <span>暂无封面</span>}
                </div>
                <div className="upload-cover-controls">
                  <input
                    ref={coverInputRef}
                    type="file"
                    accept="image/jpeg,image/png,image/webp,image/gif"
                    onChange={handleCoverFileChange}
                    style={{ display: 'none' }}
                    aria-label="选择视频封面图片"
                  />
                  <button
                    type="button"
                    className="upload-secondary-btn"
                    onClick={() => coverInputRef.current?.click()}
                    disabled={coverUploading}
                  >
                    {coverUploading ? '正在直传封面...' : '上传视频封面'}
                  </button>
                  <span className="upload-header__sub">支持 JPG / PNG / WebP / GIF，最大 5MB</span>
                  {coverNotice && (
                    <span
                      style={{
                        fontSize: 13,
                        color: coverNotice.type === 'success' ? '#15803d' : '#f85a54',
                      }}
                    >
                      {coverNotice.text}
                    </span>
                  )}
                </div>
              </div>
            </div>
          </section>
        </main>
      )}
    </div>
  )
}
