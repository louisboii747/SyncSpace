import { useEffect, useMemo, useRef, useState } from 'react'
import { api, stageAndQueue } from './api'
import { Icon } from './components/Icon'
import { TransferCard } from './components/TransferCard'
import { useSyncSpace } from './hooks/useSyncSpace'
import type { ConflictPolicy, Device, LocalFile, Transfer, UploadProgress, View } from './types'
import { cleanRelativePath, formatBytes, isActive } from './utils'

const emptyUpload: UploadProgress = { active: false, completedBytes: 0, totalBytes: 0, currentName: '', completedFiles: 0, totalFiles: 0 }

export default function App() {
  const sync = useSyncSpace()
  const [view, setView] = useState<View>('transfers')
  const [selectedDevice, setSelectedDevice] = useState('')
  const [conflictPolicy, setConflictPolicy] = useState<ConflictPolicy>('rename')
  const [dragging, setDragging] = useState(false)
  const [upload, setUpload] = useState<UploadProgress>(emptyUpload)
  const [pairTarget, setPairTarget] = useState<Device | null>(null)
  const [pairing, setPairing] = useState(false)
  const fileInput = useRef<HTMLInputElement>(null)
  const folderInput = useRef<HTMLInputElement>(null)
  const dragDepth = useRef(0)

  const capableDevices = useMemo(() => sync.devices.filter((device) => device.online && device.transferCapability && device.supportedProtocolVersion === 1), [sync.devices])
  const selected = capableDevices.find((device) => device.deviceId === selectedDevice)
  useEffect(() => {
    if (!selectedDevice) {
      const preferred = capableDevices.find((device) => sync.trustedIDs.has(device.deviceId)) || capableDevices[0]
      if (preferred) setSelectedDevice(preferred.deviceId)
    }
  }, [capableDevices, selectedDevice, sync.trustedIDs])

  const onTransferChanged = (changed: Transfer) => sync.setTransfers((current) => {
    const index = current.findIndex((item) => item.uuid === changed.uuid)
    if (index < 0) return [changed, ...current]
    const copy = [...current]
    copy[index] = changed
    return copy
  })

  const sendFiles = async (files: LocalFile[]) => {
    if (!selected) {
      sync.toast('error', 'Choose a device', 'Pick an online SyncSpace device before adding files.')
      return
    }
    if (!sync.trustedIDs.has(selected.deviceId)) {
      setPairTarget(selected)
      return
    }
    if (!files.length) return
    try {
      const queued = await stageAndQueue(files, selected.deviceId, conflictPolicy, setUpload)
      onTransferChanged(queued)
      sync.toast('success', 'Added to queue', `${files.length} ${files.length === 1 ? 'item' : 'items'} ready for ${selected.deviceName}.`)
    } catch (error) {
      sync.toast('error', 'Couldn’t queue transfer', error instanceof Error ? error.message : 'Local staging failed')
    }
  }

  const pair = async () => {
    if (!pairTarget) return
    setPairing(true)
    try {
      const trusted = await api.trustDevice(pairTarget.deviceId)
      sync.setTrusted((current) => [...current.filter((item) => item.deviceId !== trusted.deviceId), trusted])
      sync.toast('success', 'Device trusted', `${pairTarget.deviceName} can now receive transfers from this device.`)
      setPairTarget(null)
    } catch (error) {
      sync.toast('error', 'Pairing failed', error instanceof Error ? error.message : 'Unable to trust device')
    } finally { setPairing(false) }
  }

  const enterDrag = (event: React.DragEvent) => {
    event.preventDefault()
    dragDepth.current += 1
    if (event.dataTransfer.types.includes('Files')) setDragging(true)
  }
  const leaveDrag = (event: React.DragEvent) => {
    event.preventDefault()
    dragDepth.current -= 1
    if (dragDepth.current <= 0) { dragDepth.current = 0; setDragging(false) }
  }
  const drop = async (event: React.DragEvent) => {
    event.preventDefault()
    dragDepth.current = 0
    setDragging(false)
    await sendFiles(await filesFromDrop(event.dataTransfer))
  }

  const activeCount = sync.active.filter(isActive).length
  const totalSpeed = sync.active.reduce((sum, item) => sum + item.speed, 0)
  return <div className="app-shell" onDragEnter={enterDrag} onDragLeave={leaveDrag} onDragOver={(event) => event.preventDefault()} onDrop={(event) => void drop(event)}>
    <aside className="sidebar">
      <div className="brand"><span className="brand-mark"><i/><i/><i/></span><span>SyncSpace</span></div>
      <nav aria-label="Main navigation">
        <NavButton icon="send" label="Transfers" active={view === 'transfers'} badge={sync.active.length || undefined} onClick={() => setView('transfers')} />
        <NavButton icon="devices" label="Devices" active={view === 'devices'} badge={capableDevices.length || undefined} onClick={() => setView('devices')} />
        <NavButton icon="history" label="History" active={view === 'history'} onClick={() => setView('history')} />
      </nav>
      <div className="sidebar-status">
        <div className="radar"><span/><span/><i /></div>
        <strong>{capableDevices.length} nearby</strong>
        <span>Local network only</span>
      </div>
      <div className="connection-row"><span className={sync.connected ? 'online' : ''}/>{sync.connected ? 'Live updates connected' : 'Reconnecting…'}</div>
    </aside>

    <main>
      <header className="topbar">
        <div><span className="eyebrow">PRIVATE · DIRECT · FAST</span><h1>{view === 'transfers' ? 'Move anything, beautifully.' : view === 'devices' ? 'Your nearby space.' : 'Everything that moved.'}</h1></div>
        <div className="top-actions">
          <button className="icon-button" aria-label="Enable notifications" onClick={() => { if ('Notification' in window) void Notification.requestPermission() }}><Icon name="bell" /></button>
          <button className="profile" aria-label="Local SyncSpace profile">LS</button>
        </div>
      </header>

      {sync.error && <div className="backend-error"><div><strong>SyncSpace backend is out of reach</strong><span>{sync.error}</span></div><button className="button ghost" onClick={() => void sync.reload()}>Try again</button></div>}
      {view === 'transfers' && <TransferView
        devices={capableDevices} trustedIDs={sync.trustedIDs} selectedDevice={selectedDevice} onSelect={setSelectedDevice}
        conflictPolicy={conflictPolicy} onConflictPolicy={setConflictPolicy} upload={upload}
        active={sync.active} activeCount={activeCount} totalSpeed={totalSpeed}
        onBrowseFiles={() => fileInput.current?.click()} onBrowseFolder={() => folderInput.current?.click()}
        onPair={setPairTarget} onChanged={onTransferChanged} onError={(message) => sync.toast('error', 'Transfer action failed', message)}
      />}
      {view === 'devices' && <DevicesView devices={sync.devices} trustedIDs={sync.trustedIDs} onSelect={(device) => { setSelectedDevice(device.deviceId); setView('transfers') }} onPair={setPairTarget} />}
      {view === 'history' && <HistoryView transfers={sync.history} onChanged={onTransferChanged} onError={(message) => sync.toast('error', 'History action failed', message)} onCleared={() => sync.setTransfers((current) => current.filter((item) => !['Completed', 'Cancelled'].includes(item.status)))} />}
    </main>

    <input ref={fileInput} className="visually-hidden" type="file" multiple onChange={(event) => { void sendFiles(filesFromList(event.currentTarget.files)); event.currentTarget.value = '' }} />
    <input ref={folderInput} className="visually-hidden" type="file" multiple {...{ webkitdirectory: '', directory: '' }} onChange={(event) => { void sendFiles(filesFromList(event.currentTarget.files)); event.currentTarget.value = '' }} />
    {dragging && <div className="drop-overlay"><div className="drop-orb"><Icon name="plus"/></div><h2>Drop into SyncSpace</h2><p>{selected ? `Send directly to ${selected.deviceName}` : 'Choose a nearby device first'}</p></div>}
    {pairTarget && <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setPairTarget(null) }}><section className="modal" role="dialog" aria-modal="true" aria-labelledby="pair-title">
      <button className="modal-close" onClick={() => setPairTarget(null)} aria-label="Close"><Icon name="close"/></button>
      <div className="modal-icon"><Icon name="shield"/></div><span className="eyebrow">EXPLICIT TRUST</span><h2 id="pair-title">Trust {pairTarget.deviceName}?</h2>
      <p>This local decision allows transfers to this discovered device. SyncSpace never trusts a device merely because it appeared on your network.</p>
      <div className="trust-details"><span>{pairTarget.platform}</span><span>{pairTarget.localIp}</span><span>Protocol v{pairTarget.supportedProtocolVersion}</span></div>
      <div className="modal-actions"><button className="button ghost" onClick={() => setPairTarget(null)}>Not now</button><button className="button primary" disabled={pairing} onClick={() => void pair()}><Icon name="shield"/>{pairing ? 'Trusting…' : 'Trust device'}</button></div>
    </section></div>}
    {upload.active && <div className="upload-float"><div className="upload-ring" style={{ '--progress': `${upload.totalBytes ? upload.completedBytes / upload.totalBytes * 360 : 0}deg` } as React.CSSProperties}><Icon name="arrowUp"/></div><div><strong>Preparing transfer</strong><span>{upload.currentName}</span><small>{upload.completedFiles + 1} of {upload.totalFiles} · {Math.round(upload.totalBytes ? upload.completedBytes / upload.totalBytes * 100 : 0)}%</small></div></div>}
    <div className="toast-stack" aria-live="polite">{sync.toasts.map((toast) => <div className={`toast ${toast.tone}`} key={toast.id}><span><Icon name={toast.tone === 'success' ? 'check' : toast.tone === 'error' ? 'close' : 'wifi'}/></span><div><strong>{toast.title}</strong><p>{toast.message}</p></div></div>)}</div>
    {sync.celebration > 0 && <Celebration key={sync.celebration} />}
  </div>
}

