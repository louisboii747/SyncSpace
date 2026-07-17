import { useEffect, useMemo, useRef, useState } from 'react'
import { api, stageAndQueue } from './api'
import { Icon } from './components/Icon'
import type { IconName } from './components/Icon'
import { TransferCard } from './components/TransferCard'
import { DiagnosticsPage } from './components/DiagnosticsPage'
import { useSyncSpace } from './hooks/useSyncSpace'
import type { ConflictPolicy, Device, LocalFile, PairingRequest, Settings, Transfer, TrustedDevice, UploadProgress, View } from './types'
import { cleanRelativePath, formatBytes, isActive } from './utils'

const emptyUpload: UploadProgress = { active: false, completedBytes: 0, totalBytes: 0, currentName: '', completedFiles: 0, totalFiles: 0 }

export default function App() {
  const sync = useSyncSpace()
	const [view, setView] = useState<View>('home')
  const [selectedDevice, setSelectedDevice] = useState('')
  const [conflictPolicy, setConflictPolicy] = useState<ConflictPolicy>('rename')
  const [dragging, setDragging] = useState(false)
  const [upload, setUpload] = useState<UploadProgress>(emptyUpload)
  const [pairTarget, setPairTarget] = useState<Device | null>(null)
	const [pairing, setPairing] = useState(false)
	const [pairingRequest, setPairingRequest] = useState<PairingRequest | null>(null)
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
	useEffect(() => {
		const incoming = sync.pairingRequests.find((request) => request.direction === 'incoming' && request.state !== 'rejected')
		if (incoming && !pairingRequest) setPairingRequest(incoming)
	}, [pairingRequest, sync.pairingRequests])
	useEffect(() => {
		if (!sync.settings) return
		document.documentElement.dataset.theme = sync.settings.appearance
		document.documentElement.classList.toggle('reduced-motion', sync.settings.reducedMotion)
		setConflictPolicy(sync.settings.conflictPolicy)
	}, [sync.settings])

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

	const startPairing = async () => {
		if (!pairTarget) return
		setPairing(true)
		try {
			const request = await api.requestPairing(pairTarget.deviceId)
			sync.setPairingRequests((current) => [...current.filter((item) => item.requestId !== request.requestId), request])
			setPairingRequest(request)
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
			<NavButton icon="home" label="Home" active={view === 'home'} onClick={() => setView('home')} />
			<NavButton icon="send" label="Transfers" active={view === 'transfers'} badge={sync.active.length || undefined} onClick={() => setView('transfers')} />
			<NavButton icon="devices" label="Devices" active={view === 'devices'} badge={(sync.pairingRequests.length || capableDevices.length) || undefined} onClick={() => setView('devices')} />
			<NavButton icon="history" label="History" active={view === 'history'} onClick={() => setView('history')} />
			<NavButton icon="settings" label="Settings" active={view === 'settings'} onClick={() => setView('settings')} />
        <NavButton icon="diagnostics" label="Diagnostics" active={view === 'diagnostics'} onClick={() => setView('diagnostics')} />
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
		<div><span className="eyebrow">PRIVATE · DIRECT · FAST</span><h1>{view === 'home' ? 'Your private transfer space.' : view === 'transfers' ? 'Move anything, beautifully.' : view === 'devices' ? 'Your nearby space.' : view === 'history' ? 'Everything that moved.' : view === 'settings' ? 'Make SyncSpace yours.' : 'See what SyncSpace sees.'}</h1></div>
        <div className="top-actions">
          <button className="icon-button" aria-label="Enable notifications" onClick={() => { if ('Notification' in window) void Notification.requestPermission() }}><Icon name="bell" /></button>
          <button className="profile" aria-label="Local SyncSpace profile">LS</button>
        </div>
      </header>

      {sync.error && <div className="backend-error"><div><strong>SyncSpace backend is out of reach</strong><span>{sync.error}</span></div><button className="button ghost" onClick={() => void sync.reload()}>Try again</button></div>}
		{sync.loading ? <div className="app-loading glass-panel" aria-busy="true"><span className="loading-pulse"/><strong>Connecting to the local SyncSpace backend…</strong></div> : view === 'home' && <HomeView devices={capableDevices} trusted={sync.trusted} transfers={sync.transfers} onSend={() => setView('transfers')} onDevices={() => setView('devices')} />}
		{!sync.loading && view === 'transfers' && <TransferView
        devices={capableDevices} trustedIDs={sync.trustedIDs} selectedDevice={selectedDevice} onSelect={setSelectedDevice}
        conflictPolicy={conflictPolicy} onConflictPolicy={setConflictPolicy} upload={upload}
        active={sync.active} activeCount={activeCount} totalSpeed={totalSpeed}
        onBrowseFiles={() => fileInput.current?.click()} onBrowseFolder={() => folderInput.current?.click()}
		onPair={setPairTarget} onChanged={onTransferChanged} onError={(message) => sync.toast('error', 'Transfer action failed', message)} defaultDestination={sync.settings?.defaultDownloadDirectory || ''} defaultConflictPolicy={sync.settings?.conflictPolicy || 'rename'}
		/>}
		{view === 'devices' && <DevicesView devices={sync.devices} trusted={sync.trusted} onSelect={(device) => { setSelectedDevice(device.deviceId); setView('transfers') }} onPair={setPairTarget} onTrustedChanged={(device) => sync.setTrusted((current) => [...current.filter((item) => item.deviceId !== device.deviceId), device])} onForgot={(id) => sync.setTrusted((current) => current.filter((item) => item.deviceId !== id))} onError={(message) => sync.toast('error', 'Device action failed', message)} />}
		{view === 'history' && <HistoryView transfers={sync.history} onChanged={onTransferChanged} onError={(message) => sync.toast('error', 'History action failed', message)} onCleared={() => sync.setTransfers((current) => current.filter((item) => !['Completed', 'Cancelled'].includes(item.status)))} defaultDestination={sync.settings?.defaultDownloadDirectory || ''} defaultConflictPolicy={sync.settings?.conflictPolicy || 'rename'} />}
		{view === 'settings' && sync.settings && <SettingsView settings={sync.settings} onChanged={sync.setSettings} onToast={sync.toast} />}
      {view === 'diagnostics' && <DiagnosticsPage websocketConnected={sync.connected} />}
    </main>

    <input ref={fileInput} className="visually-hidden" type="file" multiple onChange={(event) => { void sendFiles(filesFromList(event.currentTarget.files)); event.currentTarget.value = '' }} />
    <input ref={folderInput} className="visually-hidden" type="file" multiple {...{ webkitdirectory: '', directory: '' }} onChange={(event) => { void sendFiles(filesFromList(event.currentTarget.files)); event.currentTarget.value = '' }} />
    {dragging && <div className="drop-overlay"><div className="drop-orb"><Icon name="plus"/></div><h2>Drop into SyncSpace</h2><p>{selected ? `Send directly to ${selected.deviceName}` : 'Choose a nearby device first'}</p></div>}
    {pairTarget && <div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setPairTarget(null) }}><section className="modal" role="dialog" aria-modal="true" aria-labelledby="pair-title">
      <button className="modal-close" onClick={() => setPairTarget(null)} aria-label="Close"><Icon name="close"/></button>
      <div className="modal-icon"><Icon name="shield"/></div><span className="eyebrow">EXPLICIT TRUST</span><h2 id="pair-title">Trust {pairTarget.deviceName}?</h2>
      <p>This local decision allows transfers to this discovered device. SyncSpace never trusts a device merely because it appeared on your network.</p>
      <div className="trust-details"><span>{pairTarget.platform}</span><span>{pairTarget.localIp}</span><span>Protocol v{pairTarget.supportedProtocolVersion}</span></div>
		<div className="modal-actions"><button className="button ghost" onClick={() => setPairTarget(null)}>Not now</button><button className="button primary" disabled={pairing} onClick={() => void startPairing()}><Icon name="shield"/>{pairing ? 'Connecting…' : 'Start secure pairing'}</button></div>
	</section></div>}
		{pairingRequest && <PairingModal request={pairingRequest} onClose={() => setPairingRequest(null)} onUpdated={(request) => { setPairingRequest(request); sync.setPairingRequests((current) => [...current.filter((item) => item.requestId !== request.requestId), request]) }} onPaired={(trusted) => { sync.setTrusted((current) => [...current.filter((item) => item.deviceId !== trusted.deviceId), trusted]); sync.setPairingRequests((current) => current.filter((item) => item.requestId !== pairingRequest.requestId)); setPairingRequest(null); sync.toast('success', 'Secure pairing complete', `${trusted.deviceName} is authenticated and ready.`) }} onError={(message) => sync.toast('error', 'Pairing failed', message)} />}
    {upload.active && <div className="upload-float"><div className="upload-ring" style={{ '--progress': `${upload.totalBytes ? upload.completedBytes / upload.totalBytes * 360 : 0}deg` } as React.CSSProperties}><Icon name="arrowUp"/></div><div><strong>Preparing transfer</strong><span>{upload.currentName}</span><small>{upload.completedFiles + 1} of {upload.totalFiles} · {Math.round(upload.totalBytes ? upload.completedBytes / upload.totalBytes * 100 : 0)}%</small></div></div>}
    <div className="toast-stack" aria-live="polite">{sync.toasts.map((toast) => <div className={`toast ${toast.tone}`} key={toast.id}><span><Icon name={toast.tone === 'success' ? 'check' : toast.tone === 'error' ? 'close' : 'wifi'}/></span><div><strong>{toast.title}</strong><p>{toast.message}</p></div></div>)}</div>
    {sync.celebration > 0 && <Celebration key={sync.celebration} />}
  </div>
}

