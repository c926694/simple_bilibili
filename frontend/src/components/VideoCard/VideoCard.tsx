import { Link } from 'react-router-dom'
import type { VideoSummary } from '../../types/video'
import { formatCount, formatDateShort, formatDuration } from '../../utils/format'
import './VideoCard.css'

interface VideoCardProps {
  video: VideoSummary
}

export function VideoCard({ video }: VideoCardProps) {
  const publishDate = formatDateShort(video.published_at || video.created_at)

  return (
    <article className="bili-video-card">
      <Link to={`/video/${video.bv_id}`} className="bili-video-card__cover-link">
        <div className="bili-video-card__cover">
          {video.cover_url ? (
            <img src={video.cover_url} alt={video.title} loading="lazy" />
          ) : (
            <div className="bili-video-card__cover-placeholder">
              <svg width="32" height="32" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <rect x="3" y="5" width="18" height="14" rx="3" stroke="currentColor" strokeWidth="1.6" />
                <path d="M10 9.5L15 12L10 14.5V9.5Z" fill="currentColor" />
              </svg>
            </div>
          )}
          <div className="bili-video-card__overlay">
            <div className="bili-video-card__stats">
              <span className="bili-video-card__stat">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <rect x="3" y="5" width="18" height="14" rx="3" stroke="currentColor" strokeWidth="1.8" />
                  <path d="M10 9.5L15 12L10 14.5V9.5Z" fill="currentColor" />
                </svg>
                {formatCount(video.stat?.play_count ?? 0)}
              </span>
              <span className="bili-video-card__stat">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <rect x="3" y="5" width="18" height="14" rx="3" stroke="currentColor" strokeWidth="1.8" />
                  <path d="M7 10h6M7 14h10" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
                </svg>
                {formatCount(video.stat?.danmaku_count ?? 0)}
              </span>
            </div>
            <span className="bili-video-card__duration">{formatDuration(video.duration_sec)}</span>
          </div>
        </div>
      </Link>

      <Link to={`/video/${video.bv_id}`} className="bili-video-card__title" title={video.title}>
        {video.title}
      </Link>

      <div className="bili-video-card__meta">
        <Link to={`/space/${video.author.id}`} className="bili-video-card__author">
          <span className="bili-video-card__up-badge">UP</span>
          <span className="bili-video-card__author-name">{video.author.nickname}</span>
        </Link>
        {publishDate && <span className="bili-video-card__date">· {publishDate}</span>}
      </div>
    </article>
  )
}
