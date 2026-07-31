import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

public struct BackendErrorBody: Codable, Sendable { public let error: String?; public let message: String? }
public enum APIError: LocalizedError, Equatable, Sendable {
    case invalidURL, transport(String), timedOut, cancelled, invalidResponse, httpStatus(Int, String?), decoding(String), encoding(String), unsupportedBackendVersion
    public var errorDescription: String? {
        switch self {
        case .invalidURL: "The server address is invalid."
        case .transport: "SyncSpace could not reach the backend."
        case .timedOut: "The backend took too long to respond."
        case .cancelled: "The request was cancelled."
        case .invalidResponse, .decoding: "The backend returned an unexpected response."
        case let .httpStatus(code, message): message ?? "The backend returned HTTP \(code)."
        case .encoding: "The request could not be prepared."
        case .unsupportedBackendVersion: "This SyncSpace backend version is not supported."
        }
    }
    public var recoverySuggestion: String? {
        switch self { case .timedOut, .transport: "Check the address, LAN connection, backend listener, and firewall, then retry."; case .httpStatus(403, _): "The current Go management API is loopback-only; a remote Apple client needs an authenticated LAN API boundary."; default: nil }
    }
}

public protocol APIClientProtocol: Sendable {
    func send<Response: Decodable & Sendable>(_ endpoint: APIEndpoint, response: Response.Type) async throws -> Response
    func send<Body: Encodable & Sendable, Response: Decodable & Sendable>(_ endpoint: APIEndpoint, body: Body, response: Response.Type) async throws -> Response
    func send(_ endpoint: APIEndpoint) async throws
}

public final class APIClient: APIClientProtocol, @unchecked Sendable {
    private let configuration: ServerConfiguration; private let session: URLSession
    private let encoder: JSONEncoder; private let decoder: JSONDecoder
    public init(configuration: ServerConfiguration, timeout: TimeInterval = 15, session: URLSession? = nil) {
        self.configuration = configuration
        let config = URLSessionConfiguration.default; config.timeoutIntervalForRequest = timeout; config.timeoutIntervalForResource = timeout * 2
        self.session = session ?? URLSession(configuration: config)
        encoder = JSONEncoder(); encoder.dateEncodingStrategy = .iso8601
        decoder = JSONDecoder(); decoder.dateDecodingStrategy = .iso8601
    }
    public func send<Response: Decodable & Sendable>(_ endpoint: APIEndpoint, response: Response.Type) async throws -> Response { try await execute(endpoint, body: nil, response: response) }
    public func send<Body: Encodable & Sendable, Response: Decodable & Sendable>(_ endpoint: APIEndpoint, body: Body, response: Response.Type) async throws -> Response {
        let data: Data; do { data = try encoder.encode(body) } catch { throw APIError.encoding(error.localizedDescription) }
        return try await execute(endpoint, body: data, response: response)
    }
    public func send(_ endpoint: APIEndpoint) async throws { let _: EmptyResponse = try await execute(endpoint, body: nil, response: EmptyResponse.self) }

    private func execute<Response: Decodable & Sendable>(_ endpoint: APIEndpoint, body: Data?, response: Response.Type) async throws -> Response {
        guard let base = configuration.httpBaseURL, let url = URL(string: endpoint.path.trimmingCharacters(in: CharacterSet(charactersIn: "/")), relativeTo: base.appendingPathComponent(""))?.absoluteURL else { throw APIError.invalidURL }
        var request = URLRequest(url: url); request.httpMethod = endpoint.method.rawValue; request.httpBody = body
        request.setValue("application/json", forHTTPHeaderField: "Accept"); if body != nil { request.setValue("application/json", forHTTPHeaderField: "Content-Type") }
        do {
            let (data, rawResponse) = try await session.data(for: request)
            guard let http = rawResponse as? HTTPURLResponse else { throw APIError.invalidResponse }
            guard (200..<300).contains(http.statusCode) else { throw Self.mapHTTPError(status: http.statusCode, data: data, decoder: decoder) }
            let responseData = data.isEmpty ? Data("{}".utf8) : data
            do { return try decoder.decode(Response.self, from: responseData) } catch { throw APIError.decoding(error.localizedDescription) }
        } catch is CancellationError { throw APIError.cancelled }
        catch let error as APIError { throw error }
        catch let error as URLError where error.code == .timedOut { throw APIError.timedOut }
        catch { throw APIError.transport(error.localizedDescription) }
    }
    public static func mapHTTPError(status: Int, data: Data, decoder: JSONDecoder = JSONDecoder()) -> APIError {
        let body = try? decoder.decode(BackendErrorBody.self, from: data); return .httpStatus(status, body?.error ?? body?.message)
    }
}
private struct EmptyResponse: Codable, Sendable { init() {} }
