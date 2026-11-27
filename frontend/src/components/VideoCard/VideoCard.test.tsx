import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { VideoCard } from './VideoCard'
import { formatCount, formatDuration, getTranscodeStatusLabel, getVideoStatusMeta } from '../../utils/format'

describe('video utils & VideoCard', () => {
  it('formats duration and count', () => {
    expect(formatDuration(0)).toBe('00:00')
    expect(formatDuration(75)).toBe('01:15')
    expect(formatDuration(3665)).toBe('1:01:05')
    expect(formatCount(999)).toBe('999')
    expect(formatCount(12500)).toBe('1.3万')
    expect(getVideoStatusMeta(3).label).toBe('已上传完成')
    expect(getTranscodeStatusLabel(2)).toBe('转码完成')
  })

  it('renders VideoCard with links and stats', () => {
    render(
      <MemoryRouter>
        <VideoCard
          video={{
            id: '1001',
            bv_id: 'BV1TestVideo01',
            title: 'Go 与 FFmpeg 转码实战',
            description: '示例简介',
            cover_url: '/storage/covers/1/test.jpg',
            duration_sec: 125,
            status: 3,
            published_at: '2026-11-27T10:00:00Z',
            created_at: '2026-11-27T09:58:00Z',
            category: { id: 6, name: '科技' },
            author: {
              id: '2001',
              username: 'tester',
              nickname: '测试UP主',
              avatar_url: '/avatars/default.webp',
            },
            stat: {
              play_count: 15000,
              danmaku_count: 320,
              like_count: 500,
              coin_count: 120,
              favorite_count: 80,
              share_count: 10,
              comment_count: 25,
            },
          }}
        />
      </MemoryRouter>,
    )

    expect(screen.getByText('Go 与 FFmpeg 转码实战')).toBeInTheDocument()
    expect(screen.getByText('测试UP主')).toBeInTheDocument()
    expect(screen.getByText('02:05')).toBeInTheDocument()
    expect(screen.getByText('1.5万')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Go 与 FFmpeg 转码实战' })).toHaveAttribute(
      'href',
      '/video/BV1TestVideo01',
    )
  })
})
