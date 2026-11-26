import { useEffect, useState } from 'react'

export const PRESET_AVATARS = [
  { label: '小电视粉', url: '/avatars/pink-tv.svg' },
  { label: '哔哩蓝', url: '/avatars/blue-tv.svg' },
  { label: '晴空橙', url: '/avatars/orange-sun.svg' },
  { label: '薄荷绿', url: '/avatars/mint-tv.svg' },
  { label: '星云紫', url: '/avatars/purple-star.svg' },
]

interface UserAvatarProps {
  src?: string
  nickname: string
  size?: number
  className?: string
}

export function UserAvatar({ src, nickname, size = 38, className = '' }: UserAvatarProps) {
  const [imgFailed, setImgFailed] = useState(false)

  useEffect(() => {
    setImgFailed(false)
  }, [src])

  const initial = (nickname || 'B').trim().charAt(0).toUpperCase()

  if (src && !imgFailed) {
    return (
      <img
        src={src}
        alt={nickname}
        width={size}
        height={size}
        className={className}
        onError={() => setImgFailed(true)}
        style={{
          width: size,
          height: size,
          borderRadius: '50%',
          objectFit: 'cover',
          display: 'block',
          flexShrink: 0,
        }}
      />
    )
  }

  return (
    <span
      className={className}
      aria-label={nickname}
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        background: 'linear-gradient(135deg, #00aeec 0%, #fb7299 100%)',
        color: '#ffffff',
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontWeight: 600,
        fontSize: Math.max(13, Math.round(size * 0.42)),
        flexShrink: 0,
        userSelect: 'none',
      }}
    >
      {initial}
    </span>
  )
}
