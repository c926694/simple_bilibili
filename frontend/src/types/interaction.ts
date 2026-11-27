import type { VideoSummary } from './video'

export interface WatchProgressInfo {
  part_id: string
  part_index: number
  progress_sec: number
  watched_at: string
}

export interface VideoInteractionState {
  liked: boolean
  coin_count: number
  favorited: boolean
  favorite_folder_ids: string[]
  coin_balance: number
  watch_progress: WatchProgressInfo | null
}

export interface LikeResponse {
  liked: boolean
  delta: number
}

export interface CoinResponse {
  coin_count: number
  added_coins: number
  coin_balance: number
}

export interface TripleResponse {
  liked: boolean
  like_delta: number
  coin_count: number
  added_coins: number
  coin_balance: number
  coin_skipped_reason: '' | 'insufficient_coins' | 'self_video' | string
  favorited: boolean
  favorite_delta: number
  favorite_folder_ids: string[]
}

export interface CountedResponse {
  counted: boolean
}

export interface FavoriteFolder {
  id: string
  user_id: string
  title: string
  is_default: boolean
  is_public: boolean
  item_count: number
  created_at: string
}

export interface FavoriteFolderVideosResponse {
  folder: FavoriteFolder
  items: VideoSummary[]
  total: number
  page: number
  page_size: number
}

export interface UpdateVideoFavoritesResponse {
  favorited: boolean
  favorite_folder_ids: string[]
  delta: number
}

export interface WatchProgressSaveResponse {
  video_id: string
  part_id: string
  progress_sec: number
  watched_at: string
}

export interface WatchHistoryEntry {
  video: VideoSummary
  part_id: string
  part_index: number
  part_title: string
  progress_sec: number
  watched_at: string
}

export interface WatchHistoryPage {
  items: WatchHistoryEntry[]
  total: number
  page: number
  page_size: number
}

export interface CheckinStatus {
  checked_in_today: boolean
  just_checked_in?: boolean
  month: string
  month_days: number
  streak_days: number
  checked_dates: number[]
  coin_balance: number
}

export interface CheckinResponse extends CheckinStatus {
  reward_coins: number
}