function HomeView({ devices, trusted, transfers, onSend, onDevices }: { devices: Device[]; trusted: TrustedDevice[]; transfers: Transfer[]; onSend: () => void; onDevices: () => void }) {
	const verified = trusted.filter((device) => !device.blocked && !device.identityKeyChanged)
	const completed = transfers.filter((transfer) => transfer.status === 'Completed')
	const recent = [...transfers].sort((left, right) => Date.parse(right.updatedAt) - Date.parse(left.updatedAt)).slice(0, 3)
	return <section className="home-page">
		<div className="home-hero glass-panel"><div><span className="section-kicker">LOCAL-FIRST FILE SHARING</span><h2>Files travel directly.<br/>Trust stays visible.</h2><p>Discover nearby devices, compare one verification code, then move files over an identity-pinned TLS connection.</p><div className="home-actions"><button className="button primary" onClick={onSend}><Icon name="send"/>Send something</button><button className="button ghost" onClick={onDevices}><Icon name="devices"/>Manage devices</button></div></div><div className="secure-orbit"><span><Icon name="shield"/></span><i/><i/><i/></div></div>
		<div className="home-stats"><article className="glass-panel"><span>Nearby now</span><strong>{devices.filter((device) => device.online).length}</strong><small>discovered on this LAN</small></article><article className="glass-panel"><span>Verified devices</span><strong>{verified.length}</strong><small>identity keys pinned</small></article><article className="glass-panel"><span>Completed</span><strong>{completed.length}</strong><small>integrity checked</small></article></div>
		<div className="home-lower"><section className="glass-panel"><div className="section-heading"><div><span className="section-kicker">RECENT ACTIVITY</span><h2>Latest transfers</h2></div><button className="text-button" onClick={onSend}>Open queue</button></div>{recent.length ? <div className="mini-activity">{recent.map((transfer) => <div key={transfer.uuid}><span><Icon name={transfer.direction === 'outbound' ? 'arrowUp' : 'arrowDown'}/></span><div><strong>{transfer.filename}</strong><small>{transfer.direction === 'outbound' ? 'To' : 'From'} {transfer.deviceName || 'device'}</small></div><em>{transfer.status}</em></div>)}</div> : <div className="home-empty">Your first verified transfer will appear here.</div>}</section><aside className="glass-panel privacy-promise"><Icon name="wifi"/><h3>Your network is the route</h3><p>No account, relay, cloud upload, analytics, or internet fallback is used for core transfers.</p></aside></div>
	</section>
}

