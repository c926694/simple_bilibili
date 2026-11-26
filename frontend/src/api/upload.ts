import { apiRequest } from './client'
import type {
  ImageBizType,
  ImagePresignRequest,
  ImagePresignResponse,
  UploadedPartInfo,
  VideoPartURLsResponse,
  VideoUploadCompleteResponse,
  VideoUploadInitRequest,
  VideoUploadInitResponse,
  VideoUploadTaskDetail,
} from '../types/upload'

export async function putBlobToPresignedUrl(
  uploadUrl: string,
  blob: Blob,
  headers: Record<string, string> = {},
  signal?: AbortSignal,
): Promise<string> {
  const res = await fetch(uploadUrl, {
    method: 'PUT',
    headers,
    body: blob,
    signal,
  })
  if (!res.ok) {
    throw new Error(`直传对象存储失败（HTTP ${res.status}）`)
  }
  const rawEtag = res.headers.get('ETag') ?? res.headers.get('etag') ?? ''
  return rawEtag.replace(/^"|"$/g, '')
}

export const uploadApi = {
  presignImage(req: ImagePresignRequest): Promise<ImagePresignResponse> {
    return apiRequest<ImagePresignResponse>('/api/v1/uploads/image-presign', {
      method: 'POST',
      body: req,
      auth: true,
    })
  },

  async uploadImageDirect(bizType: ImageBizType, file: File): Promise<ImagePresignResponse> {
    const presign = await uploadApi.presignImage({
      biz_type: bizType,
      content_type: file.type,
      file_size: file.size,
    })
    await putBlobToPresignedUrl(presign.upload_url, file, presign.headers)
    return presign
  },

  initVideoUpload(req: VideoUploadInitRequest): Promise<VideoUploadInitResponse> {
    return apiRequest<VideoUploadInitResponse>('/api/v1/uploads/video/init', {
      method: 'POST',
      body: req,
      auth: true,
    })
  },

  getVideoPartUrls(taskId: string, partNumbers: number[]): Promise<VideoPartURLsResponse> {
    return apiRequest<VideoPartURLsResponse>(
      `/api/v1/uploads/video/${encodeURIComponent(taskId)}/part-urls`,
      {
        method: 'POST',
        body: { part_numbers: partNumbers },
        auth: true,
      },
    )
  },

  completeVideoUpload(
    taskId: string,
    parts: Pick<UploadedPartInfo, 'part_number' | 'etag'>[],
  ): Promise<VideoUploadCompleteResponse> {
    return apiRequest<VideoUploadCompleteResponse>(
      `/api/v1/uploads/video/${encodeURIComponent(taskId)}/complete`,
      {
        method: 'POST',
        body: { parts },
        auth: true,
      },
    )
  },

  abortVideoUpload(taskId: string): Promise<null> {
    return apiRequest<null>(`/api/v1/uploads/video/${encodeURIComponent(taskId)}/abort`, {
      method: 'POST',
      auth: true,
    })
  },

  getVideoUploadTask(taskId: string): Promise<VideoUploadTaskDetail> {
    return apiRequest<VideoUploadTaskDetail>(
      `/api/v1/uploads/video/${encodeURIComponent(taskId)}`,
      {
        method: 'GET',
        auth: true,
      },
    )
  },
}
