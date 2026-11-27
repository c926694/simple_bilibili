import { apiRequest } from './client'
import type {
  Category,
  CreateVideoRequest,
  PagedList,
  UpdateVideoRequest,
  VideoDetail,
  VideoSearchResponse,
  VideoSortOption,
  VideoSummary,
} from '../types/video'

function buildQuery(params: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  }
  const str = query.toString()
  return str ? `?${str}` : ''
}

export const videoApi = {
  listCategories(): Promise<Category[]> {
    return apiRequest<Category[]>('/api/v1/categories', {
      method: 'GET',
      auth: false,
    })
  },

  listVideos(params: {
    category_id?: number
    sort?: VideoSortOption
    page?: number
    page_size?: number
  } = {}): Promise<PagedList<VideoSummary>> {
    return apiRequest<PagedList<VideoSummary>>(`/api/v1/videos${buildQuery(params)}`, {
      method: 'GET',
      auth: false,
    })
  },

  searchVideos(params: {
    keyword: string
    category_id?: number
    sort?: VideoSortOption
    page?: number
    page_size?: number
  }): Promise<VideoSearchResponse> {
    return apiRequest<VideoSearchResponse>(`/api/v1/videos/search${buildQuery(params)}`, {
      method: 'GET',
      auth: false,
    })
  },

  getVideoDetail(bvid: string): Promise<VideoDetail> {
    return apiRequest<VideoDetail>(`/api/v1/videos/${encodeURIComponent(bvid)}`, {
      method: 'GET',
      auth: true,
    })
  },

  listUserVideos(
    userId: string,
    params: { sort?: VideoSortOption; page?: number; page_size?: number } = {},
  ): Promise<PagedList<VideoSummary>> {
    return apiRequest<PagedList<VideoSummary>>(
      `/api/v1/users/${encodeURIComponent(userId)}/videos${buildQuery(params)}`,
      {
        method: 'GET',
        auth: false,
      },
    )
  },

  createVideo(req: CreateVideoRequest): Promise<VideoDetail> {
    return apiRequest<VideoDetail>('/api/v1/videos', {
      method: 'POST',
      body: req,
      auth: true,
    })
  },

  listMyVideos(params: {
    status?: number | string
    page?: number
    page_size?: number
  } = {}): Promise<PagedList<VideoDetail>> {
    return apiRequest<PagedList<VideoDetail>>(`/api/v1/videos/mine${buildQuery(params)}`, {
      method: 'GET',
      auth: true,
    })
  },

  updateVideo(bvid: string, req: UpdateVideoRequest): Promise<VideoDetail> {
    return apiRequest<VideoDetail>(`/api/v1/videos/${encodeURIComponent(bvid)}`, {
      method: 'PUT',
      body: req,
      auth: true,
    })
  },

  deleteVideo(bvid: string): Promise<null> {
    return apiRequest<null>(`/api/v1/videos/${encodeURIComponent(bvid)}`, {
      method: 'DELETE',
      auth: true,
    })
  },

  retryPartTranscode(bvid: string, partId: string): Promise<VideoDetail | null> {
    return apiRequest<VideoDetail | null>(
      `/api/v1/videos/${encodeURIComponent(bvid)}/parts/${encodeURIComponent(partId)}/retry-transcode`,
      {
        method: 'POST',
        auth: true,
      },
    )
  },
}
