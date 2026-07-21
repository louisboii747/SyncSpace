import { useState } from 'react'
import { api } from '../api'
import type { ConflictPolicy, Transfer, TransferStatus } from '../types'
import {
	formatBytes,
	formatDuration,
	isActive,
	isPotentiallyExecutable,
	isTerminal,
	progressPercent,
	statusTone,
} from '../utils'
import { Icon } from './Icon'

interface Props {
	transfer: Transfer
	onChanged: (transfer: Transfer) => void
	onError: (message: string) => void
	defaultDestination?: string
	defaultConflictPolicy?: ConflictPolicy
}

export function TransferCard({
	transfer,
	onChanged,
	onError,
	defaultDestination = '',
	defaultConflictPolicy = 'rename',
}: Props) {
	const [busy, setBusy] = useState(false)
	const [destination, setDestination] = useState(defaultDestination)
	const [policy, setPolicy] = useState<ConflictPolicy>(defaultConflictPolicy)
	const percent = progressPercent(transfer)
	const outbound = transfer.direction === 'outbound'
	const canPause = outbound && ['Preparing', 'Connecting', 'Negotiating', 'Sending', 'Resuming'].includes(transfer.status)
	const canResume = outbound && transfer.status === 'Paused'
	const canRetry = outbound && transfer.status === 'Failed'
	const canCancel = !isTerminal(transfer)
	const pendingApproval = (
		transfer.direction === 'inbound'
		&& transfer.approvalRequired
		&& !transfer.approved
		&& transfer.status === 'Queued'
	)
	const containsExecutable = Boolean(transfer.files?.some((file) => (
		!file.directory && isPotentiallyExecutable(file.relativePath)
	)))
	const manifestPreview = transfer.files?.slice(0, 50) || []
	const remainingManifestItems = Math.max(0, (transfer.files?.length || 0) - manifestPreview.length)

	const action = async (kind: 'pause' | 'resume' | 'cancel' | 'retry' | 'reject') => {
		setBusy(true)
		try {
			onChanged(await api.action(transfer.uuid, kind))
		} catch (error) {
			onError(error instanceof Error ? error.message : 'SyncSpace could not complete that transfer action.')
		} finally {
			setBusy(false)
		}
	}

	const accept = async () => {
		if (!destination.trim()) {
			onError('Choose the folder where these files should be saved.')
			return
		}
		setBusy(true)
		try {
			onChanged(await api.accept(transfer.uuid, destination.trim(), policy))
		} catch (error) {
			onError(error instanceof Error ? error.message : 'SyncSpace could not accept this transfer.')
		} finally {
			setBusy(false)
		}
	}

	return (
		<article className={`transfer-card tone-${statusTone(transfer.status)} ${pendingApproval ? 'approval' : ''}`}>
			<div className="transfer-icon-wrap">
				<Icon name={transfer.files?.some((file) => file.directory) ? 'folder' : 'file'} />
				<span className={`direction ${transfer.direction}`}>
					<Icon name={outbound ? 'arrowUp' : 'arrowDown'} />
				</span>
			</div>

			<div className="transfer-content">
				<div className="transfer-heading">
					<div>
						<h3>{transfer.filename}</h3>
						<p>
							{outbound ? 'Sending to' : 'Receiving from'} {transfer.deviceName || 'another device'}
							<span>·</span>
							{formatBytes(transfer.size)}
							{transfer.files?.length ? <><span>·</span>{transfer.files.length} {transfer.files.length === 1 ? 'item' : 'items'}</> : null}
						</p>
					</div>
					<span className={`status-chip ${statusTone(transfer.status)}`}>
						<i />
						{friendlyStatus(transfer.status)}
					</span>
				</div>

				{pendingApproval ? (
					<div className="approval-panel">
						<div className="approval-copy">
							<Icon name="shield" />
							<div>
								<strong>{transfer.deviceName || 'Another device'} wants to send you files</strong>
								<span>Review the request and choose a save folder. No file content is accepted until you approve it.</span>
							</div>
						</div>
						<div className="trust-details">
							<span className="trusted-label"><Icon name="shield" />Trusted device</span>
							{transfer.deviceHostname && <span>{transfer.deviceHostname}</span>}
							{transfer.devicePlatform && <span>{friendlyPlatform(transfer.devicePlatform)}</span>}
						</div>

						{containsExecutable && (
							<div className="content-warning" role="note">
								<Icon name="shield" />
								<p>
									<strong>This transfer contains executable or script files.</strong>
									Only accept files from devices and people you trust.
								</p>
							</div>
						)}

						{manifestPreview.length > 0 && (
							<section className="approval-manifest" aria-label="Files offered in this transfer">
								<div className="approval-manifest-heading">
									<strong>Files in this transfer</strong>
									<span>{transfer.files!.length} {transfer.files!.length === 1 ? 'item' : 'items'}</span>
								</div>
								<ul>
									{manifestPreview.map((file) => (
										<li key={file.fileId || file.relativePath}>
											<Icon name={file.directory ? 'folder' : 'file'} />
											<span title={file.relativePath}>{file.relativePath}</span>
											<small>{file.directory ? 'Folder' : formatBytes(file.size)}</small>
										</li>
									))}
								</ul>
								{remainingManifestItems > 0 && (
									<p className="approval-manifest-more">And {remainingManifestItems} more {remainingManifestItems === 1 ? 'item' : 'items'}.</p>
								)}
							</section>
						)}

						<label>
							<span>Save these files in</span>
							<input
								value={destination}
								onChange={(event) => setDestination(event.target.value)}
								placeholder="Enter an absolute Downloads folder path"
							/>
						</label>
						<div className="approval-row">
							<label className="approval-conflict-policy">
								<span>If a name already exists</span>
								<select value={policy} onChange={(event) => setPolicy(event.target.value as ConflictPolicy)}>
									<option value="rename">Keep both files</option>
									<option value="prompt">Ask before deciding</option>
									<option value="overwrite">Replace the existing file</option>
								</select>
							</label>
							<button className="button ghost" disabled={busy} onClick={() => void action('reject')}>Decline</button>
							<button className="button primary" disabled={busy} onClick={() => void accept()}>
								<Icon name="check" />
								{busy ? 'Accepting…' : 'Accept transfer'}
							</button>
						</div>
					</div>
				) : (
					<>
						<div
							className="progress-track"
							role="progressbar"
							aria-valuemin={0}
							aria-valuemax={100}
							aria-valuenow={Math.round(percent)}
							aria-label={`${transfer.filename} transfer progress`}
						>
							<span style={{ width: `${percent}%` }} />
						</div>
						<div className="transfer-meta">
							<span><strong>{Math.round(percent)}%</strong> · {formatBytes(transfer.progress)} of {formatBytes(transfer.size)}</span>
							<span>
								{isActive(transfer) && transfer.speed > 0
									? `${formatBytes(transfer.speed)}/s · ${formatDuration(transfer.etaSeconds)} remaining`
									: statusMessage(transfer)}
							</span>
						</div>
						{transfer.error && <p className="transfer-error" role="alert">{transfer.error}</p>}
					</>
				)}
			</div>

			{!pendingApproval && (canPause || canResume || canRetry || canCancel) && (
				<div className="transfer-actions">
					{canPause && (
						<button className="button ghost transfer-action-button" disabled={busy} onClick={() => void action('pause')}>
							<Icon name="pause" />Pause
						</button>
					)}
					{canResume && (
						<button className="button ghost transfer-action-button" disabled={busy} onClick={() => void action('resume')}>
							<Icon name="play" />Resume
						</button>
					)}
					{canRetry && (
						<button className="button ghost transfer-action-button" disabled={busy} onClick={() => void action('retry')}>
							<Icon name="retry" />Try again
						</button>
					)}
					{canCancel && (
						<button className="button ghost transfer-action-button danger" disabled={busy} onClick={() => void action('cancel')}>
							<Icon name="close" />Cancel
						</button>
					)}
				</div>
			)}
		</article>
	)
}

