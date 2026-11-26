import { useEffect, useState } from 'react'

export const DEFAULT_AVATAR_URL = '/avatars/default.webp'

export const PRESET_AVATARS = [
  { label: '默认头像', url: DEFAULT_AVATAR_URL },
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

  const resolvedSrc = !imgFailed && src ? src : DEFAULT_AVATAR_URL

  return (
    <img
      src={resolvedSrc}
      alt={nickname}
      width={size}
      height={size}
      className={className}
      onError={() => {
        if (!imgFailed && src && src !== DEFAULT_AVATAR_URL) {
          setImgFailed(true)
        }
      }}
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
