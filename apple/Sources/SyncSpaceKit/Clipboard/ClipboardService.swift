import Foundation

public protocol PlatformClipboardProviding: Sendable { func readText() async -> String?; func writeText(_ text: String) async throws }
public protocol ClipboardServiceProtocol: Sendable { func send(text: String, to deviceID: String) async throws }
public struct SharedClipboardService: ClipboardServiceProtocol {
    private struct Body: Codable, Sendable { let deviceId: String; let text: String }
    private let api: any APIClientProtocol; public init(api: any APIClientProtocol) { self.api = api }
    public func send(text: String, to deviceID: String) async throws { let _: Acknowledgement = try await api.send(.sendClipboard, body: Body(deviceId: deviceID, text: text), response: Acknowledgement.self) }
}
public struct Acknowledgement: Codable, Sendable { public var status: String?; public init(status: String? = nil) { self.status = status } }
