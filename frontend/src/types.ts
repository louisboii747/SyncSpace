export type TransferStatus =
  | 'Queued' | 'Preparing' | 'Connecting' | 'Negotiating' | 'Sending'
  | 'Receiving' | 'Paused' | 'Resuming' | 'Completed' | 'Cancelled'
  | 'Failed' | 'Verifying'

export type ConflictPolicy = 'prompt' | 'overwrite' | 'rename'

export interface Device {
  deviceId: string
  deviceName: string
  deviceType: string
  platform: string
  localIp: string
  port: number
  appVersion: string
  lastSeen: string
  online: boolean
  connectionState: 'online' | 'offline'
  availableStorage: number
  transferCapability: boolean
  supportedProtocolVersion: number
  maximumChunkSize: number
  compressionSupport: boolean
}

export interface TrustedDevice {
  deviceId: string
  deviceName: string
  platform: string
  pairedAt: string
  lastSeen: string
  trustState: 'trusted'
}

export interface TransferFile {
  fileId: string
  relativePath: string
  directory: boolean
  size: number
  checksum: string
  chunkSize: number
  chunkCount: number
}

export interface Transfer {
  uuid: string
  direction: 'outbound' | 'inbound'
  deviceId: string
  deviceName?: string
  filename: string
  path?: string
  files?: TransferFile[]
  size: number
  checksum?: string
  status: TransferStatus
  progress: number
  speed: number
  etaSeconds: number
  startTime?: string
  finishTime?: string
  createdAt: string
  updatedAt: string
  error?: string
  attempts: number
  approved: boolean
  approvalRequired: boolean
  conflictPolicy: ConflictPolicy
  compression: boolean
  protocolVersion: number
}

export interface TransferEvent {
  type: 'QueueUpdated' | 'Progress' | 'Pause' | 'Resume' | 'Complete' | 'Failure' | 'Verification'
  transfer: Transfer
  timestamp: string
}

export interface LocalFile {
  file: File
  relativePath: string
}

export interface UploadProgress {
  active: boolean
  completedBytes: number
  totalBytes: number
  currentName: string
  completedFiles: number
  totalFiles: number
}

export interface DiagnosticCheck { ok: boolean; detail?: string }
export interface DiagnosticLog { time: string; level: string; message: string; attributes?: Record<string, string> }
export interface Diagnostics {
  health: { status: 'ok' | 'degraded'; checks: Record<string, DiagnosticCheck>; checkedAt: string }
  localDevice: Device
  platform: string
  backendUrl: string
  webSockets: { discovery: number; pairing: number; transfers: number }
  discoveryRunning: boolean
  knownDevices: Device[]
  trustedDevices: TrustedDevice[]
  activeTransfers: Transfer[]
  transferQueue: Transfer[]
  storagePath: string
  databasePath: string
  protocolVersion: number
  developerMode: boolean
  logs: DiagnosticLog[]
  lastErrors: DiagnosticLog[]
}

export type View = 'transfers' | 'devices' | 'history' | 'diagnostics'
