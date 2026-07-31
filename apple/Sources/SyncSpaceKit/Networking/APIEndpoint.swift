import Foundation

public enum HTTPMethod: String, Sendable { case get = "GET", post = "POST", put = "PUT", patch = "PATCH", delete = "DELETE" }

public struct APIEndpoint: Hashable, Sendable {
    public var path: String; public var method: HTTPMethod
    public init(_ path: String, method: HTTPMethod = .get) { self.path = path; self.method = method }

    public static let localDevice = APIEndpoint("/device/self")
    public static let discoveredDevices = APIEndpoint("/devices")
    public static let refreshDiscovery = APIEndpoint("/discovery/refresh", method: .post)
    public static let trustedDevices = APIEndpoint("/pairing/trusted-devices")
    public static let pairingRequests = APIEndpoint("/pairing/requests")
    public static let requestPairing = APIEndpoint("/pairing/request", method: .post)
    public static let acceptPairing = APIEndpoint("/pairing/accept", method: .post)
    public static let rejectPairing = APIEndpoint("/pairing/reject", method: .post)
    public static func forgetDevice(_ id: String) -> APIEndpoint { APIEndpoint("/pairing/trusted-devices/\(id.pathComponent)", method: .delete) }
    public static let transfers = APIEndpoint("/transfers")
    public static func transferAction(_ id: String, _ action: String) -> APIEndpoint { APIEndpoint("/transfers/\(id.pathComponent)/\(action)", method: .post) }

    // TODO: Confirm these routes against the Go backend. Clipboard and note management APIs do not exist yet.
    public static let sendClipboard = APIEndpoint("/clipboard", method: .post)
    public static let notes = APIEndpoint("/notes")
    // Native Apple upload/download semantics need a backend contract; current staging endpoints are loopback browser-oriented.
    public static let createFileTransfer = APIEndpoint("/transfers", method: .post)
    public static func fileUpload(_ id: String) -> APIEndpoint { APIEndpoint("/transfers/\(id.pathComponent)/upload", method: .put) }
    public static func fileDownload(_ id: String) -> APIEndpoint { APIEndpoint("/transfers/\(id.pathComponent)/download") }
}

private extension String { var pathComponent: String { addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? self } }
