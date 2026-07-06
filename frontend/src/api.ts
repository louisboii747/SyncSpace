import type { ConflictPolicy, Device, Diagnostics, LocalFile, Transfer, TransferEvent, TrustedDevice, UploadProgress } from './types'

export class APIError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options?.headers },
  })
  if (!response.ok) {
    let message = `Request failed (${response.status})`
    try {
      const body = await response.json() as { error?: string }
      if (body.error) message = body.error
    } catch { /* response was not JSON */ }
    throw new APIError(message, response.status)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  devices: () => request<Device[]>('/devices'),
  trustedDevices: () => request<TrustedDevice[]>('/pairing/trusted-devices'),
  transfers: () => request<Transfer[]>('/transfers'),
  refreshDevices: () => request<{ status: string }>('/discovery/refresh', { method: 'POST' }),
  action: (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry' | 'reject') =>
    request<Transfer>(`/transfers/${encodeURIComponent(id)}/${action}`, { method: 'POST' }),
  accept: (id: string, destinationPath: string, conflictPolicy: ConflictPolicy) =>
    request<Transfer>(`/transfers/${encodeURIComponent(id)}/accept`, {
      method: 'POST', body: JSON.stringify({ destinationPath, conflictPolicy }),
    }),
  clearHistory: () => request<void>('/transfers/history', { method: 'DELETE' }),
  diagnostics: () => request<Diagnostics>('/diagnostics'),
  runHealthCheck: () => request<Diagnostics['health']>('/health'),
  sendTestTransfer: () => request<Transfer>('/diagnostics/test-transfer', { method: 'POST' }),
  simulateFailedTransfer: () => request<{ status: string }>('/diagnostics/simulate-failure', { method: 'POST' }),
  clearTestData: () => request<{ status: string }>('/diagnostics/clear', { method: 'POST' }),
  trustDevice: async (deviceId: string) => {
    const pairing = await request<{ requestId: string }>('/pairing/request', {
      method: 'POST', body: JSON.stringify({ deviceId }),
    })
    return request<TrustedDevice>('/pairing/accept', {
      method: 'POST', body: JSON.stringify({ requestId: pairing.requestId }),
    })
  },
  createStage: () => request<{ id: string; createdAt: string }>('/transfers/staging', { method: 'POST' }),
  deleteStage: (id: string) => request<void>(`/transfers/staging/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  queueStage: (id: string, deviceId: string, roots: string[], conflictPolicy: ConflictPolicy) =>
    request<Transfer>(`/transfers/staging/${encodeURIComponent(id)}/queue`, {
      method: 'POST', body: JSON.stringify({ deviceId, roots, conflictPolicy }),
    }),
}

export async function stageAndQueue(
  files: LocalFile[],
  deviceId: string,
  conflictPolicy: ConflictPolicy,
  onProgress: (progress: UploadProgress) => void,
): Promise<Transfer> {
  if (!files.length) throw new Error('Choose at least one file')
  const stage = await api.createStage()
  const totalBytes = files.reduce((sum, item) => sum + item.file.size, 0)
  let completedBytes = 0
  try {
    for (let index = 0; index < files.length; index += 1) {
      const item = files[index]
      await uploadFile(stage.id, item, (loaded) => {
        onProgress({
          active: true,
          completedBytes: completedBytes + loaded,
          totalBytes,
          currentName: item.relativePath,
          completedFiles: index,
          totalFiles: files.length,
        })
      })
      completedBytes += item.file.size
    }
    const roots = [...new Set(files.map((item) => item.relativePath.split('/')[0]))]
    return await api.queueStage(stage.id, deviceId, roots, conflictPolicy)
  } catch (error) {
    await api.deleteStage(stage.id).catch(() => undefined)
    throw error
  } finally {
    onProgress({ active: false, completedBytes, totalBytes, currentName: '', completedFiles: files.length, totalFiles: files.length })
  }
}

function uploadFile(stageId: string, item: LocalFile, onProgress: (loaded: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', `/transfers/staging/${encodeURIComponent(stageId)}/files?path=${encodeURIComponent(item.relativePath)}`)
    xhr.setRequestHeader('Content-Type', item.file.type || 'application/octet-stream')
    xhr.upload.onprogress = (event) => onProgress(event.loaded)
    xhr.onerror = () => reject(new Error('The local upload connection was interrupted'))
    xhr.onabort = () => reject(new Error('Upload cancelled'))
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve()
        return
      }
      let message = `Upload failed (${xhr.status})`
      try { message = (JSON.parse(xhr.responseText) as { error?: string }).error || message } catch { /* not JSON */ }
      reject(new APIError(message, xhr.status))
    }
    xhr.send(item.file)
  })
}

export function connectTransferEvents(
  onEvent: (event: TransferEvent) => void,
  onState: (connected: boolean) => void,
): () => void {
  let socket: WebSocket | undefined
  let retry: number | undefined
  let closed = false
  const connect = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new WebSocket(`${protocol}//${window.location.host}/ws/transfers`)
    socket.onopen = () => onState(true)
    socket.onmessage = (message) => {
      try { onEvent(JSON.parse(message.data as string) as TransferEvent) } catch (error) { console.warn('SyncSpace ignored a malformed transfer event', error) }
    }
    socket.onclose = () => {
      onState(false)
      if (!closed) retry = window.setTimeout(connect, 1500)
    }
    socket.onerror = () => {
      console.warn('SyncSpace transfer event connection failed; retrying')
      socket?.close()
    }
  }
  connect()
  return () => {
    closed = true
    if (retry) window.clearTimeout(retry)
    socket?.close()
  }
}
