import Foundation

public enum TransferDirection: String, Codable, Sendable { case inbound, outbound }
public enum TransferStatus: String, Codable, Sendable {
    case queued = "Queued", preparing = "Preparing", transferring = "Transferring", paused = "Paused"
    case completed = "Completed", cancelled = "Cancelled", failed = "Failed"

    public init(from decoder: Decoder) throws {
        let value = try decoder.singleValueContainer().decode(String.self)
        switch value.lowercased() {
        case "queued": self = .queued
        case "preparing", "connecting", "negotiating", "verifying": self = .preparing
        case "sending", "receiving", "transferring", "resuming": self = .transferring
        case "paused": self = .paused
        case "completed": self = .completed
        case "cancelled": self = .cancelled
        default: self = .failed
        }
    }
}

public struct Transfer: Codable, Identifiable, Hashable, Sendable {
    public let id: String
    public var fileName: String
    public var fileSize: Int64
    public var contentType: String?
    public var sourceDevice: Device?
    public var destinationDevice: Device?
    public var deviceID: String?
    public var deviceName: String?
    public var direction: TransferDirection
    public var status: TransferStatus
    public var bytesTransferred: Int64
    public var transferSpeed: Double
    public var estimatedTimeRemaining: TimeInterval?
    public var createdAt: Date
    public var startedAt: Date?
    public var completedAt: Date?
    public var errorMessage: String?

    public var progress: Double { Self.clampedProgress(bytesTransferred: bytesTransferred, totalBytes: fileSize) }
    public static func clampedProgress(bytesTransferred: Int64, totalBytes: Int64) -> Double {
        guard totalBytes > 0 else { return 0 }
        return min(1, max(0, Double(bytesTransferred) / Double(totalBytes)))
    }
    public static func remainingTime(totalBytes: Int64, transferredBytes: Int64, bytesPerSecond: Double) -> TimeInterval? {
        guard bytesPerSecond > 0 else { return nil }
        return max(0, Double(max(0, totalBytes - transferredBytes)) / bytesPerSecond)
    }

    enum CodingKeys: String, CodingKey {
        case id = "uuid", fileName = "filename", fileSize = "size", contentType, sourceDevice, destinationDevice
        case deviceID = "deviceId", deviceName, direction, status, backendProgress = "progress", transferSpeed = "speed"
        case estimatedTimeRemaining = "etaSeconds", createdAt, startedAt = "startTime", completedAt = "finishTime", errorMessage = "error", bytesTransferred
    }

    public init(id: String, fileName: String, fileSize: Int64, contentType: String? = nil, sourceDevice: Device? = nil, destinationDevice: Device? = nil, deviceID: String? = nil, deviceName: String? = nil, direction: TransferDirection, status: TransferStatus, bytesTransferred: Int64 = 0, transferSpeed: Double = 0, estimatedTimeRemaining: TimeInterval? = nil, createdAt: Date = .now, startedAt: Date? = nil, completedAt: Date? = nil, errorMessage: String? = nil) {
        self.id = id; self.fileName = fileName; self.fileSize = fileSize; self.contentType = contentType
        self.sourceDevice = sourceDevice; self.destinationDevice = destinationDevice; self.deviceID = deviceID; self.deviceName = deviceName
        self.direction = direction; self.status = status; self.bytesTransferred = bytesTransferred; self.transferSpeed = transferSpeed
        self.estimatedTimeRemaining = estimatedTimeRemaining; self.createdAt = createdAt; self.startedAt = startedAt
        self.completedAt = completedAt; self.errorMessage = errorMessage
    }

    public init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id); fileName = try c.decode(String.self, forKey: .fileName)
        fileSize = try c.decode(Int64.self, forKey: .fileSize); contentType = try c.decodeIfPresent(String.self, forKey: .contentType)
        sourceDevice = try c.decodeIfPresent(Device.self, forKey: .sourceDevice); destinationDevice = try c.decodeIfPresent(Device.self, forKey: .destinationDevice)
        deviceID = try c.decodeIfPresent(String.self, forKey: .deviceID); deviceName = try c.decodeIfPresent(String.self, forKey: .deviceName)
        direction = try c.decode(TransferDirection.self, forKey: .direction); status = try c.decode(TransferStatus.self, forKey: .status)
        transferSpeed = Double(try c.decodeIfPresent(Int64.self, forKey: .transferSpeed) ?? 0)
        estimatedTimeRemaining = try c.decodeIfPresent(TimeInterval.self, forKey: .estimatedTimeRemaining)
        createdAt = try c.decodeIfPresent(Date.self, forKey: .createdAt) ?? .now; startedAt = try c.decodeIfPresent(Date.self, forKey: .startedAt)
        completedAt = try c.decodeIfPresent(Date.self, forKey: .completedAt); errorMessage = try c.decodeIfPresent(String.self, forKey: .errorMessage)
        if let exact = try c.decodeIfPresent(Int64.self, forKey: .bytesTransferred) { bytesTransferred = exact }
        else { bytesTransferred = Int64(Double(fileSize) * Double(try c.decodeIfPresent(Int64.self, forKey: .backendProgress) ?? 0) / 100) }
    }
    public func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self); try c.encode(id, forKey: .id); try c.encode(fileName, forKey: .fileName); try c.encode(fileSize, forKey: .fileSize)
        try c.encodeIfPresent(contentType, forKey: .contentType); try c.encodeIfPresent(sourceDevice, forKey: .sourceDevice); try c.encodeIfPresent(destinationDevice, forKey: .destinationDevice)
        try c.encodeIfPresent(deviceID, forKey: .deviceID); try c.encodeIfPresent(deviceName, forKey: .deviceName); try c.encode(direction, forKey: .direction); try c.encode(status, forKey: .status)
        try c.encode(Int64((progress * 100).rounded()), forKey: .backendProgress); try c.encode(Int64(transferSpeed.rounded()), forKey: .transferSpeed); try c.encodeIfPresent(estimatedTimeRemaining, forKey: .estimatedTimeRemaining)
        try c.encode(createdAt, forKey: .createdAt); try c.encodeIfPresent(startedAt, forKey: .startedAt); try c.encodeIfPresent(completedAt, forKey: .completedAt); try c.encodeIfPresent(errorMessage, forKey: .errorMessage); try c.encode(bytesTransferred, forKey: .bytesTransferred)
    }
}
