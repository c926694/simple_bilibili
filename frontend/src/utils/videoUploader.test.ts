import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tokenStorage } from '../api/client'
import { VideoMultipartUploader, type VideoUploadProgress } from './videoUploader'

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('VideoMultipartUploader 分片直传、断点续传与秒传', () => {
  beforeEach(() => {
    localStorage.clear()
    tokenStorage.setTokens({ access_token: 'acc', refresh_token: 'ref' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('命中秒传时跳过所有分片上传并直接完成', async () => {
    const file = new File([new Uint8Array(1024)], 'instant.mp4', { type: 'video/mp4' })
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      jsonResponse({
        code: 0,
        data: {
          task_id: '3001',
          instant: true,
          resumed: false,
          object_key: 'raw/1/3001.mp4',
          chunk_size: 0,
          total_chunks: 0,
          uploaded_parts: [],
        },
        msg: 'ok',
      }),
    )

    const events: VideoUploadProgress[] = []
    const uploader = new VideoMultipartUploader(file, (p) => events.push(p))
    const done = await uploader.start()

    expect(done).toEqual({ task_id: '3001', object_key: 'raw/1/3001.mp4', status: 1 })
    expect(fetchSpy).toHaveBeenCalledTimes(1)
    expect(events[events.length - 1].instant).toBe(true)
    expect(events[events.length - 1].uploadPercent).toBe(100)
  })

  it('断点续传时跳过已上传分片，仅补传缺失分片并合并', async () => {
    const file = new File([new Uint8Array(16)], 'resume.mp4', { type: 'video/mp4' })
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(
        jsonResponse({
          code: 0,
          data: {
            task_id: '3002',
            instant: false,
            resumed: true,
            object_key: 'raw/1/3002.mp4',
            chunk_size: 8,
            total_chunks: 2,
            uploaded_parts: [{ part_number: 1, etag: 'etag-p1', size: 8 }],
          },
          msg: 'ok',
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          code: 0,
          data: {
            urls: [{ part_number: 2, upload_url: 'http://127.0.0.1:8088/bilibili/raw/1/3002.mp4?partNumber=2' }],
            expires_in: 900,
          },
          msg: 'ok',
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 200, headers: { ETag: '"etag-p2"' } }))
      .mockResolvedValueOnce(
        jsonResponse({
          code: 0,
          data: { task_id: '3002', object_key: 'raw/1/3002.mp4', status: 1 },
          msg: 'ok',
        }),
      )

    const uploader = new VideoMultipartUploader(file, () => {})
    const done = await uploader.start()

    expect(done).toEqual({ task_id: '3002', object_key: 'raw/1/3002.mp4', status: 1 })
    expect(fetchSpy).toHaveBeenCalledTimes(4)
    const partReqBody = JSON.parse((fetchSpy.mock.calls[1][1] as RequestInit).body as string)
    expect(partReqBody.part_numbers).toEqual([2])
    const completeBody = JSON.parse((fetchSpy.mock.calls[3][1] as RequestInit).body as string)
    expect(completeBody.parts).toEqual([
      { part_number: 1, etag: 'etag-p1' },
      { part_number: 2, etag: 'etag-p2' },
    ])
  })
})
