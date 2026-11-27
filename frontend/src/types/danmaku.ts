export type DanmakuMode = 1 | 2 | 3
export type DanmakuFontSize = 18 | 25 | 36

export interface DanmakuItem {
  id: string
  video_id: string
  part_id: string
  user_id: string
  content: string
  time_offset_ms: number
  mode: DanmakuMode
  font_size: DanmakuFontSize
  color: string
  created_at: string
  isSelf?: boolean
}

export interface DanmakuSegmentResponse {
  segment: number
  segment_duration_ms: number
  online_count: number
  items: DanmakuItem[]
}

export interface SendDanmakuRequest {
  part_id: string
  content: string
  time_offset_ms: number
  mode: DanmakuMode
  font_size: DanmakuFontSize
  color: string
}

export interface DanmakuStyleConfig {
  mode: DanmakuMode
  fontSize: DanmakuFontSize
  color: string
  opacity: number
}

export type VideoRoomWsMessage =
  | { type: 'online_count'; data: { count: number } }
  | { type: 'danmaku'; data: DanmakuItem }
  | { type: 'pong'; data: Record<string, never> }
