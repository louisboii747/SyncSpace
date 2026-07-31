import Foundation

public struct Note: Codable, Identifiable, Hashable, Sendable {
    public let id: String; public var title: String; public var content: String
    public var senderDevice: Device?; public var recipientDevice: Device?
    public var createdAt: Date; public var receivedAt: Date?; public var isRead: Bool
}

public enum ClipboardContentType: String, Codable, Sendable { case plainText, url, richText, image }
public struct ClipboardItem: Codable, Identifiable, Hashable, Sendable {
    public let id: String; public var type: ClipboardContentType; public var text: String?
    public var metadata: [String: String]?; public var senderDevice: Device?; public var createdAt: Date
}