function friendlyStatus(status: TransferStatus): string {
	const labels: Record<TransferStatus, string> = {
		Queued: 'Waiting',
		Preparing: 'Preparing',
		Connecting: 'Connecting',
		Negotiating: 'Agreeing details',
		Sending: 'Sending',
		Receiving: 'Receiving',
		Paused: 'Paused',
		Resuming: 'Resuming',
		Completed: 'Completed',
		Cancelled: 'Cancelled',
		Failed: 'Needs attention',
		Verifying: 'Checking file',
	}
	return labels[status]
}

function statusMessage(transfer: Transfer): string {
	if (transfer.status === 'Queued') return transfer.direction === 'inbound' ? 'Waiting for your approval' : 'Waiting in the queue'
	if (transfer.status === 'Verifying') return 'Checking that the completed file arrived correctly…'
	if (transfer.status === 'Completed') {
		return transfer.finishTime
			? `Completed ${new Date(transfer.finishTime).toLocaleString()}`
			: 'The transfer completed and passed its integrity check'
	}
	if (transfer.status === 'Paused') return 'Paused on the sending device'
	if (transfer.status === 'Cancelled') return 'This transfer was cancelled'
	if (transfer.status === 'Failed') return transfer.direction === 'outbound'
		? 'Review the error, then try again if it is safe to do so'
		: 'The sender must start a new transfer'
	return 'Preparing the local connection…'
}

function friendlyPlatform(platform: string): string {
	return platform ? platform.charAt(0).toUpperCase() + platform.slice(1) : 'Platform unavailable'
}
