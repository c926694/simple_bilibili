import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type DragEvent,
  type FormEvent,
  type KeyboardEvent,
} from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ApiError } from '../../api/client'
import { uploadApi } from '../../api/upload'
import { videoApi } from '../../api/video'
import { BiliHeader } from '../../components/BiliHeader/BiliHeader'
import { useAuth } from '../../context/AuthContext'
import type { Category, VideoDetail } from '../../types/video'
import {
  formatDateShort,
  formatDuration,
  getTranscodeStatusLabel,
  getVideoStatusMeta,
} from '../../utils/format'
import { VideoMultipartUploader, type VideoUploadProgress } from '../../utils/videoUploader'
import './UploadPage.css'

const ALLOWED_VIDEO_EXTS = ['.mp4', '.webm', '.mov', '.mkv']
const ALLOWED_COVER_TYPES = ['image/jpeg', 'image/png', 'image/webp', 'image/gif']
const MAX_COVER_BYTES = 5 * 1024 * 1024

interface DraftPart {
  uploadTaskId: string
  fileName: string
  objectKey: string
  title: string
}

function stripExtension(name: string): string {
  const idx = name.lastIndexOf('.')
  return idx > 0 ? name.slice(0, idx) : name
}

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
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = searchParams.get('tab') === 'manage' ? 'manage' : 'submit'
  const editBvidParam = searchParams.get('edit') || ''

  const videoInputRef = useRef<HTMLInputElement | null>(null)
  const coverInputRef = useRef<HTMLInputElement | null>(null)
  const editCoverInputRef = useRef<HTMLInputElement | null>(null)
  const uploaderRef = useRef<VideoMultipartUploader | null>(null)

  const [categories, setCategories] = useState<Category[]>([])

  // 上传与投稿表单状态
  const [dragging, setDragging] = useState(false)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [progress, setProgress] = useState<VideoUploadProgress | null>(null)
  const [videoError, setVideoError] = useState('')
  const [draftParts, setDraftParts] = useState<DraftPart[]>([])

  const [title, setTitle] = useState('')
  const [categoryId, setCategoryId] = useState<number>(0)
  const [description, setDescription] = useState('')
  const [tags, setTags] = useState<string[]>([])
  const [tagInput, setTagInput] = useState('')
  const [coverUrl, setCoverUrl] = useState('')
  const [coverUploading, setCoverUploading] = useState(false)
  const [coverNotice, setCoverNotice] = useState<{ type: 'success' | 'error'; text: string } | null>(
    null,
  )
  const [submitting, setSubmitting] = useState(false)
  const [submitNotice, setSubmitNotice] = useState<{
    type: 'success' | 'error'
    text: string
  } | null>(null)

  // 稿件管理状态
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [myVideos, setMyVideos] = useState<VideoDetail[]>([])
  const [myLoading, setMyLoading] = useState(false)
  const [manageNotice, setManageNotice] = useState<{
    type: 'success' | 'error'
    text: string
  } | null>(null)

  // 在线编辑状态
  const [editingBvid, setEditingBvid] = useState<string>('')
  const [editTitle, setEditTitle] = useState('')
  const [editCategoryId, setEditCategoryId] = useState<number>(0)
  const [editDescription, setEditDescription] = useState('')
  const [editCoverUrl, setEditCoverUrl] = useState('')
  const [editTagsInput, setEditTagsInput] = useState('')
  const [editParts, setEditParts] = useState<{ partId: string; title: string }[]>([])
  const [editSaving, setEditSaving] = useState(false)

  useEffect(() => {
    videoApi
      .listCategories()
      .then((list) => {
        setCategories(list)
        if (list.length > 0) {
          setCategoryId((prev) => (prev > 0 ? prev : list[0].id))
        }
      })
      .catch(() => {})
  }, [])

  const loadMyVideos = useCallback(
    async (silent = false) => {
      if (!user) {
        return
      }
      if (!silent) {
        setMyLoading(true)
      }
      try {
        const res = await videoApi.listMyVideos({
          status: statusFilter === 'all' ? undefined : Number(statusFilter),
          page: 1,
          page_size: 30,
        })
        setMyVideos(res.items)
      } catch (err) {
        if (!silent) {
          setManageNotice({
            type: 'error',
            text: err instanceof ApiError ? err.message : '加载稿件列表失败',
          })
        }
      } finally {
        if (!silent) {
          setMyLoading(false)
        }
      }
    },
    [user, statusFilter],
  )

  useEffect(() => {
    if (activeTab === 'manage' && user) {
      void loadMyVideos(false)
    }
  }, [activeTab, user, loadMyVideos])

  // 若存在转码中的稿件，每 3 秒静默轮询刷新状态
  const hasTranscoding = myVideos.some((item) => item.status === 1)
  useEffect(() => {
    if (activeTab !== 'manage' || !hasTranscoding) {
      return
    }
    const timer = window.setInterval(() => {
      void loadMyVideos(true)
    }, 3000)
    return () => window.clearInterval(timer)
  }, [activeTab, hasTranscoding, loadMyVideos])

  const openEditor = useCallback((video: VideoDetail) => {
    setEditingBvid(video.bv_id)
    setEditTitle(video.title)
    setEditCategoryId(video.category?.id ?? 0)
    setEditDescription(video.description || '')
    setEditCoverUrl(video.cover_url || '')
    setEditTagsInput((video.tags || []).join(', '))
    setEditParts(
      (video.parts || []).map((part) => ({
        partId: part.id,
        title: part.title,
      })),
    )
    setManageNotice(null)
  }, [])

  useEffect(() => {
    if (activeTab === 'manage' && editBvidParam && myVideos.length > 0 && !editingBvid) {
      const target = myVideos.find((v) => v.bv_id === editBvidParam)
      if (target) {
        openEditor(target)
      }
    }
  }, [activeTab, editBvidParam, myVideos, editingBvid, openEditor])

  const switchTab = (next: 'submit' | 'manage') => {
    const params = new URLSearchParams(searchParams)
    if (next === 'manage') {
      params.set('tab', 'manage')
    } else {
      params.delete('tab')
      params.delete('edit')
    }
    setSearchParams(params)
  }

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
    setSubmitNotice(null)
    setSelectedFile(file)
    const uploader = new VideoMultipartUploader(file, (next) => {
      setProgress(next)
    })
    uploaderRef.current = uploader

    try {
      const completed = await uploader.start()
      if (completed) {
        const defaultTitle = stripExtension(file.name).slice(0, 128) || '分 P 视频'
        setDraftParts((prev) => {
          if (prev.some((p) => p.uploadTaskId === completed.task_id)) {
            return prev
          }
          return [
            ...prev,
            {
              uploadTaskId: completed.task_id,
              fileName: file.name,
              objectKey: completed.object_key,
              title: defaultTitle,
            },
          ]
        })
        setTitle((prev) => (prev.trim() ? prev : defaultTitle))
      }
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
      const completed = await uploaderRef.current?.resume()
      if (completed && selectedFile) {
        const defaultTitle = stripExtension(selectedFile.name).slice(0, 128) || '分 P 视频'
        setDraftParts((prev) => {
          if (prev.some((p) => p.uploadTaskId === completed.task_id)) {
            return prev
          }
          return [
            ...prev,
            {
              uploadTaskId: completed.task_id,
              fileName: selectedFile.name,
              objectKey: completed.object_key,
              title: defaultTitle,
            },
          ]
        })
        setTitle((prev) => (prev.trim() ? prev : defaultTitle))
      }
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

  const handleUploadCover = async (
    file: File,
    onDone: (publicUrl: string) => void,
    setBusy: (v: boolean) => void,
  ) => {
    if (!ALLOWED_COVER_TYPES.includes(file.type)) {
      setCoverNotice({ type: 'error', text: '封面仅支持 JPG、PNG、WebP 或 GIF 图片' })
      return
    }
    if (file.size > MAX_COVER_BYTES) {
      setCoverNotice({ type: 'error', text: '封面图片大小不能超过 5MB' })
      return
    }
    setCoverNotice(null)
    setBusy(true)
    try {
      const res = await uploadApi.uploadImageDirect('cover', file)
      onDone(res.public_url)
      setCoverNotice({ type: 'success', text: '封面上传成功' })
    } catch (err) {
      setCoverNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '封面上传失败',
      })
    } finally {
      setBusy(false)
    }
  }

  const handleCoverFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (file) {
      await handleUploadCover(file, setCoverUrl, setCoverUploading)
    }
  }

  const handleEditCoverChange = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) {
      return
    }
    try {
      const res = await uploadApi.uploadImageDirect('cover', file)
      setEditCoverUrl(res.public_url)
      setManageNotice({ type: 'success', text: '新封面已上传，请点击保存生效' })
    } catch (err) {
      setManageNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '封面上传失败',
      })
    }
  }

  const addTagFromInput = () => {
    const items = tagInput
      .split(/[,，\s]+/)
      .map((item) => item.trim())
      .filter(Boolean)
    if (items.length === 0) {
      return
    }
    setTags((prev) => {
      const next = [...prev]
      for (const item of items) {
        if (next.length >= 10) {
          break
        }
        if (!next.includes(item) && item.length <= 32) {
          next.push(item)
        }
      }
      return next
    })
    setTagInput('')
  }

  const handleTagKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' || event.key === ',') {
      event.preventDefault()
      addTagFromInput()
    }
  }

  const moveDraftPart = (index: number, delta: -1 | 1) => {
    const target = index + delta
    if (target < 0 || target >= draftParts.length) {
      return
    }
    setDraftParts((prev) => {
      const copy = [...prev]
      ;[copy[index], copy[target]] = [copy[target], copy[index]]
      return copy
    })
  }

  const handleSubmitVideo = async (event: FormEvent) => {
    event.preventDefault()
    setSubmitNotice(null)
    if (draftParts.length === 0) {
      setSubmitNotice({ type: 'error', text: '请先至少上传完成一个视频分 P' })
      return
    }
    const trimmedTitle = title.trim()
    if (!trimmedTitle) {
      setSubmitNotice({ type: 'error', text: '请填写视频标题' })
      return
    }
    if (!categoryId) {
      setSubmitNotice({ type: 'error', text: '请选择视频分区' })
      return
    }

    setSubmitting(true)
    try {
      const created = await videoApi.createVideo({
        title: trimmedTitle,
        description: description.trim() || undefined,
        category_id: categoryId,
        cover_url: coverUrl || undefined,
        tags,
        parts: draftParts.map((part, idx) => ({
          upload_task_id: part.uploadTaskId,
          title: part.title.trim() || `P${idx + 1}`,
        })),
      })

      setDraftParts([])
      setSelectedFile(null)
      setProgress(null)
      setTitle('')
      setDescription('')
      setTags([])
      setTagInput('')
      setCoverUrl('')
      setCoverNotice(null)
      setManageNotice({
        type: 'success',
        text:
          created.status === 3
            ? `稿件「${created.title}」已复用转码产物并立即发布！`
            : `稿件「${created.title}」提交成功，后台正在进行 FFmpeg 转码...`,
      })
      switchTab('manage')
      await loadMyVideos(false)
    } catch (err) {
      setSubmitNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '提交稿件失败，请稍后重试',
      })
    } finally {
      setSubmitting(false)
    }
  }

  const handleSaveEdit = async (event: FormEvent) => {
    event.preventDefault()
    if (!editingBvid) {
      return
    }
    setEditSaving(true)
    setManageNotice(null)
    try {
      const parsedTags = Array.from(
        new Set(
          editTagsInput
            .split(/[,，]+/)
            .map((t) => t.trim())
            .filter(Boolean),
        ),
      ).slice(0, 10)
      await videoApi.updateVideo(editingBvid, {
        title: editTitle.trim(),
        description: editDescription.trim(),
        category_id: editCategoryId,
        cover_url: editCoverUrl || undefined,
        tags: parsedTags,
        parts: editParts.map((p, idx) => ({
          part_id: p.partId,
          title: p.title.trim() || `P${idx + 1}`,
        })),
      })
      setEditingBvid('')
      setManageNotice({ type: 'success', text: '稿件信息已更新' })
      await loadMyVideos(false)
    } catch (err) {
      setManageNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '保存稿件修改失败',
      })
    } finally {
      setEditSaving(false)
    }
  }

  const handleDeleteVideo = async (video: VideoDetail) => {
    if (!window.confirm(`确认下架稿件「${video.title}」吗？下架后其他用户将无法观看。`)) {
      return
    }
    try {
      await videoApi.deleteVideo(video.bv_id)
      setManageNotice({ type: 'success', text: `稿件「${video.title}」已下架` })
      if (editingBvid === video.bv_id) {
        setEditingBvid('')
      }
      await loadMyVideos(false)
    } catch (err) {
      setManageNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '下架稿件失败',
      })
    }
  }

  const handleRetryTranscode = async (bvid: string, partId: string) => {
    try {
      await videoApi.retryPartTranscode(bvid, partId)
      setManageNotice({ type: 'success', text: '已重新投递转码任务，请稍候查看进度' })
      await loadMyVideos(false)
    } catch (err) {
      setManageNotice({
        type: 'error',
        text: err instanceof ApiError ? err.message : '重新转码失败',
      })
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
          <p>支持 8MB 分片并发直传 MinIO、SHA-256 秒传、断点续传与 FFmpeg 多清晰度转码。</p>
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
          <div className="creator-tabs">
            <button
              type="button"
              className={`creator-tab ${activeTab === 'submit' ? 'creator-tab--active' : ''}`}
              onClick={() => switchTab('submit')}
            >
              视频投稿
            </button>
            <button
              type="button"
              className={`creator-tab ${activeTab === 'manage' ? 'creator-tab--active' : ''}`}
              onClick={() => switchTab('manage')}
            >
              稿件管理
            </button>
          </div>

          {activeTab === 'submit' ? (
            <section className="upload-card">
              <div className="upload-header">
                <h1 className="upload-header__title">
                  <span className="upload-header__bar" />
                  <span>创作中心 · 视频投稿</span>
                </h1>
                <span className="upload-header__sub">分片直传 MinIO · 秒传 · FFmpeg 自动转码</span>
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
                    支持 MP4 / WebM / MOV / MKV，最大 4GB，可连续添加多个分 P
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
                      {progress.instant && (
                        <span className="upload-badge upload-badge--pink">秒传命中</span>
                      )}
                      {progress.resumed && !progress.instant && (
                        <span className="upload-badge upload-badge--blue">断点续传</span>
                      )}
                      {progress.phase === 'completed' ? (
                        <span className="upload-badge upload-badge--green">已加入分 P 清单</span>
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
                        继续添加分 P 视频
                      </button>
                    )}
                  </div>
                </div>
              )}

              {videoError && (
                <div className="upload-error-box" role="alert">
                  {videoError}
                </div>
              )}

              {draftParts.length > 0 && (
                <div className="draft-parts-box">
                  <div className="draft-parts-box__head">
                    <h2>已上传分 P 清单（{draftParts.length}）</h2>
                    <button
                      type="button"
                      className="upload-secondary-btn"
                      onClick={() => videoInputRef.current?.click()}
                    >
                      + 添加分 P
                    </button>
                  </div>
                  <ul className="draft-parts-list">
                    {draftParts.map((part, idx) => (
                      <li key={part.uploadTaskId} className="draft-part-row">
                        <span className="draft-part-row__idx">P{idx + 1}</span>
                        <input
                          type="text"
                          className="upload-input draft-part-row__input"
                          value={part.title}
                          maxLength={128}
                          onChange={(e) => {
                            const val = e.target.value
                            setDraftParts((prev) =>
                              prev.map((item, i) => (i === idx ? { ...item, title: val } : item)),
                            )
                          }}
                          placeholder={`分 P${idx + 1} 标题`}
                        />
                        <div className="draft-part-row__ops">
                          <button
                            type="button"
                            className="upload-mini-btn"
                            disabled={idx === 0}
                            onClick={() => moveDraftPart(idx, -1)}
                          >
                            上移
                          </button>
                          <button
                            type="button"
                            className="upload-mini-btn"
                            disabled={idx === draftParts.length - 1}
                            onClick={() => moveDraftPart(idx, 1)}
                          >
                            下移
                          </button>
                          <button
                            type="button"
                            className="upload-mini-btn upload-mini-btn--danger"
                            onClick={() =>
                              setDraftParts((prev) => prev.filter((_, i) => i !== idx))
                            }
                          >
                            移除
                          </button>
                        </div>
                      </li>
                    ))}
                  </ul>
                </div>
              )}

              <form className="video-form" onSubmit={handleSubmitVideo}>
                <h2 className="video-form__heading">基本设置</h2>

                <div className="video-form__row">
                  <label className="video-form__label">封面</label>
                  <div className="upload-cover-row">
                    <div className="upload-cover-preview">
                      {coverUrl ? (
                        <img src={coverUrl} alt="视频封面预览" />
                      ) : (
                        <span>未设置（转码自动截取）</span>
                      )}
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
                      <div className="upload-cover-btns">
                        <button
                          type="button"
                          className="upload-secondary-btn"
                          onClick={() => coverInputRef.current?.click()}
                          disabled={coverUploading}
                        >
                          {coverUploading ? '正在直传封面...' : '上传封面'}
                        </button>
                        {coverUrl && (
                          <button
                            type="button"
                            className="upload-secondary-btn"
                            onClick={() => setCoverUrl('')}
                          >
                            恢复自动截取
                          </button>
                        )}
                      </div>
                      <span className="upload-header__sub">
                        可选：JPG / PNG / WebP / GIF（≤5MB），不上传将由 FFmpeg 自动截取视频帧
                      </span>
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

                <div className="video-form__row">
                  <label className="video-form__label" htmlFor="video-title-input">
                    标题 *
                  </label>
                  <input
                    id="video-title-input"
                    type="text"
                    className="upload-input"
                    value={title}
                    maxLength={128}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="给你的稿件起一个吸引人的标题"
                    required
                  />
                </div>

                <div className="video-form__row">
                  <label className="video-form__label" htmlFor="video-category-select">
                    分区 *
                  </label>
                  <select
                    id="video-category-select"
                    className="upload-input upload-select"
                    value={categoryId}
                    onChange={(e) => setCategoryId(Number(e.target.value))}
                  >
                    {categories.map((cat) => (
                      <option key={cat.id} value={cat.id}>
                        {cat.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="video-form__row">
                  <label className="video-form__label" htmlFor="video-tag-input">
                    标签
                  </label>
                  <div className="video-tags-editor">
                    <div className="video-tags-input-row">
                      <input
                        id="video-tag-input"
                        type="text"
                        className="upload-input"
                        value={tagInput}
                        onChange={(e) => setTagInput(e.target.value)}
                        onKeyDown={handleTagKeyDown}
                        placeholder="输入标签后按回车添加（最多 10 个）"
                      />
                      <button
                        type="button"
                        className="upload-secondary-btn"
                        onClick={addTagFromInput}
                      >
                        添加标签
                      </button>
                    </div>
                    {tags.length > 0 && (
                      <div className="video-tags-list">
                        {tags.map((tag) => (
                          <span key={tag} className="video-tag-item">
                            {tag}
                            <button
                              type="button"
                              aria-label={`移除标签 ${tag}`}
                              onClick={() => setTags((prev) => prev.filter((item) => item !== tag))}
                            >
                              ×
                            </button>
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                <div className="video-form__row">
                  <label className="video-form__label" htmlFor="video-desc-input">
                    简介
                  </label>
                  <textarea
                    id="video-desc-input"
                    className="upload-textarea"
                    rows={4}
                    maxLength={2000}
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    placeholder="填写更全面的相关信息，让更多人找到你的视频"
                  />
                </div>

                {submitNotice && (
                  <div
                    className={
                      submitNotice.type === 'error' ? 'upload-error-box' : 'upload-result-box'
                    }
                    role="status"
                  >
                    {submitNotice.text}
                  </div>
                )}

                <div className="video-form__submit">
                  <button
                    type="submit"
                    className="upload-primary-btn upload-primary-btn--pink"
                    disabled={submitting || draftParts.length === 0}
                  >
                    {submitting ? '正在提交投稿...' : '立即投稿'}
                  </button>
                </div>
              </form>
            </section>
          ) : (
            <section className="upload-card">
              <div className="upload-header">
                <h1 className="upload-header__title">
                  <span className="upload-header__bar" />
                  <span>创作中心 · 稿件管理</span>
                </h1>
                <div className="manage-filters">
                  {[
                    { value: 'all', label: '全部' },
                    { value: '1', label: '转码中' },
                    { value: '3', label: '已发布' },
                    { value: '4', label: '转码失败' },
                  ].map((item) => (
                    <button
                      key={item.value}
                      type="button"
                      className={`manage-filter-btn ${statusFilter === item.value ? 'manage-filter-btn--active' : ''}`}
                      onClick={() => setStatusFilter(item.value)}
                    >
                      {item.label}
                    </button>
                  ))}
                </div>
              </div>

              {manageNotice && (
                <div
                  className={
                    manageNotice.type === 'error' ? 'upload-error-box' : 'upload-result-box'
                  }
                  style={{ marginTop: 0, marginBottom: 16 }}
                  role="status"
                >
                  {manageNotice.text}
                </div>
              )}

              {myLoading ? (
                <div className="manage-list" aria-label="正在加载稿件列表">
                  {Array.from({ length: 3 }).map((_, idx) => (
                    <div key={idx} className="manage-item manage-item--skeleton" aria-hidden="true">
                      <div className="bili-skeleton manage-skeleton__cover" />
                      <div className="manage-skeleton__body">
                        <div className="bili-skeleton manage-skeleton__line" />
                        <div className="bili-skeleton manage-skeleton__line manage-skeleton__line--short" />
                      </div>
                    </div>
                  ))}
                </div>
              ) : myVideos.length === 0 ? (
                <div className="manage-empty">
                  <p>当前筛选下暂无稿件</p>
                  <button
                    type="button"
                    className="upload-primary-btn"
                    onClick={() => switchTab('submit')}
                  >
                    去投稿新视频
                  </button>
                </div>
              ) : (
                <div className="manage-list">
                  {myVideos.map((item) => {
                    const statusMeta = getVideoStatusMeta(item.status)
                    const isEditing = editingBvid === item.bv_id
                    return (
                      <article key={item.bv_id} className="manage-item">
                        <div className="manage-item__main">
                          <Link to={`/video/${item.bv_id}`} className="manage-item__cover">
                            {item.cover_url ? (
                              <img src={item.cover_url} alt={item.title} />
                            ) : (
                              <span>等待生成封面</span>
                            )}
                          </Link>

                          <div className="manage-item__body">
                            <div className="manage-item__title-row">
                              <Link to={`/video/${item.bv_id}`} className="manage-item__title">
                                {item.title}
                              </Link>
                              <span
                                className={`upload-badge upload-badge--${statusMeta.tone === 'red' ? 'red' : statusMeta.tone === 'green' ? 'green' : 'blue'}`}
                              >
                                {statusMeta.label}
                              </span>
                            </div>

                            <div className="manage-item__meta">
                              <span>{item.bv_id}</span>
                              {item.category && <span>{item.category.name}</span>}
                              <span>时长 {formatDuration(item.duration_sec)}</span>
                              <span>创建于 {formatDateShort(item.created_at)}</span>
                            </div>

                            <div className="manage-parts-strip">
                              {(item.parts || []).map((part) => (
                                <span key={part.id} className="manage-part-chip">
                                  P{part.part_index} {part.title} ·{' '}
                                  {getTranscodeStatusLabel(part.transcode_status)}
                                  {part.transcode_status === 3 && (
                                    <button
                                      type="button"
                                      className="manage-retry-link"
                                      onClick={() =>
                                        void handleRetryTranscode(item.bv_id, part.id)
                                      }
                                    >
                                      重试
                                    </button>
                                  )}
                                </span>
                              ))}
                            </div>
                          </div>

                          <div className="manage-item__actions">
                            <Link to={`/video/${item.bv_id}`} className="upload-secondary-btn">
                              查看
                            </Link>
                            <button
                              type="button"
                              className="upload-secondary-btn"
                              onClick={() =>
                                isEditing ? setEditingBvid('') : openEditor(item)
                              }
                            >
                              {isEditing ? '收起' : '编辑'}
                            </button>
                            <button
                              type="button"
                              className="upload-danger-btn"
                              onClick={() => void handleDeleteVideo(item)}
                            >
                              下架
                            </button>
                          </div>
                        </div>

                        {isEditing && (
                          <form className="manage-edit-panel" onSubmit={handleSaveEdit}>
                            <div className="manage-edit-grid">
                              <label>
                                <span>标题</span>
                                <input
                                  type="text"
                                  className="upload-input"
                                  value={editTitle}
                                  maxLength={128}
                                  onChange={(e) => setEditTitle(e.target.value)}
                                  required
                                />
                              </label>
                              <label>
                                <span>分区</span>
                                <select
                                  className="upload-input"
                                  value={editCategoryId}
                                  onChange={(e) => setEditCategoryId(Number(e.target.value))}
                                >
                                  {categories.map((cat) => (
                                    <option key={cat.id} value={cat.id}>
                                      {cat.name}
                                    </option>
                                  ))}
                                </select>
                              </label>
                              <label>
                                <span>标签（逗号分隔）</span>
                                <input
                                  type="text"
                                  className="upload-input"
                                  value={editTagsInput}
                                  onChange={(e) => setEditTagsInput(e.target.value)}
                                />
                              </label>
                              <div>
                                <span className="manage-edit-label">封面</span>
                                <div className="manage-edit-cover-row">
                                  <input
                                    ref={editCoverInputRef}
                                    type="file"
                                    accept="image/jpeg,image/png,image/webp,image/gif"
                                    onChange={handleEditCoverChange}
                                    style={{ display: 'none' }}
                                  />
                                  <button
                                    type="button"
                                    className="upload-secondary-btn"
                                    onClick={() => editCoverInputRef.current?.click()}
                                  >
                                    更换封面
                                  </button>
                                  <span className="upload-header__sub">
                                    {editCoverUrl ? '已设置封面' : '默认封面'}
                                  </span>
                                </div>
                              </div>
                            </div>

                            <label className="manage-edit-full">
                              <span>简介</span>
                              <textarea
                                className="upload-textarea"
                                rows={3}
                                value={editDescription}
                                onChange={(e) => setEditDescription(e.target.value)}
                              />
                            </label>

                            {editParts.length > 0 && (
                              <div className="manage-edit-parts">
                                <span className="manage-edit-label">分 P 标题</span>
                                {editParts.map((part, idx) => (
                                  <div key={part.partId} className="draft-part-row">
                                    <span className="draft-part-row__idx">P{idx + 1}</span>
                                    <input
                                      type="text"
                                      className="upload-input"
                                      value={part.title}
                                      onChange={(e) => {
                                        const val = e.target.value
                                        setEditParts((prev) =>
                                          prev.map((p, i) => (i === idx ? { ...p, title: val } : p)),
                                        )
                                      }}
                                    />
                                  </div>
                                ))}
                              </div>
                            )}

                            <div className="manage-edit-actions">
                              <button
                                type="submit"
                                className="upload-primary-btn"
                                disabled={editSaving}
                              >
                                {editSaving ? '正在保存...' : '保存修改'}
                              </button>
                              <button
                                type="button"
                                className="upload-secondary-btn"
                                onClick={() => setEditingBvid('')}
                              >
                                取消
                              </button>
                            </div>
                          </form>
                        )}
                      </article>
                    )
                  })}
                </div>
              )}
            </section>
          )}
        </main>
      )}
    </div>
  )
}
