import Foundation
#if canImport(Combine)
import Combine
#endif

public enum APIConnectionState: String, Sendable { case disconnected, connecting, connected, failed }
@MainActor public final class SyncSpaceAppState: ObservableObject {
    @Published public var settings: AppSettings
    @Published public private(set) var localDevice: Device?
    @Published public private(set) var discoveredDevices: [Device] = []
    @Published public private(set) var trustedDevices: [TrustedDevice] = []
    @Published public private(set) var pairingRequests: [PairingRequest] = []
    @Published public private(set) var recentNotes: [Note] = []
    @Published public private(set) var webSocketState: WebSocketConnectionState = .disconnected
    @Published public private(set) var connectionState: APIConnectionState = .disconnected
    @Published public private(set) var lastError: AppError?
    @Published public private(set) var isInitiallyLoading = false
    public let transferManager: TransferManager
    private let devices: any DeviceServiceProtocol; private let pairing: any PairingServiceProtocol; private let notes: any NoteServiceProtocol; private let settingsStore: any SettingsStore

    public init(settings: AppSettings, devices: any DeviceServiceProtocol, pairing: any PairingServiceProtocol, notes: any NoteServiceProtocol, transferManager: TransferManager, settingsStore: any SettingsStore) { self.settings = settings; self.devices = devices; self.pairing = pairing; self.notes = notes; self.transferManager = transferManager; self.settingsStore = settingsStore }
    public func load() async { isInitiallyLoading = true; connectionState = .connecting; defer { isInitiallyLoading = false }; do { async let local = devices.localDevice(); async let nearby = devices.discoveredDevices(); async let trusted = devices.trustedDevices(); async let requests = pairing.activeRequests(); async let savedNotes = notes.recent(); localDevice = try await local; discoveredDevices = try await nearby; trustedDevices = try await trusted; pairingRequests = try await requests; recentNotes = await savedNotes; try await transferManager.refresh(); connectionState = .connected; lastError = nil } catch { connectionState = .failed; lastError = AppError(error) } }
    public func refreshDiscovery() async { do { try await devices.refreshDiscovery(); discoveredDevices = try await devices.discoveredDevices(); lastError = nil } catch { lastError = AppError(error, title: "Discovery failed") } }
    public func saveSettings() { do { settings = try settings.normalized(); try settingsStore.save(settings) } catch { lastError = AppError(error, title: "Settings are invalid") } }
    public func dismissError() { lastError = nil }
}
