import { useState } from 'react'
import { api } from '../api'
import type { ConflictPolicy, Transfer } from '../types'
import { formatBytes, formatDuration, progressPercent, statusTone } from '../utils'
import { Icon } from './Icon'

interface Props {
  transfer: Transfer
  onChanged: (transfer: Transfer) => void
  onError: (message: string) => void
}

export function TransferCard({ transfer, onChanged, onError }: Props) {
  const [busy, setBusy] = useState(false)
  const [destination, setDestination] = useState('')
  const [policy, setPolicy] = useState<ConflictPolicy>('rename')
  const percent = progressPercent(transfer)
  const canPause = ['Preparing', 'Connecting', 'Negotiating', 'Sending', 'Receiving', 'Resuming'].includes(transfer.status)
  const canCancel = !['Completed', 'Cancelled'].includes(transfer.status)
  const pendingApproval = transfer.direction === 'inbound' && transfer.approvalRequired && !transfer.approved && transfer.status === 'Queued'

  const action = async (kind: 'pause' | 'resume' | 'cancel' | 'retry' | 'reject') => {
    setBusy(true)
    try { onChanged(await api.action(transfer.uuid, kind)) }
    catch (error) { onError(error instanceof Error ? error.message : 'Action failed') }
    finally { setBusy(false) }
  }

  const accept = async () => {
    if (!destination.trim()) {
      onError('Choose a destination folder path first')
      return
    }
    setBusy(true)
    try { onChanged(await api.accept(transfer.uuid, destination.trim(), policy)) }
    catch (error) { onError(error instanceof Error ? error.message : 'Unable to accept transfer') }
    finally { setBusy(false) }
  }

  return <article className={`transfer-card tone-${statusTone(transfer.status)} ${pendingApproval ? 'approval' : ''}`}>
    <div className="transfer-icon-wrap">
      <Icon name={transfer.files?.some((file) => file.directory) ? 'folder' : 'file'} />
      <span className={`direction ${transfer.direction}`}><Icon name={transfer.direction === 'outbound' ? 'arrowUp' : 'arrowDown'} /></span>
    </div>
    <div className="transfer-content">
      <div className="transfer-heading">
        <div>
          <h3>{transfer.filename}</h3>
          <p>{transfer.direction === 'outbound' ? 'To' : 'From'} {transfer.deviceName || 'Nearby device'} <span>·</span> {formatBytes(transfer.size)}</p>
        </div>
        <span className={`status-chip ${statusTone(transfer.status)}`}><i />{transfer.status}</span>
      </div>

      {pendingApproval ? <div className="approval-panel">
        <div className="approval-copy"><Icon name="shield"/><div><strong>Incoming transfer</strong><span>Approval is required before any bytes are accepted.</span></div></div>
        <label><span>Save to folder</span><input value={destination} onChange={(event) => setDestination(event.target.value)} placeholder="C:\\Users\\you\\Downloads" /></label>
        <div className="approval-row">
          <select value={policy} onChange={(event) => setPolicy(event.target.value as ConflictPolicy)} aria-label="File conflict behavior">
            <option value="rename">Keep both (rename)</option>
            <option value="prompt">Ask on conflict</option>
            <option value="overwrite">Replace existing</option>
          </select>
          <button className="button ghost" disabled={busy} onClick={() => void action('reject')}>Decline</button>
          <button className="button primary" disabled={busy} onClick={() => void accept()}><Icon name="check"/>Accept</button>
        </div>
      </div> : <>
        <div className="progress-track" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(percent)} aria-label={`${transfer.filename} progress`}>
          <span style={{ width: `${percent}%` }} />
        </div>
        <div className="transfer-meta">
          <span><strong>{Math.round(percent)}%</strong> · {formatBytes(transfer.progress)} of {formatBytes(transfer.size)}</span>
          <span>{transfer.speed > 0 ? `${formatBytes(transfer.speed)}/s · ${formatDuration(transfer.etaSeconds)} left` : statusMessage(transfer)}</span>
        </div>
        {transfer.error && <p className="transfer-error">{transfer.error}</p>}
      </>}
    </div>
    {!pendingApproval && <div className="transfer-actions">
      {canPause && <button className="icon-button" disabled={busy} onClick={() => void action('pause')} aria-label="Pause transfer"><Icon name="pause"/></button>}
      {transfer.status === 'Paused' && <button className="icon-button accent" disabled={busy} onClick={() => void action('resume')} aria-label="Resume transfer"><Icon name="play"/></button>}
      {transfer.status === 'Failed' && <button className="icon-button accent" disabled={busy} onClick={() => void action('retry')} aria-label="Retry transfer"><Icon name="retry"/></button>}
      {canCancel && <button className="icon-button danger" disabled={busy} onClick={() => void action('cancel')} aria-label="Cancel transfer"><Icon name="close"/></button>}
    </div>}
  </article>
}

function statusMessage(transfer: Transfer): string {
  if (transfer.status === 'Queued') return 'Waiting in queue'
  if (transfer.status === 'Verifying') return 'Checking integrity…'
  if (transfer.status === 'Completed') return transfer.finishTime ? `Completed ${new Date(transfer.finishTime).toLocaleString()}` : 'Verified and complete'
  if (transfer.status === 'Paused') return 'Paused — progress is saved'
  if (transfer.status === 'Cancelled') return 'Cancelled'
  if (transfer.status === 'Failed') return 'Retry available'
  return 'Preparing secure local transfer…'
}
