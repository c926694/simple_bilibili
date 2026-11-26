import { sha256 } from 'js-sha256'

const HASH_CHUNK_SIZE = 4 * 1024 * 1024

interface HashRequestMessage {
  file: File
}

self.onmessage = async (event: MessageEvent<HashRequestMessage>) => {
  const { file } = event.data
  try {
    const hasher = sha256.create()
    let offset = 0
    while (offset < file.size) {
      const end = Math.min(offset + HASH_CHUNK_SIZE, file.size)
      const buffer = await file.slice(offset, end).arrayBuffer()
      hasher.update(new Uint8Array(buffer))
      offset = end
      self.postMessage({
        type: 'progress',
        percent: Math.min(100, Math.round((offset / file.size) * 100)),
      })
    }
    self.postMessage({
      type: 'done',
      hash: hasher.hex(),
    })
  } catch (err) {
    self.postMessage({
      type: 'error',
      message: err instanceof Error ? err.message : '计算文件哈希失败',
    })
  }
}
