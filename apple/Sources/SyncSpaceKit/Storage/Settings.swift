import Foundation

public struct AppSettings: Codable, Equatable, Sendable {
    public var serverHost = "127.0.0.1"; public var serverPort = 8384; public var useHTTPS = false
    public var autoConnect = true; public var webSocketUpdates = true; public var showNotifications = true
    public var saveTransferHistory = true; public var deviceNameOverride = ""; public var debugLogging = false
    public init() {}
    public func normalized() throws -> AppSettings { var copy = self; let config = try ServerConfiguration(host: serverHost, port: serverPort, useHTTPS: useHTTPS).validated(); copy.serverHost = config.host; return copy }
    public var serverConfiguration: ServerConfiguration { ServerConfiguration(host: serverHost, port: serverPort, useHTTPS: useHTTPS) }
}
public protocol SettingsStore: Sendable { func load() -> AppSettings; func save(_ settings: AppSettings) throws }
public final class UserDefaultsSettingsStore: SettingsStore, @unchecked Sendable {
    private let defaults: UserDefaults; private let key = "SyncSpace.AppSettings"
    public init(defaults: UserDefaults = .standard) { self.defaults = defaults }
    public func load() -> AppSettings { guard let data = defaults.data(forKey: key), let value = try? JSONDecoder().decode(AppSettings.self, from: data) else { return AppSettings() }; return value }
    public func save(_ settings: AppSettings) throws { defaults.set(try JSONEncoder().encode(settings.normalized()), forKey: key) }
}
public protocol TrustedDeviceCache: Sendable { func load() async -> [TrustedDevice]; func save(_ devices: [TrustedDevice]) async }
public actor InMemoryTrustedDeviceCache: TrustedDeviceCache { private var values: [TrustedDevice] = []; public init() {}; public func load() -> [TrustedDevice] { values }; public func save(_ devices: [TrustedDevice]) { values = devices } }
