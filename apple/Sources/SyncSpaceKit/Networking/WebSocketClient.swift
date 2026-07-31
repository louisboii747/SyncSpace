import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

public enum WebSocketConnectionState: String, Codable, Sendable { case disconnected, connecting, connected, reconnecting }
public enum WebSocketMessage: Sendable { case text(String), binary(Data) }
public protocol WebSocketClientProtocol: AnyObject, Sendable {
    var messages: AsyncStream<WebSocketMessage> { get }
    var states: AsyncStream<WebSocketConnectionState> { get }
    func connect() async; func disconnect() async; func ping() async throws
}

public final class WebSocketClient: WebSocketClientProtocol, @unchecked Sendable {
    public let messages: AsyncStream<WebSocketMessage>; public let states: AsyncStream<WebSocketConnectionState>
    private let url: URL; private let session: URLSession; private var task: URLSessionWebSocketTask?
    private var receiveTask: Task<Void, Never>?; private var shouldReconnect = false; private var attempts = 0
    private let messageContinuation: AsyncStream<WebSocketMessage>.Continuation
    private let stateContinuation: AsyncStream<WebSocketConnectionState>.Continuation
    private let lock = NSLock()
    public init(url: URL, session: URLSession = .shared) {
        self.url = url; self.session = session
        var mc: AsyncStream<WebSocketMessage>.Continuation!; messages = AsyncStream { mc = $0 }; messageContinuation = mc
        var sc: AsyncStream<WebSocketConnectionState>.Continuation!; states = AsyncStream { sc = $0 }; stateContinuation = sc
    }
    public func connect() async {
        let socket: URLSessionWebSocketTask? = lock.withLock { guard task == nil else { return nil }; shouldReconnect = true; let value = session.webSocketTask(with: url); task = value; return value }
        guard let socket else { return }; stateContinuation.yield(attempts == 0 ? .connecting : .reconnecting)
        socket.resume(); stateContinuation.yield(.connected)
        receiveTask = Task { [weak self] in await self?.receiveLoop(socket) }
    }
    public func disconnect() async {
        let socket: URLSessionWebSocketTask? = lock.withLock { shouldReconnect = false; let value = task; task = nil; return value }
        receiveTask?.cancel(); socket?.cancel(with: .goingAway, reason: nil); stateContinuation.yield(.disconnected)
    }
    public func ping() async throws { try await task?.sendPing() }
    private func receiveLoop(_ socket: URLSessionWebSocketTask) async {
        do {
            while !Task.isCancelled {
                let message = try await socket.receive(); attempts = 0
                switch message { case let .string(value): messageContinuation.yield(.text(value)); case let .data(value): messageContinuation.yield(.binary(value)); @unknown default: break }
            }
        } catch { await connectionEnded(socket) }
    }
    private func connectionEnded(_ socket: URLSessionWebSocketTask) async {
        let reconnect = lock.withLock { if task === socket { task = nil }; return shouldReconnect }
        guard reconnect, !Task.isCancelled else { stateContinuation.yield(.disconnected); return }
        attempts += 1; let delay = min(30.0, pow(2.0, Double(min(attempts, 5))))
        stateContinuation.yield(.reconnecting); try? await Task.sleep(for: .seconds(delay)); await connect()
    }
}

public struct WebSocketEventEnvelope: Decodable, Sendable {
    public let type: String; public let timestamp: Date?; public let payload: JSONValue?
    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: DynamicKey.self); type = try c.decode(String.self, forKey: DynamicKey("type")); timestamp = try c.decodeIfPresent(Date.self, forKey: DynamicKey("timestamp"))
        let known = ["device", "request", "trustedDevice", "transfer", "payload"].compactMap { key in try? c.decodeIfPresent(JSONValue.self, forKey: DynamicKey(key)) }
        payload = known.first ?? nil
    }
}
public enum JSONValue: Codable, Sendable { case string(String), number(Double), bool(Bool), object([String: JSONValue]), array([JSONValue]), null
    public init(from decoder: Decoder) throws { let c = try decoder.singleValueContainer(); if c.decodeNil() { self = .null } else if let v = try? c.decode(Bool.self) { self = .bool(v) } else if let v = try? c.decode(Double.self) { self = .number(v) } else if let v = try? c.decode(String.self) { self = .string(v) } else if let v = try? c.decode([String: JSONValue].self) { self = .object(v) } else { self = .array(try c.decode([JSONValue].self)) } }
    public func encode(to encoder: Encoder) throws { var c = encoder.singleValueContainer(); switch self { case let .string(v): try c.encode(v); case let .number(v): try c.encode(v); case let .bool(v): try c.encode(v); case let .object(v): try c.encode(v); case let .array(v): try c.encode(v); case .null: try c.encodeNil() } }
}
private struct DynamicKey: CodingKey { let stringValue: String; let intValue: Int? = nil; init(_ value: String) { stringValue = value }; init?(stringValue: String) { self.init(stringValue) }; init?(intValue: Int) { return nil } }