function PairingModal({ request, onClose, onUpdated, onPaired, onError }: { request: PairingRequest; onClose: () => void; onUpdated: (request: PairingRequest) => void; onPaired: (device: TrustedDevice) => void; onError: (message: string) => void }) {
	const [busy, setBusy] = useState(false)
	useEffect(() => {
		if (!request.localConfirmed || request.state === 'rejected' || request.state === 'paired') return
		const poll = window.setInterval(() => {
			void api.refreshPairing(request.requestId).then((decision) => { if (decision.trustedDevice) onPaired(decision.trustedDevice); else onUpdated(decision.request) }).catch((error) => onError(error instanceof Error ? error.message : 'Unable to refresh pairing'))
		}, 1500)
		return () => window.clearInterval(poll)
	}, [onError, onPaired, onUpdated, request.localConfirmed, request.requestId, request.state])
	const confirm = async () => { setBusy(true); try { const decision = await api.confirmPairing(request.requestId); if (decision.trustedDevice) onPaired(decision.trustedDevice); else onUpdated(decision.request) } catch (error) { onError(error instanceof Error ? error.message : 'Unable to confirm pairing') } finally { setBusy(false) } }
	const reject = async () => { setBusy(true); try { await api.rejectPairing(request.requestId); onClose() } catch (error) { onError(error instanceof Error ? error.message : 'Unable to reject pairing') } finally { setBusy(false) } }
	return <div className="modal-backdrop"><section className="modal pairing-modal" role="dialog" aria-modal="true" aria-labelledby="verify-title">
		<button className="modal-close" onClick={onClose} aria-label="Close"><Icon name="close"/></button><div className="modal-icon"><Icon name="shield"/></div><span className="eyebrow">AUTHENTICATED PAIRING · {request.direction.toUpperCase()}</span><h2 id="verify-title">Verify {request.deviceName}</h2>
		<p>Check that this exact code appears on both devices. A mismatch means someone may be intercepting the connection.</p><div className="verification-code" aria-label={`Verification code ${request.verificationCode}`}>{request.verificationCode}</div>
		<div className="fingerprint-box"><span>Ed25519 identity fingerprint</span><code>{request.fingerprint}</code></div>
		{request.localConfirmed ? <div className="pairing-wait"><span className="loading-pulse"/><div><strong>Your confirmation is recorded</strong><small>Waiting for the other device to confirm the same code…</small></div></div> : <div className="modal-actions"><button className="button ghost" disabled={busy} onClick={() => void reject()}>Codes differ</button><button className="button primary" disabled={busy} onClick={() => void confirm()}><Icon name="check"/>{busy ? 'Confirming…' : 'Codes match'}</button></div>}
	</section></div>
}

