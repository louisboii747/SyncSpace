import type { LocalFile, Transfer, TransferStatus } from './types'

const terminalStatuses = new Set<TransferStatus>(['Completed', 'Cancelled'])
const activeStatuses = new Set<TransferStatus>(['Preparing', 'Connecting', 'Negotiating', 'Sending', 'Receiving', 'Resuming', 'Verifying'])

export function isTerminal(transfer: Transfer): boolean {
  return terminalStatuses.has(transfer.status)
}

export function isActive(transfer: Transfer): boolean {
  return activeStatuses.has(transfer.status)
}

export function formatBytes(value: number, decimals = 1): string {
  if (!Number.isFinite(value) || value < 0) return 'Unavailable'
  if (value === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const unit = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / 1024 ** unit).toFixed(unit === 0 ? 0 : decimals)} ${units[unit]}`
}

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return 'Calculating…'
  if (seconds < 60) return `${Math.ceil(seconds)} sec`
  if (seconds < 3600) return `${Math.ceil(seconds / 60)} min`
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.ceil((seconds % 3600) / 60)
  return `${hours}h ${minutes}m`
}

export function progressPercent(transfer: Transfer): number {
  if (transfer.status === 'Completed') return 100
  if (!transfer.size) return 0
  return Math.max(0, Math.min(100, (transfer.progress / transfer.size) * 100))
}

export function topLevelRoots(files: LocalFile[]): string[] {
  return [...new Set(files.map(({ relativePath }) => relativePath.replaceAll('\\', '/').split('/')[0]).filter(Boolean))]
}

export function cleanRelativePath(path: string): string {
  return path.replaceAll('\\', '/').split('/').filter((part) => part && part !== '.' && part !== '..').join('/')
}

export function statusTone(status: TransferStatus): 'blue' | 'green' | 'amber' | 'red' | 'muted' {
  if (status === 'Completed') return 'green'
  if (status === 'Failed') return 'red'
  if (status === 'Paused' || status === 'Verifying') return 'amber'
  if (status === 'Cancelled') return 'muted'
  return 'blue'
}