function NavButton({ icon, label, active, badge, onClick }: { icon: 'send' | 'devices' | 'history'; label: string; active: boolean; badge?: number; onClick: () => void }) {
  return <button className={active ? 'active' : ''} onClick={onClick}><Icon name={icon}/><span>{label}</span>{badge !== undefined && <em>{badge}</em>}</button>
}

interface TransferViewProps {
  devices: Device[]; trustedIDs: Set<string>; selectedDevice: string; onSelect: (id: string) => void
  conflictPolicy: ConflictPolicy; onConflictPolicy: (policy: ConflictPolicy) => void; upload: UploadProgress
  active: Transfer[]; activeCount: number; totalSpeed: number
  onBrowseFiles: () => void; onBrowseFolder: () => void; onPair: (device: Device) => void
  onChanged: (transfer: Transfer) => void; onError: (message: string) => void
}

function TransferView(props: TransferViewProps) {
  return <div className="page-grid">
    <section className="send-panel glass-panel">
      <div className="section-heading"><div><span className="section-kicker">CHOOSE A DESTINATION</span><h2>Nearby devices</h2></div><button className="text-button" onClick={() => void api.refreshDevices()}><Icon name="refresh"/>Refresh</button></div>
      <div className="device-strip">
        {props.devices.length ? props.devices.map((device) => <button key={device.deviceId} className={`device-orb-card ${props.selectedDevice === device.deviceId ? 'selected' : ''}`} onClick={() => props.onSelect(device.deviceId)}>
          <span className="device-orb"><PlatformGlyph platform={device.platform}/><i className="online-dot"/></span>
          <strong>{device.deviceName}</strong><small>{device.platform}</small>
          {!props.trustedIDs.has(device.deviceId) && <em onClick={(event) => { event.stopPropagation(); props.onPair(device) }}>Trust</em>}
        </button>) : <div className="empty-devices"><div className="radar small"><span/><span/><i/></div><strong>Looking for SyncSpace devices…</strong><span>Keep both devices on the same local network.</span></div>}
      </div>
      <div className="drop-zone" onClick={props.onBrowseFiles} role="button" tabIndex={0} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') props.onBrowseFiles() }}>
        <span className="drop-icon"><Icon name="plus"/></span><h3>Drop files or folders here</h3><p>They stream into the persistent queue—without touching the cloud.</p>
        <div className="drop-actions"><button className="button primary" onClick={(event) => { event.stopPropagation(); props.onBrowseFiles() }}><Icon name="file"/>Choose files</button><button className="button ghost" onClick={(event) => { event.stopPropagation(); props.onBrowseFolder() }}><Icon name="folder"/>Choose folder</button></div>
      </div>
      <div className="send-options"><span>When a name already exists</span><select value={props.conflictPolicy} onChange={(event) => props.onConflictPolicy(event.target.value as ConflictPolicy)}><option value="rename">Keep both</option><option value="prompt">Ask first</option><option value="overwrite">Replace</option></select><span className="secure-note"><Icon name="shield"/>SHA-256 verified</span></div>
    </section>

    <aside className="pulse-panel glass-panel">
      <span className="section-kicker">LIVE PULSE</span><div className="pulse-number">{props.activeCount}<span>active</span></div>
      <div className="speed-visual"><i/><i/><i/><i/><i/><i/><i/><i/><i/><i/></div>
      <div className="pulse-stats"><div><span>Combined speed</span><strong>{props.totalSpeed ? `${formatBytes(props.totalSpeed)}/s` : 'Standing by'}</strong></div><div><span>Queue</span><strong>{props.active.length} transfer{props.active.length === 1 ? '' : 's'}</strong></div></div>
      <div className="privacy-card"><Icon name="wifi"/><div><strong>Direct over your LAN</strong><span>No relay. No internet server. No cloud copy.</span></div></div>
    </aside>

    <section className="queue-section">
      <div className="section-heading"><div><span className="section-kicker">TRANSFER QUEUE</span><h2>In motion</h2></div><span className="queue-count">{props.active.length} total</span></div>
      <div className="transfer-list">{props.active.length ? props.active.map((transfer) => <TransferCard key={transfer.uuid} transfer={transfer} onChanged={props.onChanged} onError={props.onError}/>) : <div className="empty-queue"><span><Icon name="send"/></span><h3>Nothing in flight</h3><p>Choose a device and add something worth sending.</p></div>}</div>
    </section>
  </div>
}

