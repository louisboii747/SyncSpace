import { renderToStaticMarkup } from 'react-dom/server'
import { expect, it, vi } from 'vitest'

vi.mock('./hooks/useSyncSpace', () => ({
	useSyncSpace: () => ({
		localDevice: null, devices: [], trusted: [], trustedIDs: new Set<string>(), pairingRequests: [], settings: null, privacyPolicy: null, transfers: [], active: [], history: [], loading: true, connected: false, error: '', toasts: [], celebration: 0,
		toast: vi.fn(), reload: vi.fn(), setLocalDevice: vi.fn(), setTrusted: vi.fn(), setPairingRequests: vi.fn(), setSettings: vi.fn(), setPrivacyPolicy: vi.fn(), setTransfers: vi.fn(),
  }),
}))

import App from './App'

it('renders the backend loading state', () => {
  const html = renderToStaticMarkup(<App/>)
  expect(html).toContain('Starting SyncSpace on this device')
  expect(html).toContain('aria-busy="true"')
})
