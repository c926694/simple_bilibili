import { useEffect, useState } from 'react'

interface HealthResponse {
  code: number
  data: {
    status: string
  } | null
  msg: string
}

export default function App() {
  const [healthStatus, setHealthStatus] = useState<string>('检查中')

  useEffect(() => {
    fetch('/healthz')
      .then((res) => res.json() as Promise<HealthResponse>)
      .then((body) => {
        if (body.code === 0 && body.data) {
          setHealthStatus(body.data.status)
          return
        }
        setHealthStatus(`异常 (${body.code}: ${body.msg})`)
      })
      .catch(() => {
        setHealthStatus('后端连接失败')
      })
  }, [])

  return (
    <main style={{ fontFamily: 'sans-serif', padding: '32px' }}>
      <h1>simple_bilibili</h1>
      <p>后端健康状态：{healthStatus}</p>
    </main>
  )
}