function DevicesView({ devices, trustedIDs, onSelect, onPair }: { devices: Device[]; trustedIDs: Set<string>; onSelect: (device: Device) => void; onPair: (device: Device) => void }) {
  return <section className="devices-page"><div className="device-grid">{devices.map((device) => <article className="device-card glass-panel" key={device.deviceId}>
    <div className="device-card-top"><span className="large-device-icon"><PlatformGlyph platform={device.platform}/></span><span className={`presence ${device.online ? 'online' : ''}`}>{device.online ? 'Online' : 'Offline'}</span></div>
    <h2>{device.deviceName}</h2><p>{device.platform} · SyncSpace {device.appVersion}</p>
    <div className="capability-grid"><div><span>Available</span><strong>{formatBytes(device.availableStorage)}</strong></div><div><span>Chunk limit</span><strong>{formatBytes(device.maximumChunkSize)}</strong></div><div><span>Compression</span><strong>{device.compressionSupport ? 'Supported' : 'No'}</strong></div><div><span>Protocol</span><strong>v{device.supportedProtocolVersion}</strong></div></div>
    <div className="device-card-actions">{trustedIDs.has(device.deviceId) ? <><span className="trusted-label"><Icon name="shield"/>Trusted locally</span><button className="button primary" disabled={!device.online || !device.transferCapability} onClick={() => onSelect(device)}>Send files</button></> : <><span className="untrusted-label">Approval needed</span><button className="button ghost" disabled={!device.online} onClick={() => onPair(device)}>Trust device</button></>}</div>
  </article>)}{!devices.length && <div className="large-empty"><div className="radar"><span/><span/><i/></div><h2>No nearby devices yet</h2><p>SyncSpace automatically discovers peers on the same local network.</p></div>}</div></section>
}

