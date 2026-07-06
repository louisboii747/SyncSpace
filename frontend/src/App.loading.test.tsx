import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it, vi } from 'vitest'

vi.mock('./hooks/useSyncSpace', () => ({
  useSyncSpace: () => ({
    devices: [], trusted: [], trustedIDs: new Set<string>(), transfers: [], active: [], history: [], loading: true, connected: false, error: '', toasts: [], celebration: 0,
    toast: vi.fn(), reload: vi.fn(), setTrusted: vi.fn(), setTransfers: vi.fn(),
  }),
}))

import App from './App'

it('renders the backend loading state', () => {
  const html = renderToStaticMarkup(<App/>)
  expect(html).toContain('Connecting to the local SyncSpace backend')
  expect(html).toContain('aria-busy="true"')
})
