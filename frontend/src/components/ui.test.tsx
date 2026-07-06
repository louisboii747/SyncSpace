import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { DevicesView, HistoryView, TransferView } from '../App'
import { deviceFixture, transferFixture } from '../testFixtures'
import { DiagnosticsOverview, DiagnosticsPage } from './DiagnosticsPage'
import { TransferCard } from './TransferCard'
import { diagnosticsFixture } from '../testFixtures'

const noop = vi.fn()

describe('SyncSpace React views', () => {
  it('renders discovered devices and their trust state', () => {
    const html = renderToStaticMarkup(<DevicesView devices={[deviceFixture]} trustedIDs={new Set()} onSelect={noop} onPair={noop}/>)
    expect(html).toContain('Studio Laptop')
    expect(html).toContain('Approval needed')
    expect(html).toContain('Trust device')
  })

  it('renders transfer queue progress updates', () => {
    const first = renderToStaticMarkup(<TransferCard transfer={transferFixture('Sending', 20)} onChanged={noop} onError={noop}/>)
    const second = renderToStaticMarkup(<TransferCard transfer={transferFixture('Sending', 80)} onChanged={noop} onError={noop}/>)
    expect(first).toContain('aria-valuenow="20"')
    expect(second).toContain('aria-valuenow="80"')
  })

  it('renders failed and completed transfer states', () => {
    const failed = renderToStaticMarkup(<TransferCard transfer={{ ...transferFixture('Failed'), error: 'checksum mismatch' }} onChanged={noop} onError={noop}/>)
    const completed = renderToStaticMarkup(<TransferCard transfer={{ ...transferFixture('Completed', 100), speed: 0 }} onChanged={noop} onError={noop}/>)
    expect(failed).toContain('checksum mismatch')
    expect(failed).toContain('Retry transfer')
    expect(completed).toContain('Verified and complete')
    expect(completed).toContain('aria-valuenow="100"')
  })

  it('renders queue and history empty states', () => {
    const queue = renderToStaticMarkup(<TransferView devices={[]} trustedIDs={new Set()} selectedDevice="" onSelect={noop} conflictPolicy="rename" onConflictPolicy={noop} upload={{ active: false, completedBytes: 0, totalBytes: 0, currentName: '', completedFiles: 0, totalFiles: 0 }} active={[]} activeCount={0} totalSpeed={0} onBrowseFiles={noop} onBrowseFolder={noop} onPair={noop} onChanged={noop} onError={noop}/>)
    const history = renderToStaticMarkup(<HistoryView transfers={[]} onChanged={noop} onError={noop} onCleared={noop}/>)
    expect(queue).toContain('Looking for SyncSpace devices')
    expect(queue).toContain('Nothing in flight')
    expect(history).toContain('Your history is clear')
  })

  it('renders diagnostics state and its initial loading state', () => {
    const overview = renderToStaticMarkup(<DiagnosticsOverview snapshot={diagnosticsFixture} websocketConnected={true}/>)
    const loading = renderToStaticMarkup(<DiagnosticsPage websocketConnected={false}/>)
    expect(overview).toContain('API health')
    expect(overview).toContain('CONNECTED')
    expect(overview).toContain('RUNNING')
    expect(overview).toContain('v1')
    expect(loading).toContain('Reading runtime diagnostics')
  })
})
