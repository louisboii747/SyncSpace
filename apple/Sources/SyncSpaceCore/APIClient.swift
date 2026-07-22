import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

public enum APIError: LocalizedError, Sendable {
    case invalidResponse
    case rejected(status: Int, message: String)

    public var errorDescription: String? {
        switch self {
        case .invalidResponse: return "SyncSpace returned an invalid response."
        case let .rejected(_, message): return message
        }
    }
}

public actor SyncSpaceAPIClient {
    private let baseURL: URL
    private let session: URLSession
    private let decoder = JSONDecoder()

    public init(baseURL: URL = URL(string: "http://127.0.0.1:8384/api/v1")!, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    public func snapshot() async throws -> AppSnapshot {
        async let local: Device = get("device/self")
        async let devices: [Device] = get("devices")
        async let transfers: [Transfer] = get("transfers")
        async let policy: PrivacyPolicy = get("privacy-policy")
        return try await AppSnapshot(localDevice: local, devices: devices, transfers: transfers, policy: policy)
    }

    public func acceptPrivacyPolicy(version: String) async throws -> PrivacyPolicy {
        try await send("privacy-policy/accept", method: "POST", body: ["version": version])
    }

    public func refreshDiscovery() async throws {
        let _: EmptyResponse = try await send("discovery/refresh", method: "POST", body: Optional<String>.none)
    }

    private func get<T: Decodable>(_ path: String) async throws -> T {
        try await send(path, method: "GET", body: Optional<String>.none)
    }

    private func send<T: Decodable, Body: Encodable>(_ path: String, method: String, body: Body?) async throws -> T {
        var request = URLRequest(url: baseURL.appendingPathComponent(path))
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let body { request.httpBody = try JSONEncoder().encode(body) }
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw APIError.invalidResponse }
        guard (200..<300).contains(http.statusCode) else {
            let message = (try? decoder.decode(ErrorEnvelope.self, from: data).error) ?? "Request failed (\(http.statusCode))."
            throw APIError.rejected(status: http.statusCode, message: message)
        }
        if T.self == EmptyResponse.self { return EmptyResponse() as! T }
        return try decoder.decode(T.self, from: data)
    }
}

private struct ErrorEnvelope: Decodable { let error: String }
private struct EmptyResponse: Codable { init() {} }
