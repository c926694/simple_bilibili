import { sha256 } from 'js-sha256'
import { putBlobToPresignedUrl, uploadApi } from '../api/upload'
import type { UploadedPartInfo, VideoUploadCompleteResponse } from '../types/upload'

const HASH_CHUNK_SIZE = 4 * 1024 * 1024
const CONCURRENCY = 3
const MAX_PART_RETRIES = 3
const URL_BATCH_SIZE = 10

export type UploadPhase =
  | 'idle'
  | 'hashing'
  | 'initializing'
  | 'uploading'
  | 'paused'
  | 'completing'
  | 'completed'
  | 'aborted'
  | 'error'

export interface VideoUploadProgress {
  phase: UploadPhase
  hashPercent: number
  uploadPercent: number
  uploadedBytes: number
  totalBytes: number
  speedBytesPerSec: number
  taskId: string
  objectKey: string
  instant: boolean
  resumed: boolean
  completedParts: number
  totalParts: number
  message: string
}

async function hashFileStreaming(
  file: File,
  onProgress?: (percent: number) => void,
): Promise<string> {
  const hasher = sha256.create()
  let offset = 0
  while (offset < file.size) {
    const end = Math.min(offset + HASH_CHUNK_SIZE, file.size)
    const buffer = await file.slice(offset, end).arrayBuffer()
    hasher.update(new Uint8Array(buffer))
    offset = end
    onProgress?.(Math.min(100, Math.round((offset / file.size) * 100)))
  }
  return hasher.hex()
}

export function computeFileSHA256(
  file: File,
  onProgress?: (percent: number) => void,
): Promise<string> {
  if (typeof Worker === 'undefined') {
    return hashFileStreaming(file, onProgress)
  }

  return new Promise<string>((resolve, reject) => {
    const worker = new Worker(new URL('../workers/sha256.worker.ts', import.meta.url), {
      type: 'module',
    })
    worker.onmessage = (event: MessageEvent<{ type: string; percent?: number; hash?: string; message?: string }>) => {
      const msg = event.data
      if (msg.type === 'progress' && typeof msg.percent === 'number') {
        onProgress?.(msg.percent)
        return
      }
      if (msg.type === 'done' && msg.hash) {
        worker.terminate()
        resolve(msg.hash)
        return
      }
      if (msg.type === 'error') {
        worker.terminate()
        reject(new Error(msg.message || '计算文件哈希失败'))
      }
    }
    worker.onerror = () => {
      worker.terminate()
      void hashFileStreaming(file, onProgress).then(resolve, reject)
    }
    worker.postMessage({ file })
  })
}

export class VideoMultipartUploader {
  private readonly file: File
  private readonly onProgress: (progress: VideoUploadProgress) => void
  private paused = false
  private aborted = false
  private abortController: AbortController | null = null
  private taskId = ''
  private objectKey = ''
  private chunkSize = 0
  private totalChunks = 0
  private resumed = false
  private completedParts = new Map<number, string>()
  private uploadedBytes = 0
  private startTime = 0

  constructor(file: File, onProgress: (progress: VideoUploadProgress) => void) {
    this.file = file
    this.onProgress = onProgress
  }

  pause(): void {
    if (this.aborted || this.paused) {
      return
    }
    this.paused = true
    this.abortController?.abort()
    this.emit('paused', '已暂停上传，点击继续可从断点恢复')
  }

  async resume(): Promise<VideoUploadCompleteResponse | null> {
    if (this.aborted || !this.paused || !this.taskId) {
      return null
    }
    this.paused = false
    return this.uploadMissingPartsAndComplete()
  }

  async abort(): Promise<void> {
    this.aborted = true
    this.paused = false
    this.abortController?.abort()
    if (this.taskId) {
      try {
        await uploadApi.abortVideoUpload(this.taskId)
      } catch {
        // 已完成或已过期的任务允许忽略取消错误
      }
    }
    this.emit('aborted', '已取消上传任务')
  }

  async start(): Promise<VideoUploadCompleteResponse | null> {
    this.aborted = false
    this.paused = false
    this.emit('hashing', '正在后台计算文件指纹 (SHA-256)...', 0)

    const fileHash = await computeFileSHA256(this.file, (percent) => {
      if (!this.aborted) {
        this.emit('hashing', `正在计算文件指纹 (${percent}%)...`, percent)
      }
    })
    if (this.aborted) {
      return null
    }

    this.emit('initializing', '正在初始化上传任务并检测秒传/断点...', 100)
    const init = await uploadApi.initVideoUpload({
      file_hash: fileHash,
      file_size: this.file.size,
      file_name: this.file.name,
      content_type: this.file.type || 'video/mp4',
    })

    this.taskId = init.task_id
    this.objectKey = init.object_key
    this.chunkSize = init.chunk_size
    this.totalChunks = init.total_chunks
    this.resumed = init.resumed

    if (init.instant) {
      this.uploadedBytes = this.file.size
      this.emit('completed', '命中全站秒传，视频已极速上传完成', 100, true)
      return {
        task_id: init.task_id,
        object_key: init.object_key,
        status: 1,
      }
    }

    this.completedParts.clear()
    this.uploadedBytes = 0
    for (const part of init.uploaded_parts) {
      this.completedParts.set(part.part_number, part.etag)
      this.uploadedBytes += this.partByteLength(part.part_number)
    }

    return this.uploadMissingPartsAndComplete()
  }

