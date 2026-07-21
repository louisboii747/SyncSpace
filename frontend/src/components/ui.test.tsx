import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import {
	DevicesView,
	HistoryView,
	PrivacyGate,
	SendReviewModal,
	TransferView,
} from '../App'
import { deviceFixture, diagnosticsFixture, transferFixture } from '../testFixtures'
import { DiagnosticsOverview, DiagnosticsPage } from './DiagnosticsPage'
import { TransferCard } from './TransferCard'

const noop = vi.fn()

describe('SyncSpace React views', () => {
	it('renders discovered devices and explains their pairing state', () => {
		const html = renderToStaticMarkup(
			<DevicesView
				devices={[deviceFixture]}
				discoveryEnabled={true}
				localDevice={null}
				trusted={[]}
				onSelect={noop}
				onPair={noop}
				onRefresh={noop}
				onOpenSettings={noop}
				onTrustedChanged={noop}
				onForgot={noop}
				onError={noop}
			/>,
		)
		expect(html).toContain('Studio Laptop')
		expect(html).toContain('studio-laptop')
		expect(html).toContain('Not paired yet')
		expect(html).toContain('Pair device')
	})

	it('labels incompatible devices clearly and does not offer pairing', () => {
		const incompatible = {
			...deviceFixture,
			transferCapability: false,
			supportedProtocolVersion: 0,
			pairingAvailable: false,
		}
		const html = renderToStaticMarkup(
			<DevicesView
				devices={[incompatible]}
				discoveryEnabled={true}
				localDevice={null}
				trusted={[]}
				onSelect={noop}
				onPair={noop}
				onRefresh={noop}
				onOpenSettings={noop}
				onTrustedChanged={noop}
				onForgot={noop}
				onError={noop}
			/>,
		)
		expect(html).toContain('needs a compatible SyncSpace version')
		expect(html).toContain('Update needed')
		expect(html).toContain('disabled=""')
	})

	it('marks the local device and does not present it as a destination', () => {
		const html = renderToStaticMarkup(
			<DevicesView
				devices={[]}
				discoveryEnabled={true}
				localDevice={deviceFixture}
				trusted={[]}
				onSelect={noop}
				onPair={noop}
				onRefresh={noop}
				onOpenSettings={noop}
				onTrustedChanged={noop}
				onForgot={noop}
				onError={noop}
			/>,
		)
		expect(html).toContain('This device')
		expect(html).toContain('Managed on this device')
	})

	it('renders transfer queue progress updates', () => {
		const first = renderToStaticMarkup(
			<TransferCard transfer={transferFixture('Sending', 20)} onChanged={noop} onError={noop} />,
		)
		const second = renderToStaticMarkup(
			<TransferCard transfer={transferFixture('Sending', 80)} onChanged={noop} onError={noop} />,
		)
		expect(first).toContain('aria-valuenow="20"')
		expect(second).toContain('aria-valuenow="80"')
	})

	it('renders failed and completed outbound transfer states', () => {
		const failed = renderToStaticMarkup(
			<TransferCard
				transfer={{ ...transferFixture('Failed'), error: 'checksum mismatch' }}
				onChanged={noop}
				onError={noop}
			/>,
		)
		const completed = renderToStaticMarkup(
			<TransferCard
				transfer={{ ...transferFixture('Completed', 100), speed: 0 }}
				onChanged={noop}
				onError={noop}
			/>,
		)
		expect(failed).toContain('checksum mismatch')
		expect(failed).toContain('Try again')
		expect(completed).toContain('passed its integrity check')
		expect(completed).toContain('aria-valuenow="100"')
	})

	it('warns about executable incoming files and hides unsupported inbound actions', () => {
		const inbound = {
			...transferFixture('Queued', 0),
			direction: 'inbound' as const,
			approved: false,
			approvalRequired: true,
			files: [{
				fileId: 'script',
				relativePath: 'tools/setup.ps1',
				directory: false,
				size: 10,
				checksum: '',
				chunkSize: 10,
				chunkCount: 1,
			}],
		}
		const approval = renderToStaticMarkup(
			<TransferCard transfer={inbound} onChanged={noop} onError={noop} />,
		)
		const failed = renderToStaticMarkup(
			<TransferCard transfer={{ ...inbound, status: 'Failed', approvalRequired: false }} onChanged={noop} onError={noop} />,
		)
		expect(approval).toContain('contains executable or script files')
		expect(approval).toContain('Only accept files from devices and people you trust')
		expect(approval).toContain('Files in this transfer')
		expect(approval).toContain('tools/setup.ps1')
		expect(approval).toContain('Trusted device')
		expect(approval).toContain('studio-laptop')
		expect(approval).toContain('Linux')
		expect(failed).not.toContain('Try again')
		expect(failed).not.toContain('Pause')
	})

	it('renders practical transfer and history empty states', () => {
		const queue = renderToStaticMarkup(
			<TransferView
				devices={[]}
				discoveryEnabled={true}
				trustedIDs={new Set()}
				selectedDevice=""
				onSelect={noop}
				active={[]}
				activeCount={0}
				totalSpeed={0}
				onBrowseFiles={noop}
				onBrowseFolder={noop}
				onPair={noop}
				onRefresh={noop}
				onOpenSettings={noop}
				onChanged={noop}
				onError={noop}
				defaultDestination=""
				defaultConflictPolicy="rename"
			/>,
		)
		const history = renderToStaticMarkup(
			<HistoryView
				transfers={[]}
				onChanged={noop}
				onError={noop}
				onCleared={noop}
				defaultDestination=""
				defaultConflictPolicy="rename"
			/>,
		)
		expect(queue).toContain('No compatible device is available yet')
		expect(queue).toContain('There are no current transfers')
		expect(history).toContain('There is no transfer history yet')
	})

	it('renders a file review with folder roots and executable warnings', () => {
		const files = [
			{ file: new File(['hello'], 'hello.txt'), relativePath: 'project/hello.txt' },
			{ file: new File(['run'], 'setup.ps1'), relativePath: 'project/setup.ps1' },
		]
		const html = renderToStaticMarkup(
			<SendReviewModal
				files={files}
				device={deviceFixture}
				busy={false}
				onAddFiles={noop}
				onAddFolder={noop}
				onRemove={noop}
				onClear={noop}
				onCancel={noop}
				onConfirm={noop}
			/>,
		)
		expect(html).toContain('Check these files and the destination')
		expect(html).toContain('Folders included')
		expect(html).toContain('project')
		expect(html).toContain('Add more files')
		expect(html).toContain('Remove project/hello.txt')
		expect(html).toContain('executable or script files')
	})

	it('renders the backend privacy policy before network activity', () => {
		const html = renderToStaticMarkup(
			<PrivacyGate
				policy={{
					version: '2026-07-1',
					title: 'SyncSpace Privacy Policy',
					summary: 'A clear local-network summary.',
					accepted: false,
					sections: [{ title: 'Finding devices', paragraphs: ['Only on your local network.'] }],
				}}
				onAccept={async () => undefined}
			/>,
		)
		expect(html).toContain('Before SyncSpace uses your network')
		expect(html).toContain('Only on your local network')
		expect(html).toContain('Decline and keep SyncSpace paused')
	})

	it('renders diagnostics state and its initial loading state', () => {
		const overview = renderToStaticMarkup(
			<DiagnosticsOverview snapshot={diagnosticsFixture} websocketConnected={true} />,
		)
		const loading = renderToStaticMarkup(<DiagnosticsPage websocketConnected={false} />)
		expect(overview).toContain('API health')
		expect(overview).toContain('CONNECTED')
		expect(overview).toContain('RUNNING')
		expect(overview).toContain('v1')
		expect(loading).toContain('Reading runtime diagnostics')
	})
})
