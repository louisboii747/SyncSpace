import Foundation

public struct Device: Codable, Identifiable, Hashable, Sendable {
    public let id: String
    public var name: String
    public var hostname: String?
    public var addresses: [String]?
    public var port: Int?
    public var appVersion: String?
    public var online: Bool?

    public init(id: String, name: String, hostname: String? = nil, addresses: [String]? = nil, port: Int? = nil, appVersion: String? = nil, online: Bool? = nil) {
        self.id = id
        self.name = name
        self.hostname = hostname
        self.addresses = addresses
        self.port = port
        self.appVersion = appVersion
        self.online = online
    }
}

public struct PrivacyPolicy: Codable, Sendable {
    public let version: String
    public let accepted: Bool
    public let title: String?
    public let summary: String?
}

public struct Transfer: Codable, Identifiable, Hashable, Sendable {
    public let id: String
    public let deviceId: String?
    public let direction: String?
    public let state: String
    public let totalBytes: Int64?
    public let transferredBytes: Int64?
    public let createdAt: String?
}

public struct AppSnapshot: Sendable {
    public var localDevice: Device?
    public var devices: [Device]
    public var transfers: [Transfer]
    public var policy: PrivacyPolicy?

    public init(localDevice: Device? = nil, devices: [Device] = [], transfers: [Transfer] = [], policy: PrivacyPolicy? = nil) {
        self.localDevice = localDevice
        self.devices = devices
        self.transfers = transfers
        self.policy = policy
    }
}
