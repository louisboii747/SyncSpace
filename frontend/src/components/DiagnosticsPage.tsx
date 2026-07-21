import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import type { Diagnostics } from '../types'
import { formatBytes } from '../utils'
import { Icon } from './Icon'

export function DiagnosticsPage({ websocketConnected }: { websocketConnected: boolean }) {
  const [snapshot, setSnapshot] = useState<Diagnostics | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setSnapshot(await api.diagnostics())
      setError('')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Unable to load diagnostics')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const action = async (name: string, task: () => Promise<unknown>, success: string) => {
    setBusy(name)
    setMessage('')
    setError('')
    try {
      await task()
      setMessage(success)
      await load()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Diagnostic action failed')
    } finally {
      setBusy('')
    }
  }

  if (loading && !snapshot) return <section className="diagnostics-page" aria-busy="true"><div className="diagnostics-loading glass-panel"><span className="loading-pulse"/><h2>Reading runtime diagnostics…</h2><p>Checking the local database, storage, discovery, and transfer services.</p></div></section>
  if (!snapshot) return <section className="diagnostics-page"><div className="backend-error"><div><strong>Diagnostics unavailable</strong><span>{error}</span></div><button className="button ghost" onClick={() => void load()}>Try again</button></div></section>

  const healthEntries = Object.entries(snapshot.health.checks)
  return <section className="diagnostics-page">
    <div className="diagnostics-actions glass-panel">
      <div><span className="section-kicker">LOCAL SERVICE INFORMATION</span><h2>Runtime diagnostics</h2><p>Diagnostic exports leave out file contents and secret keys, but may include device names, IP addresses, filenames, local paths, and recent errors. Review before sharing.</p></div>
      <div className="diagnostics-button-row">
        <button className="button ghost" disabled={Boolean(busy)} onClick={() => void action('health', api.runHealthCheck, 'Health check completed.')}><Icon name="check"/>Run health check</button>
        <button className="button ghost" disabled={Boolean(busy)} onClick={() => void action('discovery', api.refreshDevices, 'Discovery refresh requested.')}><Icon name="refresh"/>Refresh discovery</button>
		<a className="button ghost" href="/api/v1/diagnostics/export" download><Icon name="arrowDown"/>Export bundle</a>
      </div>
    </div>
    {(message || error) && <div className={`diagnostics-notice ${error ? 'error' : 'success'}`} role="status">{error || message}</div>}

    <DiagnosticsOverview snapshot={snapshot} websocketConnected={websocketConnected}/>

    <div className="diagnostics-grid">
      <article className="diagnostic-card glass-panel"><span className="section-kicker">LOCAL DEVICE</span><h3>{snapshot.localDevice.deviceName}</h3><dl>
        <Row label="Device ID" value={snapshot.localDevice.deviceId}/><Row label="Platform" value={snapshot.platform}/><Row label="Backend URL" value={snapshot.backendUrl}/><Row label="Storage" value={snapshot.storagePath}/><Row label="Database" value={snapshot.databasePath}/><Row label="Mode" value={snapshot.developerMode ? 'Developer' : 'Production'}/>
      </dl></article>
      <article className="diagnostic-card glass-panel"><span className="section-kicker">HEALTH CHECKS</span><h3>{healthEntries.filter(([, check]) => check.ok).length} of {healthEntries.length} passing</h3><div className="health-list">{healthEntries.map(([name, check]) => <div key={name}><span className={check.ok ? 'check-good' : 'check-bad'}>{check.ok ? '✓' : '!'}</span><p><strong>{name}</strong><small>{check.detail || 'No detail'}</small></p></div>)}</div></article>
      <article className="diagnostic-card glass-panel"><span className="section-kicker">LIVE COUNTS</span><h3>Services and queues</h3><dl>
        <Row label="Known devices" value={String(snapshot.knownDevices.length)}/><Row label="Trusted devices" value={String(snapshot.trustedDevices.length)}/><Row label="Active transfers" value={String(snapshot.activeTransfers.length)}/><Row label="Queued transfers" value={String(snapshot.transferQueue.length)}/><Row label="WebSocket clients" value={String(snapshot.webSockets.discovery + snapshot.webSockets.pairing + snapshot.webSockets.transfers)}/>
      </dl></article>
    </div>

    <DiagnosticTable title="Known devices" empty="No devices are currently known." headings={['Device', 'Address', 'Trust / state']} rows={snapshot.knownDevices.map((device) => [device.deviceName, `${device.localIp}:${device.port}`, `${snapshot.trustedDevices.some((item) => item.deviceId === device.deviceId) ? 'Trusted' : 'Untrusted'} · ${device.online ? 'Online' : 'Offline'}`])}/>
    <DiagnosticTable title="Trusted devices" empty="No devices have been trusted locally." headings={['Device', 'Platform', 'Last seen']} rows={snapshot.trustedDevices.map((device) => [device.deviceName, device.platform, new Date(device.lastSeen).toLocaleString()])}/>
    <DiagnosticTable title="Active transfers" empty="There are no active transfers." headings={['Transfer', 'Device', 'Progress']} rows={snapshot.activeTransfers.map((item) => [item.filename, item.deviceName || 'Unknown device', `${formatBytes(item.progress)} / ${formatBytes(item.size)} · ${item.status}`])}/>
    <DiagnosticTable title="Transfer queue" empty="The transfer queue is empty." headings={['Transfer', 'Device', 'Progress']} rows={snapshot.transferQueue.map((item) => [item.filename, item.deviceName || 'Unknown device', `${formatBytes(item.progress)} / ${formatBytes(item.size)} · ${item.status}`])}/>

    <div className="diagnostics-grid logs-grid">
      <article className="diagnostic-card glass-panel"><span className="section-kicker">LOG PREVIEW</span><h3>Recent backend activity</h3><LogList entries={snapshot.logs}/></article>
      <article className="diagnostic-card glass-panel"><span className="section-kicker">LAST ERRORS</span><h3>Recent errors</h3><LogList entries={snapshot.lastErrors}/></article>
    </div>
  </section>
}

