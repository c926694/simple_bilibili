import { apiRequest } from './client'
import type {
  DanmakuItem,
  DanmakuSegmentResponse,
  SendDanmakuRequest,
} from '../types/danmaku'

export const DANMAKU_SEGMENT_MS = 300000

export const danmakuApi = {
  listSegment(bvid: string, partId: string, segment = 0): Promise<DanmakuSegmentResponse> {
    const query = new URLSearchParams({
      part_id: partId,
      segment: String(Math.max(0, segment)),
    })
    return apiRequest<DanmakuSegmentResponse>(
      `/api/v1/videos/${encodeURIComponent(bvid)}/danmaku?${query.toString()}`,
      {
        method: 'GET',
        auth: false,
      },
    )
  },

  send(bvid: string, req: SendDanmakuRequest): Promise<DanmakuItem> {
    return apiRequest<DanmakuItem>(`/api/v1/videos/${encodeURIComponent(bvid)}/danmaku`, {
      method: 'POST',
      body: req,
      auth: true,
    })
  },
}

export function buildVideoRoomWsUrl(bvid: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/ws/videos/${encodeURIComponent(bvid)}`
}
