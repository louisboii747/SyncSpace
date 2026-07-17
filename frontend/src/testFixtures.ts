import type { Device, Diagnostics, Transfer, TrustedDevice } from './types'

export const deviceFixture: Device = {
	deviceId: '11111111-1111-4111-8111-111111111111', deviceName: 'Studio Laptop', deviceType: 'desktop', platform: 'linux', localIp: '127.0.0.1', port: 8385, appVersion: 'dev', lastSeen: '2026-07-06T12:00:00Z', online: true, connectionState: 'online', availableStorage: 1_000_000, transferCapability: true, supportedProtocolVersion: 1, maximumChunkSize: 4_194_304, compressionSupport: true, identityHint: 'AAAA:BBBB:CCCC:DDDD', pairingAvailable: true,
}

export const trustedFixture: TrustedDevice = { deviceId: deviceFixture.deviceId, deviceName: deviceFixture.deviceName, platform: deviceFixture.platform, pairedAt: '2026-07-06T12:00:00Z', lastSeen: '2026-07-06T12:00:00Z', trustState: 'trusted', publicKey: 'test', fingerprint: 'AAAA:BBBB:CCCC:DDDD', blocked: false, identityKeyChanged: false }

export function transferFixture(status: Transfer['status'] = 'Sending', progress = 50): Transfer {
  return { uuid: '22222222-2222-4222-8222-222222222222', direction: 'outbound', deviceId: deviceFixture.deviceId, deviceName: deviceFixture.deviceName, filename: 'demo.bin', size: 100, status, progress, speed: 25, etaSeconds: 2, createdAt: '2026-07-06T12:00:00Z', updatedAt: '2026-07-06T12:00:01Z', attempts: 0, approved: true, approvalRequired: false, conflictPolicy: 'rename', compression: true, protocolVersion: 1 }
}

export const diagnosticsFixture: Diagnostics = {
  health: { status: 'ok', checkedAt: '2026-07-06T12:00:00Z', checks: { database: { ok: true, detail: 'SQLite is reachable' } } },
  localDevice: { ...deviceFixture, deviceId: '00000000-0000-4000-8000-00000000000a', deviceName: 'SyncSpace Device A', port: 8384 }, platform: 'windows', backendUrl: 'http://127.0.0.1:8384', webSockets: { discovery: 0, pairing: 0, transfers: 1 }, discoveryRunning: true, knownDevices: [deviceFixture], trustedDevices: [trustedFixture], activeTransfers: [transferFixture()], transferQueue: [transferFixture('Queued', 0)], storagePath: 'C:\\syncspace\\transfers', databasePath: 'C:\\syncspace\\syncspace.db', protocolVersion: 1, developerMode: true, logs: [], lastErrors: [],
}
