export type ImageBizType = 'avatar' | 'cover' | 'dynamic'

export interface ImagePresignRequest {
  biz_type: ImageBizType
  content_type: string
  file_size: number
}

export interface ImagePresignResponse {
  upload_url: string
  object_key: string
  public_url: string
  headers: Record<string, string>
  expires_in: number
}

export interface UploadedPartInfo {
  part_number: number
  etag: string
  size?: number
}

export interface VideoUploadInitRequest {
  file_hash: string
  file_size: number
  file_name: string
  content_type: string
}

export interface VideoUploadInitResponse {
  task_id: string
  instant: boolean
  resumed: boolean
  object_key: string
  chunk_size: number
  total_chunks: number
  uploaded_parts: UploadedPartInfo[]
}

export interface VideoPartURL {
  part_number: number
  upload_url: string
}

export interface VideoPartURLsResponse {
  urls: VideoPartURL[]
  expires_in: number
}

export interface VideoUploadCompleteResponse {
  task_id: string
  object_key: string
  status: number
}

export interface VideoUploadTaskDetail {
  task_id: string
  file_name: string
  file_size: number
  chunk_size: number
  total_chunks: number
  object_key: string
  status: number
  uploaded_parts: UploadedPartInfo[]
}