function SettingsView({ settings, onChanged, onToast }: { settings: Settings; onChanged: (settings: Settings) => void; onToast: (tone: 'success' | 'error' | 'info', title: string, message: string) => void }) {
	const [draft, setDraft] = useState(settings)
	const [busy, setBusy] = useState(false)
	useEffect(() => setDraft(settings), [settings])
	const update = <K extends keyof Settings,>(key: K, value: Settings[K]) => setDraft((current) => ({ ...current, [key]: value }))
	const save = async () => { setBusy(true); try { const saved = await api.updateSettings(draft); onChanged(saved); if (saved.notificationsEnabled && 'Notification' in window && Notification.permission === 'default') void Notification.requestPermission(); onToast('success', 'Settings saved', 'The local backend and interface now use these preferences.') } catch (error) { onToast('error', 'Couldn’t save settings', error instanceof Error ? error.message : 'Unknown settings error') } finally { setBusy(false) } }
	const reset = async () => { setBusy(true); try { const defaults = await api.resetSettings(); setDraft(defaults); onChanged(defaults); onToast('info', 'Defaults restored', 'SyncSpace returned to its safe defaults.') } catch (error) { onToast('error', 'Couldn’t reset settings', error instanceof Error ? error.message : 'Unknown settings error') } finally { setBusy(false) } }
	return <section className="settings-page"><div className="settings-grid">
		<article className="glass-panel settings-card"><div><span className="section-kicker">APPEARANCE</span><h2>Theme and motion</h2></div><label><span>Colour mode</span><select value={draft.appearance} onChange={(event) => update('appearance', event.target.value as Settings['appearance'])}><option value="system">Follow system</option><option value="dark">Dark</option><option value="light">Light</option></select></label><label className="toggle-row"><span><strong>Reduce motion</strong><small>Use calmer transitions and no celebratory movement.</small></span><input type="checkbox" checked={draft.reducedMotion} onChange={(event) => update('reducedMotion', event.target.checked)}/></label></article>
		<article className="glass-panel settings-card"><div><span className="section-kicker">DOWNLOADS</span><h2>Incoming files</h2></div><label><span>Default destination</span><input value={draft.defaultDownloadDirectory} onChange={(event) => update('defaultDownloadDirectory', event.target.value)} placeholder="Absolute Downloads folder path"/><small>Incoming requests still require approval. You can change this path each time.</small></label><label><span>Name conflicts</span><select value={draft.conflictPolicy} onChange={(event) => update('conflictPolicy', event.target.value as ConflictPolicy)}><option value="rename">Keep both safely</option><option value="prompt">Ask on conflict</option><option value="overwrite">Replace deliberately</option></select></label></article>
		<article className="glass-panel settings-card"><div><span className="section-kicker">NOTIFICATIONS</span><h2>Useful, not noisy</h2></div><label className="toggle-row"><span><strong>Completion notifications</strong><small>Show a system notification after a verified transfer completes.</small></span><input type="checkbox" checked={draft.notificationsEnabled} onChange={(event) => update('notificationsEnabled', event.target.checked)}/></label><div className="settings-note"><Icon name="shield"/><p>Private keys, pairing secrets, and full local paths are never included in browser responses or diagnostic exports.</p></div></article>
	</div><div className="settings-actions"><button className="button ghost" disabled={busy} onClick={() => void reset()}>Restore defaults</button><button className="button primary" disabled={busy} onClick={() => void save()}><Icon name="check"/>{busy ? 'Saving…' : 'Save settings'}</button></div></section>
}