function HistoryView({ transfers, onChanged, onError, onCleared }: { transfers: Transfer[]; onChanged: (transfer: Transfer) => void; onError: (message: string) => void; onCleared: () => void }) {
  const clear = async () => { try { await api.clearHistory(); onCleared() } catch (error) { onError(error instanceof Error ? error.message : 'Unable to clear history') } }
  return <section className="history-page"><div className="section-heading"><div><span className="section-kicker">VERIFIED RECORD</span><h2>Transfer history</h2></div>{transfers.length > 0 && <button className="button ghost" onClick={() => void clear()}>Clear history</button>}</div><div className="transfer-list">{transfers.length ? transfers.map((transfer) => <TransferCard key={transfer.uuid} transfer={transfer} onChanged={onChanged} onError={onError}/>) : <div className="large-empty"><span className="empty-history-icon"><Icon name="history"/></span><h2>Your history is clear</h2><p>Completed and cancelled transfers will settle here.</p></div>}</div></section>
}

function PlatformGlyph({ platform }: { platform: string }) {
  const value = platform.toLowerCase()
  return <span className="platform-glyph">{value.includes('android') ? 'A' : value.includes('ios') || value.includes('mac') ? '⌘' : value.includes('linux') ? 'L' : 'W'}</span>
}

function Celebration() {
  return <div className="celebration" aria-hidden="true">{Array.from({ length: 18 }, (_, index) => <i key={index} style={{ '--i': index } as React.CSSProperties}/>)}</div>
}

function filesFromList(list: FileList | null): LocalFile[] {
  return Array.from(list || []).map((file) => ({ file, relativePath: cleanRelativePath((file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name) })).filter((item) => item.relativePath)
}

async function filesFromDrop(data: DataTransfer): Promise<LocalFile[]> {
  const items = Array.from(data.items)
  const entries = items.map((item) => item.webkitGetAsEntry()).filter((entry): entry is FileSystemEntry => entry !== null)
  if (!entries.length) return filesFromList(data.files)
  const output: LocalFile[] = []
  for (const entry of entries) await walkEntry(entry, '', output)
  return output
}

async function walkEntry(entry: FileSystemEntry, parent: string, output: LocalFile[]): Promise<void> {
  const relativePath = cleanRelativePath(parent ? `${parent}/${entry.name}` : entry.name)
  if (entry.isFile) {
    const file = await new Promise<File>((resolve, reject) => (entry as FileSystemFileEntry).file(resolve, reject))
    output.push({ file, relativePath })
    return
  }
  const reader = (entry as FileSystemDirectoryEntry).createReader()
  const children: FileSystemEntry[] = []
  while (true) {
    const batch = await new Promise<FileSystemEntry[]>((resolve, reject) => reader.readEntries(resolve, reject))
    if (!batch.length) break
    children.push(...batch)
  }
  for (const child of children) await walkEntry(child, relativePath, output)
}
