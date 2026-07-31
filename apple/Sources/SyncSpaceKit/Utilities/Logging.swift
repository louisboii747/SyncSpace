import Foundation

public enum LogCategory: String, Sendable { case networking, discovery, pairing, transfers, clipboard, storage, webSocket, uiState }
public protocol SyncSpaceLogging: Sendable { func log(_ message: @autoclosure () -> String, category: LogCategory, isSensitive: Bool) }
public struct SyncSpaceLogger: SyncSpaceLogging {
    public var debugEnabled: Bool; public init(debugEnabled: Bool = false) { self.debugEnabled = debugEnabled }
    public func log(_ message: @autoclosure () -> String, category: LogCategory, isSensitive: Bool = false) { guard debugEnabled, !isSensitive else { return }; print("[SyncSpace][\(category.rawValue)] \(message())") }
}
