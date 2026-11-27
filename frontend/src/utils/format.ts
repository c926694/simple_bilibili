export function formatDuration(seconds: number): string {
  const safe = Math.max(0, Math.floor(seconds || 0))
  const hours = Math.floor(safe / 3600)
  const mins = Math.floor((safe % 3600) / 60)
  const secs = safe % 60
  const pad = (n: number) => String(n).padStart(2, '0')
  if (hours > 0) {
    return `${hours}:${pad(mins)}:${pad(secs)}`
  }
  return `${pad(mins)}:${pad(secs)}`
}

export function formatCount(count: number): string {
  const value = Math.max(0, Number(count || 0))
  if (value >= 100000000) {
    return `${(value / 100000000).toFixed(1)}亿`
  }
  if (value >= 10000) {
    return `${(value / 10000).toFixed(1)}万`
  }
  return String(value)
}

export function formatDateShort(iso?: string | null): string {
  if (!iso) {
    return ''
  }
  return iso.slice(0, 10)
}

export function getVideoStatusMeta(status: number): {
  label: string
  tone: 'blue' | 'green' | 'red' | 'gray'
} {
  switch (status) {
    case 1:
      return { label: '转码中', tone: 'blue' }
    case 2:
      return { label: '审核中', tone: 'blue' }
    case 3:
      return { label: '已发布', tone: 'green' }
    case 4:
      return { label: '转码失败', tone: 'red' }
    case 5:
      return { label: '已下架', tone: 'gray' }
    default:
      return { label: '草稿', tone: 'gray' }
  }
}

export function getTranscodeStatusLabel(status: number): string {
  switch (status) {
    case 0:
      return '待转码'
    case 1:
      return '转码中'
    case 2:
      return '转码完成'
    case 3:
      return '转码失败'
    default:
      return '未知状态'
  }
}
