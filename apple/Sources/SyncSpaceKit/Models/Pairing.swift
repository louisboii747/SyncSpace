import Foundation

public enum PairingState: String, Codable, Sendable {
    case notPaired, requesting, awaitingConfirmation, paired, rejected, expired, failed

    public init(backendValue: String) {
        switch backendValue.lowercased() {
        case "pending": self = .awaitingConfirmation
        case "confirming": self = .requesting
        case "paired": self = .paired
        case "rejected": self = .rejected
        case "expired": self = .expired
        default: self = .failed
        }
    }
}

public struct PairingRequest: Codable, Identifiable, Hashable, Sendable {
    public let id: String
    public var sourceDevice: Device?
    public var destinationDevice: Device?
    public var deviceID: String?
    public var deviceName: String?
    public var verificationCode: String?
    public var createdAt: Date?
    public var expiresAt: Date?
    public var state: PairingState
    public var direction: String?

    enum CodingKeys: String, CodingKey { case id = "requestId", sourceDevice, destinationDevice, deviceID = "deviceId", deviceName, verificationCode, createdAt = "requestedAt", expiresAt, state, direction }
    public init(id: String, sourceDevice: Device? = nil, destinationDevice: Device? = nil, deviceID: String? = nil, deviceName: String? = nil, verificationCode: String? = nil, createdAt: Date? = nil, expiresAt: Date? = nil, state: PairingState, direction: String? = nil) { self.id = id; self.sourceDevice = sourceDevice; self.destinationDevice = destinationDevice; self.deviceID = deviceID; self.deviceName = deviceName; self.verificationCode = verificationCode; self.createdAt = createdAt; self.expiresAt = expiresAt; self.state = state; self.direction = direction }
    public init(from decoder: Decoder) throws { let c = try decoder.container(keyedBy: CodingKeys.self); id = try c.decode(String.self, forKey: .id); sourceDevice = try c.decodeIfPresent(Device.self, forKey: .sourceDevice); destinationDevice = try c.decodeIfPresent(Device.self, forKey: .destinationDevice); deviceID = try c.decodeIfPresent(String.self, forKey: .deviceID); deviceName = try c.decodeIfPresent(String.self, forKey: .deviceName); verificationCode = try c.decodeIfPresent(String.self, forKey: .verificationCode); createdAt = try c.decodeIfPresent(Date.self, forKey: .createdAt); expiresAt = try c.decodeIfPresent(Date.self, forKey: .expiresAt); state = PairingState(backendValue: try c.decode(String.self, forKey: .state)); direction = try c.decodeIfPresent(String.self, forKey: .direction) }
    public func encode(to encoder: Encoder) throws { var c = encoder.container(keyedBy: CodingKeys.self); try c.encode(id, forKey: .id); try c.encodeIfPresent(sourceDevice, forKey: .sourceDevice); try c.encodeIfPresent(destinationDevice, forKey: .destinationDevice); try c.encodeIfPresent(deviceID, forKey: .deviceID); try c.encodeIfPresent(deviceName, forKey: .deviceName); try c.encodeIfPresent(verificationCode, forKey: .verificationCode); try c.encodeIfPresent(createdAt, forKey: .createdAt); try c.encodeIfPresent(expiresAt, forKey: .expiresAt); try c.encode(state.rawValue, forKey: .state); try c.encodeIfPresent(direction, forKey: .direction) }
}

public struct TrustedDevice: Codable, Identifiable, Hashable, Sendable {
    public let id: String
    public var name: String
    public var platform: String
    public var fingerprint: String?
    public var pairedAt: Date?
    public var lastSeen: Date?
    public var blocked: Bool
    enum CodingKeys: String, CodingKey { case id = "deviceId", name = "deviceName", platform, fingerprint, pairedAt, lastSeen, blocked }
}