function NavButton({ icon, label, active, badge, onClick }: { icon: IconName; label: string; active: boolean; badge?: number; onClick: () => void }) {
  return <button className={active ? 'active' : ''} onClick={onClick}><Icon name={icon}/><span>{label}</span>{badge !== undefined && <em>{badge}</em>}</button>
}

interface TransferViewProps {
  devices: Device[]; trustedIDs: Set<string>; selectedDevice: string; onSelect: (id: string) => void
  conflictPolicy: ConflictPolicy; onConflictPolicy: (policy: ConflictPolicy) => void; upload: UploadProgress
  active: Transfer[]; activeCount: number; totalSpeed: number
  onBrowseFiles: () => void; onBrowseFolder: () => void; onPair: (device: Device) => void
	onChanged: (transfer: Transfer) => void; onError: (message: string) => void
	defaultDestination: string; defaultConflictPolicy: ConflictPolicy
}

export function TransferView(props: TransferViewProps) {
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
		<div className="transfer-list">{props.active.length ? props.active.map((transfer) => <TransferCard key={transfer.uuid} transfer={transfer} onChanged={props.onChanged} onError={props.onError} defaultDestination={props.defaultDestination} defaultConflictPolicy={props.defaultConflictPolicy}/>) : <div className="empty-queue"><span><Icon name="send"/></span><h3>Nothing in flight</h3><p>Choose a device and add something worth sending.</p></div>}</div>
    </section>
  </div>
}

