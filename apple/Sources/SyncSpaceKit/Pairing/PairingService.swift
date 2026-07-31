import Foundation

public enum PairingError: LocalizedError, Sendable { case rejected, expired, deviceOffline, backend(String)
    public var errorDescription: String? { switch self { case .rejected: "The pairing request was rejected."; case .expired: "The pairing request expired."; case .deviceOffline: "The device is offline."; case let .backend(value): value } }
}
public protocol PairingServiceProtocol: Sendable {
    func request(deviceID: String) async throws -> PairingRequest
    func accept(requestID: String) async throws -> PairingRequest
    func reject(requestID: String) async throws -> PairingRequest
    func cancel(requestID: String) async throws
    func activeRequests() async throws -> [PairingRequest]
    func trustedDevices() async throws -> [TrustedDevice]
}
public struct PairingService: PairingServiceProtocol {
    private struct DeviceBody: Codable, Sendable { let deviceId: String }; private struct RequestBody: Codable, Sendable { let requestId: String }
    private struct Decision: Codable, Sendable { let request: PairingRequest }
    private let api: any APIClientProtocol; public init(api: any APIClientProtocol) { self.api = api }
    public func request(deviceID: String) async throws -> PairingRequest { try await api.send(.requestPairing, body: DeviceBody(deviceId: deviceID), response: PairingRequest.self) }
    public func accept(requestID: String) async throws -> PairingRequest { try await api.send(.acceptPairing, body: RequestBody(requestId: requestID), response: Decision.self).request }
    public func reject(requestID: String) async throws -> PairingRequest { try await api.send(.rejectPairing, body: RequestBody(requestId: requestID), response: PairingRequest.self) }
    public func cancel(requestID: String) async throws { throw PairingError.backend("The Go backend does not currently expose a local pairing-cancel route.") }
    public func activeRequests() async throws -> [PairingRequest] { try await api.send(.pairingRequests, response: [PairingRequest].self) }
    public func trustedDevices() async throws -> [TrustedDevice] { try await api.send(.trustedDevices, response: [TrustedDevice].self) }
}
