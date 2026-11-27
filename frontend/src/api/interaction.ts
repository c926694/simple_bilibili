import { apiRequest } from './client'
import type {
  CheckinResponse,
  CheckinStatus,
  CoinResponse,
  CountedResponse,
  FavoriteFolder,
  FavoriteFolderVideosResponse,
  LikeResponse,
  TripleResponse,
  UpdateVideoFavoritesResponse,
  VideoInteractionState,
  WatchHistoryPage,
  WatchProgressSaveResponse,
} from '../types/interaction'

export function createRequestId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `req_${Date.now()}_${Math.random().toString(36).slice(2, 12)}`
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') {
      query.set(k, String(v))
    }
  }
  const str = query.toString()
  return str ? `?${str}` : ''
}

export const interactionApi = {
  getVideoInteraction(bvid: string): Promise<VideoInteractionState> {
    return apiRequest<VideoInteractionState>(
      `/api/v1/videos/${encodeURIComponent(bvid)}/interaction`,
      { method: 'GET', auth: true },
    )
  },

  likeVideo(bvid: string): Promise<LikeResponse> {
    return apiRequest<LikeResponse>(`/api/v1/videos/${encodeURIComponent(bvid)}/like`, {
      method: 'POST',
      auth: true,
    })
  },

  unlikeVideo(bvid: string): Promise<LikeResponse> {
    return apiRequest<LikeResponse>(`/api/v1/videos/${encodeURIComponent(bvid)}/like`, {
      method: 'DELETE',
      auth: true,
    })
  },

  coinVideo(bvid: string, count: 1 | 2, requestId = createRequestId()): Promise<CoinResponse> {
    return apiRequest<CoinResponse>(`/api/v1/videos/${encodeURIComponent(bvid)}/coin`, {
      method: 'POST',
      body: { count, request_id: requestId },
      auth: true,
    })
  },

  tripleVideo(bvid: string, requestId = createRequestId()): Promise<TripleResponse> {
    return apiRequest<TripleResponse>(`/api/v1/videos/${encodeURIComponent(bvid)}/triple`, {
      method: 'POST',
      body: { request_id: requestId },
      auth: true,
    })
  },

  shareVideo(bvid: string): Promise<CountedResponse> {
    return apiRequest<CountedResponse>(`/api/v1/videos/${encodeURIComponent(bvid)}/share`, {
      method: 'POST',
      auth: true,
    })
  },

  reportPlay(bvid: string): Promise<CountedResponse> {
    return apiRequest<CountedResponse>(`/api/v1/videos/${encodeURIComponent(bvid)}/play`, {
      method: 'POST',
      auth: true,
    })
  },

  reportProgress(
    bvid: string,
    partId: string,
    progressSec: number,
  ): Promise<WatchProgressSaveResponse> {
    return apiRequest<WatchProgressSaveResponse>(
      `/api/v1/videos/${encodeURIComponent(bvid)}/progress`,
      {
        method: 'POST',
        body: { part_id: partId, progress_sec: Math.max(0, Math.floor(progressSec)) },
        auth: true,
      },
    )
  },

  listFavoriteFolders(userId?: string): Promise<FavoriteFolder[]> {
    return apiRequest<FavoriteFolder[]>(
      `/api/v1/favorite-folders${buildQuery({ user_id: userId })}`,
      { method: 'GET', auth: true },
    )
  },

  createFavoriteFolder(title: string, isPublic: boolean): Promise<FavoriteFolder> {
    return apiRequest<FavoriteFolder>('/api/v1/favorite-folders', {
      method: 'POST',
      body: { title, is_public: isPublic },
      auth: true,
    })
  },

  updateFavoriteFolder(id: string, title: string, isPublic: boolean): Promise<FavoriteFolder> {
    return apiRequest<FavoriteFolder>(`/api/v1/favorite-folders/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: { title, is_public: isPublic },
      auth: true,
    })
  },

  deleteFavoriteFolder(id: string): Promise<null> {
    return apiRequest<null>(`/api/v1/favorite-folders/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      auth: true,
    })
  },

  listFavoriteFolderVideos(
    folderId: string,
    params: { page?: number; page_size?: number } = {},
  ): Promise<FavoriteFolderVideosResponse> {
    return apiRequest<FavoriteFolderVideosResponse>(
      `/api/v1/favorite-folders/${encodeURIComponent(folderId)}/videos${buildQuery(params)}`,
      { method: 'GET', auth: true },
    )
  },

  updateVideoFavorites(bvid: string, folderIds: string[]): Promise<UpdateVideoFavoritesResponse> {
    return apiRequest<UpdateVideoFavoritesResponse>(
      `/api/v1/videos/${encodeURIComponent(bvid)}/favorites`,
      {
        method: 'PUT',
        body: { folder_ids: folderIds },
        auth: true,
      },
    )
  },

  listWatchHistory(
    params: { keyword?: string; page?: number; page_size?: number } = {},
  ): Promise<WatchHistoryPage> {
    return apiRequest<WatchHistoryPage>(`/api/v1/history${buildQuery(params)}`, {
      method: 'GET',
      auth: true,
    })
  },

  deleteWatchHistoryItem(bvid: string): Promise<null> {
    return apiRequest<null>(`/api/v1/history/${encodeURIComponent(bvid)}`, {
      method: 'DELETE',
      auth: true,
    })
  },

  clearWatchHistory(): Promise<null> {
    return apiRequest<null>('/api/v1/history', {
      method: 'DELETE',
      auth: true,
    })
  },

  getCheckinStatus(): Promise<CheckinStatus> {
    return apiRequest<CheckinStatus>('/api/v1/users/me/checkin', {
      method: 'GET',
      auth: true,
    })
  },

  checkin(): Promise<CheckinResponse> {
    return apiRequest<CheckinResponse>('/api/v1/users/me/checkin', {
      method: 'POST',
      auth: true,
    })
  },
}
