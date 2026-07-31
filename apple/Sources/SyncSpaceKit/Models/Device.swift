import Foundation

public enum DeviceCapability: String, Codable, CaseIterable, Sendable {
    case clipboard, notes, files, pairing, webSocket, notifications
}

public struct Device: Codable, Identifiable, Hashable, Sendable {
    public let id: String
    public var name: String
    public var deviceType: String
    public var platform: String
    public var host: String
    public var port: Int
    public var version: String
    public var isOnline: Bool
    public var isTrusted: Bool
    public var lastSeen: Date?
    public var capabilities: Set<DeviceCapability>?

    public init(id: String, name: String, deviceType: String = "computer", platform: String, host: String, port: Int = 8384, version: String = "", isOnline: Bool = true, isTrusted: Bool = false, lastSeen: Date? = nil, capabilities: Set<DeviceCapability>? = nil) {
        self.id = id; self.name = name; self.deviceType = deviceType; self.platform = platform
        self.host = host; self.port = port; self.version = version; self.isOnline = isOnline
        self.isTrusted = isTrusted; self.lastSeen = lastSeen; self.capabilities = capabilities
    }

    enum CodingKeys: String, CodingKey {
        case id = "deviceId", name = "deviceName", deviceType, platform, host = "localIp", port
        case version = "appVersion", isOnline = "online", isTrusted, lastSeen, capabilities
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        name = try c.decode(String.self, forKey: .name)
        deviceType = try c.decodeIfPresent(String.self, forKey: .deviceType) ?? "unknown"
        platform = try c.decodeIfPresent(String.self, forKey: .platform) ?? "unknown"
        host = try c.decodeIfPresent(String.self, forKey: .host) ?? ""
        port = try c.decodeIfPresent(Int.self, forKey: .port) ?? SyncSpaceKit.defaultPort
        version = try c.decodeIfPresent(String.self, forKey: .version) ?? ""
        isOnline = try c.decodeIfPresent(Bool.self, forKey: .isOnline) ?? true
        isTrusted = try c.decodeIfPresent(Bool.self, forKey: .isTrusted) ?? false
        lastSeen = try c.decodeIfPresent(Date.self, forKey: .lastSeen)
        capabilities = try c.decodeIfPresent(Set<DeviceCapability>.self, forKey: .capabilities)
    }
}
