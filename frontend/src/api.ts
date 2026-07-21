import type {
	ConflictPolicy,
	Device,
	Diagnostics,
	LocalFile,
	PairingDecision,
	PairingRequest,
	PrivacyPolicy,
	Settings,
	Transfer,
	TransferEvent,
	TrustedDevice,
	UploadProgress,
} from './types'

const API = '/api/v1'

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
	self: () => request<Device>(`${API}/device/self`),
	devices: () => request<Device[]>(`${API}/devices`),
	privacyPolicy: () => request<PrivacyPolicy>(`${API}/privacy-policy`),
	acceptPrivacyPolicy: (version: string) => request<PrivacyPolicy>(`${API}/privacy-policy/accept`, {
		method: 'POST',
		body: JSON.stringify({ version }),
	}),
	trustedDevices: () => request<TrustedDevice[]>(`${API}/pairing/trusted-devices`),
	pairingRequests: () => request<PairingRequest[]>(`${API}/pairing/requests`),
	transfers: () => request<Transfer[]>(`${API}/transfers`),
	settings: () => request<Settings>(`${API}/settings`),
	updateSettings: (settings: Settings) => request<Settings>(`${API}/settings`, { method: 'PUT', body: JSON.stringify(settings) }),
	resetSettings: () => request<Settings>(`${API}/settings/reset`, { method: 'POST' }),
	refreshDevices: () => request<{ status: string }>(`${API}/discovery/refresh`, { method: 'POST' }),
  action: (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry' | 'reject') =>
		request<Transfer>(`${API}/transfers/${encodeURIComponent(id)}/${action}`, { method: 'POST' }),
  accept: (id: string, destinationPath: string, conflictPolicy: ConflictPolicy) =>
		request<Transfer>(`${API}/transfers/${encodeURIComponent(id)}/accept`, {
      method: 'POST', body: JSON.stringify({ destinationPath, conflictPolicy }),
    }),
	clearHistory: () => request<void>(`${API}/transfers/history`, { method: 'DELETE' }),
	diagnostics: () => request<Diagnostics>(`${API}/diagnostics`),
	runHealthCheck: () => request<Diagnostics['health']>(`${API}/health`),
	sendTestTransfer: () => request<Transfer>(`${API}/diagnostics/test-transfer`, { method: 'POST' }),
	simulateFailedTransfer: () => request<{ status: string }>(`${API}/diagnostics/simulate-failure`, { method: 'POST' }),
	clearTestData: () => request<{ status: string }>(`${API}/diagnostics/clear`, { method: 'POST' }),
	requestPairing: (deviceId: string) => request<PairingRequest>(`${API}/pairing/request`, { method: 'POST', body: JSON.stringify({ deviceId }) }),
	confirmPairing: (requestId: string) => request<PairingDecision>(`${API}/pairing/accept`, { method: 'POST', body: JSON.stringify({ requestId }) }),
	refreshPairing: (requestId: string) => request<PairingDecision>(`${API}/pairing/requests/${encodeURIComponent(requestId)}`),
	rejectPairing: (requestId: string) => request<PairingRequest>(`${API}/pairing/reject`, { method: 'POST', body: JSON.stringify({ requestId }) }),
	forgetDevice: (deviceId: string) => request<void>(`${API}/pairing/trusted-devices/${encodeURIComponent(deviceId)}`, { method: 'DELETE' }),
	setDeviceBlocked: (deviceId: string, blocked: boolean) => request<TrustedDevice>(`${API}/pairing/trusted-devices/${encodeURIComponent(deviceId)}/${blocked ? 'block' : 'unblock'}`, { method: 'POST' }),
	createStage: () => request<{ id: string; createdAt: string }>(`${API}/transfers/staging`, { method: 'POST' }),
	deleteStage: (id: string) => request<void>(`${API}/transfers/staging/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  queueStage: (id: string, deviceId: string, roots: string[], conflictPolicy: ConflictPolicy) =>
		request<Transfer>(`${API}/transfers/staging/${encodeURIComponent(id)}/queue`, {
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
		xhr.open('PUT', `${API}/transfers/staging/${encodeURIComponent(stageId)}/files?path=${encodeURIComponent(item.relativePath)}`)
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
		socket = new WebSocket(`${protocol}//${window.location.host}${API}/ws/transfers`)
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
