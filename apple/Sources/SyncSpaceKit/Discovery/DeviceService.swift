import Foundation

public protocol DeviceServiceProtocol: Sendable {
    func localDevice() async throws -> Device
    func discoveredDevices() async throws -> [Device]
    func refreshDiscovery() async throws
    func trustedDevices() async throws -> [TrustedDevice]
    func forget(deviceID: String) async throws
}
public struct DeviceService: DeviceServiceProtocol {
    private let api: any APIClientProtocol
    public init(api: any APIClientProtocol) { self.api = api }
    public func localDevice() async throws -> Device { try await api.send(.localDevice, response: Device.self) }
    public func discoveredDevices() async throws -> [Device] { try await api.send(.discoveredDevices, response: [Device].self) }
    public func refreshDiscovery() async throws { try await api.send(.refreshDiscovery) }
    public func trustedDevices() async throws -> [TrustedDevice] { try await api.send(.trustedDevices, response: [TrustedDevice].self) }
    public func forget(deviceID: String) async throws { try await api.send(.forgetDevice(deviceID)) }
}

public protocol DiscoveryServiceProtocol: Sendable { func refresh() async throws -> [Device] }
public struct BackendDiscoveryService: DiscoveryServiceProtocol {
    private let devices: any DeviceServiceProtocol
    public init(devices: any DeviceServiceProtocol) { self.devices = devices }
    public func refresh() async throws -> [Device] { try await devices.refreshDiscovery(); return try await devices.discoveredDevices() }
}
public struct NativeBonjourDiscoveryService: DiscoveryServiceProtocol {
    public init() {}
    public func refresh() async throws -> [Device] { throw DiscoveryError.nativeBonjourNotConfigured }
}
public enum DiscoveryError: LocalizedError { case nativeBonjourNotConfigured
    public var errorDescription: String? { "Native Bonjour discovery is not configured. Use backend discovery until NWBrowser is implemented and tested on Apple hardware." }
}
