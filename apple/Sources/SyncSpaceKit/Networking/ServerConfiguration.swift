import Foundation

public enum ServerConfigurationError: LocalizedError, Equatable {
    case emptyHost, invalidHost, invalidPort
    public var errorDescription: String? {
        switch self { case .emptyHost: "Enter a server address."; case .invalidHost: "Enter a hostname or IP address without a URL scheme."; case .invalidPort: "Port must be between 1 and 65535." }
    }
}

public struct ServerConfiguration: Codable, Hashable, Sendable {
    public var host: String; public var port: Int; public var useHTTPS: Bool
    public var apiBasePath: String; public var webSocketPath: String?

    public init(host: String = "127.0.0.1", port: Int = 8384, useHTTPS: Bool = false, apiBasePath: String = "/api/v1", webSocketPath: String? = nil) {
        self.host = host; self.port = port; self.useHTTPS = useHTTPS; self.apiBasePath = apiBasePath; self.webSocketPath = webSocketPath
    }

    public func validated() throws -> ServerConfiguration {
        let normalized = host.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalized.isEmpty else { throw ServerConfigurationError.emptyHost }
        guard !(normalized.lowercased().hasPrefix("http://") || normalized.lowercased().hasPrefix("https://")), !normalized.contains("/") else { throw ServerConfigurationError.invalidHost }
        guard (1...65535).contains(port) else { throw ServerConfigurationError.invalidPort }
        var copy = self; copy.host = normalized.trimmingCharacters(in: CharacterSet(charactersIn: "[]")); return copy
    }

    public var httpBaseURL: URL? { buildURL(scheme: useHTTPS ? "https" : "http", path: apiBasePath) }
    public func webSocketURL(path: String? = nil) -> URL? { buildURL(scheme: useHTTPS ? "wss" : "ws", path: path ?? webSocketPath ?? "/ws/discovery") }

    private func buildURL(scheme: String, path: String) -> URL? {
        let cleanHost = host.trimmingCharacters(in: CharacterSet(charactersIn: "[]")); let normalizedPath = path.isEmpty ? "" : (path.hasPrefix("/") ? path : "/" + path)
        if cleanHost.contains(":") { return URL(string: "\(scheme)://[\(cleanHost)]:\(port)\(normalizedPath)") }
        var components = URLComponents(); components.scheme = scheme; components.host = cleanHost; components.port = port; components.path = normalizedPath; return components.url
    }
}