export function DevicesView({ devices, trusted, onSelect, onPair, onTrustedChanged, onForgot, onError }: { devices: Device[]; trusted: TrustedDevice[]; onSelect: (device: Device) => void; onPair: (device: Device) => void; onTrustedChanged: (device: TrustedDevice) => void; onForgot: (id: string) => void; onError: (message: string) => void }) {
	const byID = new Map(trusted.map((device) => [device.deviceId, device]))
	const updateBlock = async (device: TrustedDevice) => { try { onTrustedChanged(await api.setDeviceBlocked(device.deviceId, !device.blocked)) } catch (error) { onError(error instanceof Error ? error.message : 'Unable to change block state') } }
	const forget = async (device: TrustedDevice) => { if (!window.confirm(`Forget ${device.deviceName}? A new verification-code pairing will be required.`)) return; try { await api.forgetDevice(device.deviceId); onForgot(device.deviceId) } catch (error) { onError(error instanceof Error ? error.message : 'Unable to forget device') } }
	return <section className="devices-page"><div className="device-grid">{devices.map((device) => {
		const trust = byID.get(device.deviceId)
		return <article className="device-card glass-panel" key={device.deviceId}>
			<div className="device-card-top"><span className="large-device-icon"><PlatformGlyph platform={device.platform}/></span><span className={`presence ${device.online ? 'online' : ''}`}>{device.online ? 'Online' : 'Offline'}</span></div>
			<h2>{trust?.localName || device.deviceName}</h2><p>{device.platform} · SyncSpace {device.appVersion}</p>
			<div className="capability-grid"><div><span>Available</span><strong>{formatBytes(device.availableStorage)}</strong></div><div><span>Chunk limit</span><strong>{formatBytes(device.maximumChunkSize)}</strong></div><div><span>Pairing</span><strong>{device.pairingAvailable ? 'Secure v1' : 'Unavailable'}</strong></div><div><span>Protocol</span><strong>v{device.supportedProtocolVersion}</strong></div></div>
			{trust && <div className={`fingerprint-box ${trust.identityKeyChanged ? 'warning' : ''}`}><span>{trust.identityKeyChanged ? 'Identity changed — transfers blocked' : 'Pinned identity fingerprint'}</span><code>{trust.fingerprint}</code></div>}
			<div className="device-card-actions">{trust ? <><span className={trust.blocked || trust.identityKeyChanged ? 'untrusted-label' : 'trusted-label'}><Icon name="shield"/>{trust.identityKeyChanged ? 'Identity warning' : trust.blocked ? 'Blocked' : 'Verified'}</span><button className="button ghost" onClick={() => void updateBlock(trust)}>{trust.blocked ? 'Unblock' : 'Block'}</button><button className="button ghost danger-text" onClick={() => void forget(trust)}>Forget</button><button className="button primary" disabled={!device.online || !device.transferCapability || trust.blocked || trust.identityKeyChanged} onClick={() => onSelect(device)}>Send files</button></> : <><span className="untrusted-label">Verification required</span><button className="button ghost" disabled={!device.online || !device.pairingAvailable} onClick={() => onPair(device)}>Pair securely</button></>}</div>
		</article>
	})}{!devices.length && <div className="large-empty"><div className="radar"><span/><span/><i/></div><h2>No nearby devices yet</h2><p>SyncSpace automatically discovers peers on the same local network.</p></div>}</div></section>
}

export function HistoryView({ transfers, onChanged, onError, onCleared, defaultDestination, defaultConflictPolicy }: { transfers: Transfer[]; onChanged: (transfer: Transfer) => void; onError: (message: string) => void; onCleared: () => void; defaultDestination: string; defaultConflictPolicy: ConflictPolicy }) {
  const clear = async () => { try { await api.clearHistory(); onCleared() } catch (error) { onError(error instanceof Error ? error.message : 'Unable to clear history') } }
	return <section className="history-page"><div className="section-heading"><div><span className="section-kicker">VERIFIED RECORD</span><h2>Transfer history</h2></div>{transfers.length > 0 && <button className="button ghost" onClick={() => void clear()}>Clear history</button>}</div><div className="transfer-list">{transfers.length ? transfers.map((transfer) => <TransferCard key={transfer.uuid} transfer={transfer} onChanged={onChanged} onError={onError} defaultDestination={defaultDestination} defaultConflictPolicy={defaultConflictPolicy}/>) : <div className="large-empty"><span className="empty-history-icon"><Icon name="history"/></span><h2>Your history is clear</h2><p>Completed and cancelled transfers will settle here.</p></div>}</div></section>
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