export function DiagnosticsOverview({ snapshot, websocketConnected }: { snapshot: Diagnostics; websocketConnected: boolean }) { return <div className="diagnostics-summary">
  <DiagnosticValue label="API health" value={snapshot.health.status.toUpperCase()} tone={snapshot.health.status === 'ok' ? 'good' : 'bad'} />
  <DiagnosticValue label="WebSocket" value={websocketConnected ? 'CONNECTED' : 'DISCONNECTED'} tone={websocketConnected ? 'good' : 'bad'} />
  <DiagnosticValue label="Discovery" value={snapshot.discoveryRunning ? 'RUNNING' : 'STOPPED'} tone={snapshot.discoveryRunning ? 'good' : 'bad'} />
  <DiagnosticValue label="Protocol" value={`v${snapshot.protocolVersion}`} />
</div> }

function DiagnosticValue({ label, value, tone = '' }: { label: string; value: string; tone?: string }) { return <div className="diagnostic-value glass-panel"><span>{label}</span><strong className={tone}>{value}</strong></div> }
function Row({ label, value }: { label: string; value: string }) { return <div><dt>{label}</dt><dd title={value}>{value}</dd></div> }
function DiagnosticTable({ title, empty, headings, rows }: { title: string; empty: string; headings: string[]; rows: string[][] }) { return <article className="diagnostic-table glass-panel"><div className="section-heading"><div><span className="section-kicker">RUNTIME STATE</span><h2>{title}</h2></div><span className="queue-count">{rows.length} total</span></div>{rows.length ? <div className="table-scroll"><table><thead><tr>{headings.map((heading) => <th key={heading}>{heading}</th>)}</tr></thead><tbody>{rows.map((row, index) => <tr key={`${row[0]}-${index}`}>{row.map((cell, cellIndex) => <td key={headings[cellIndex]}>{cell}</td>)}</tr>)}</tbody></table></div> : <p className="diagnostic-empty">{empty}</p>}</article> }
function LogList({ entries }: { entries: Diagnostics['logs'] }) { return <div className="diagnostic-logs">{entries.length ? entries.slice(-12).reverse().map((entry, index) => <div key={`${entry.time}-${index}`}><time>{new Date(entry.time).toLocaleTimeString()}</time><span className={entry.level.toLowerCase()}>{entry.level}</span><p>{entry.message}</p></div>) : <p className="diagnostic-empty">No entries recorded.</p>}</div> }