  private async uploadMissingPartsAndComplete(): Promise<VideoUploadCompleteResponse | null> {
    const missing: number[] = []
    for (let partNo = 1; partNo <= this.totalChunks; partNo += 1) {
      if (!this.completedParts.has(partNo)) {
        missing.push(partNo)
      }
    }

    this.startTime = Date.now()
    const baseBytes = this.uploadedBytes
    this.abortController = new AbortController()
    const signal = this.abortController.signal

    this.emit(
      'uploading',
      this.resumed ? `已从断点恢复，跳过 ${this.completedParts.size} 个已传分片` : '正在并发分片直传至对象存储...',
      100,
    )

    for (let cursor = 0; cursor < missing.length; cursor += URL_BATCH_SIZE) {
      if (this.paused || this.aborted) {
        return null
      }
      const batchNumbers = missing.slice(cursor, cursor + URL_BATCH_SIZE)
      const { urls } = await uploadApi.getVideoPartUrls(this.taskId, batchNumbers)
      const queue = [...urls]

      const workers = Array.from({ length: Math.min(CONCURRENCY, queue.length) }, async () => {
        while (queue.length > 0) {
          if (this.paused || this.aborted) {
            return
          }
          const item = queue.shift()
          if (!item) {
            return
          }
          const etag = await this.uploadSinglePart(item.part_number, item.upload_url, signal)
          this.completedParts.set(item.part_number, etag)
          this.uploadedBytes += this.partByteLength(item.part_number)
          const elapsedSec = Math.max(0.1, (Date.now() - this.startTime) / 1000)
          const speed = Math.round((this.uploadedBytes - baseBytes) / elapsedSec)
          this.emit(
            'uploading',
            `正在上传分片 ${this.completedParts.size} / ${this.totalChunks}`,
            100,
            false,
            speed,
          )
        }
      })

      try {
        await Promise.all(workers)
      } catch (err) {
        if (this.paused || this.aborted) {
          return null
        }
        throw err
      }
    }

    if (this.paused || this.aborted) {
      return null
    }

    this.emit('completing', '分片上传完毕，正在通知对象存储合并文件...', 100)
    const parts: Pick<UploadedPartInfo, 'part_number' | 'etag'>[] = Array.from(
      this.completedParts.entries(),
    )
      .sort((a, b) => a[0] - b[0])
      .map(([part_number, etag]) => ({ part_number, etag }))

    const done = await uploadApi.completeVideoUpload(this.taskId, parts)
    this.emit('completed', '视频上传并合并完成，请完善稿件信息后提交投稿', 100)
    return done
  }

  private async uploadSinglePart(
    partNumber: number,
    uploadUrl: string,
    signal: AbortSignal,
  ): Promise<string> {
    const start = (partNumber - 1) * this.chunkSize
    const end = Math.min(start + this.chunkSize, this.file.size)
    const chunk = this.file.slice(start, end)

    let lastError: unknown = null
    for (let attempt = 1; attempt <= MAX_PART_RETRIES; attempt += 1) {
      if (this.paused || this.aborted) {
        throw new Error('aborted')
      }
      try {
        return await putBlobToPresignedUrl(uploadUrl, chunk, {}, signal)
      } catch (err) {
        lastError = err
        if (this.paused || this.aborted) {
          throw err
        }
      }
    }
    throw lastError instanceof Error ? lastError : new Error(`分片 ${partNumber} 上传失败`)
  }

  private partByteLength(partNumber: number): number {
    const start = (partNumber - 1) * this.chunkSize
    const end = Math.min(start + this.chunkSize, this.file.size)
    return Math.max(0, end - start)
  }

  private emit(
    phase: UploadPhase,
    message: string,
    hashPercent = 100,
    instant = false,
    speedBytesPerSec = 0,
  ): void {
    const uploadPercent =
      this.file.size > 0
        ? Math.min(100, Math.round((this.uploadedBytes / this.file.size) * 100))
        : 0
    this.onProgress({
      phase,
      hashPercent,
      uploadPercent: instant ? 100 : uploadPercent,
      uploadedBytes: this.uploadedBytes,
      totalBytes: this.file.size,
      speedBytesPerSec,
      taskId: this.taskId,
      objectKey: this.objectKey,
      instant,
      resumed: this.resumed,
      completedParts: this.completedParts.size,
      totalParts: this.totalChunks,
      message,
    })
  }
}
