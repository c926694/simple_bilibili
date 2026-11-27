import type { PublicUserProfile } from './user'

export interface Category {
  id: number
  name: string
  sort_order: number
}

export interface VideoAuthor {
  id: string
  username: string
  nickname: string
  avatar_url: string
  bio?: string
  follower_count?: number
  following_count?: number
}

export interface VideoStat {
  play_count: number
  danmaku_count: number
  like_count: number
  coin_count: number
  favorite_count: number
  share_count: number
  comment_count: number
}

export interface VideoPart {
  id: string
  part_index: number
  title: string
  duration_sec: number
  transcode_status: number
  hls_1080p_url?: string | null
  hls_720p_url?: string | null
  hls_480p_url?: string | null
}

export interface VideoCategoryRef {
  id: number
  name: string
}

export interface VideoSummary {
  id: string
  bv_id: string
  title: string
  description: string
  cover_url: string
  duration_sec: number
  status: number
  published_at?: string | null
  created_at: string
  category?: VideoCategoryRef | null
  author: VideoAuthor
  stat: VideoStat
  parts?: VideoPart[]
}

export interface VideoDetail extends VideoSummary {
  tags: string[]
  parts: VideoPart[]
}

export interface PagedList<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface VideoSearchResponse {
  videos: PagedList<VideoSummary>
  users: {
    items: PublicUserProfile[]
    total: number
  }
}

export interface CreateVideoPartInput {
  upload_task_id: string
  title: string
}

export interface CreateVideoRequest {
  title: string
  description?: string
  category_id: number
  cover_url?: string
  tags?: string[]
  upload_task_id?: string
  parts?: CreateVideoPartInput[]
}

export interface UpdateVideoPartInput {
  part_id?: string
  upload_task_id?: string
  title: string
}

export interface UpdateVideoRequest {
  title: string
  description?: string
  category_id: number
  cover_url?: string
  tags?: string[]
  parts?: UpdateVideoPartInput[]
}

export type VideoSortOption = 'latest' | 'play' | 'danmaku' | 'hot'
