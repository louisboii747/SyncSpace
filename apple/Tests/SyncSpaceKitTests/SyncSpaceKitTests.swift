import XCTest
@testable import SyncSpaceKit

final class SyncSpaceKitTests: XCTestCase {
    func testDeviceBackendFieldMapping() throws {
        let data = Data(#"{"deviceId":"abc","deviceName":"Studio","deviceType":"desktop","platform":"windows","localIp":"192.168.1.8","port":8384,"appVersion":"1.0.3","online":true}"#.utf8)
        let device = try JSONDecoder().decode(Device.self, from: data)
        XCTAssertEqual(device.id, "abc"); XCTAssertEqual(device.name, "Studio"); XCTAssertEqual(device.host, "192.168.1.8"); XCTAssertEqual(device.version, "1.0.3")
    }
    func testIPv4BaseURL() { XCTAssertEqual(ServerConfiguration(host: "192.168.1.10").httpBaseURL?.absoluteString, "http://192.168.1.10:8384/api/v1") }
    func testIPv6BaseURL() { XCTAssertEqual(ServerConfiguration(host: "2001:db8::1").httpBaseURL?.absoluteString, "http://[2001:db8::1]:8384/api/v1") }
    func testBracketedIPv6Normalizes() throws { XCTAssertEqual(try ServerConfiguration(host: "[fe80::1]").validated().host, "fe80::1") }
    func testLocalHostnameURL() { XCTAssertEqual(ServerConfiguration(host: "syncspace.local", useHTTPS: true).httpBaseURL?.absoluteString, "https://syncspace.local:8384/api/v1") }
    func testPortValidation() { XCTAssertThrowsError(try ServerConfiguration(host: "server.local", port: 0).validated()); XCTAssertThrowsError(try ServerConfiguration(host: "server.local", port: 65_536).validated()) }
    func testSettingsNormalization() throws { var settings = AppSettings(); settings.serverHost = "  syncspace.local  "; XCTAssertEqual(try settings.normalized().serverHost, "syncspace.local"); settings.serverHost = "http://syncspace.local"; XCTAssertThrowsError(try settings.normalized()) }
    func testTransferProgressAndClamping() { XCTAssertEqual(Transfer.clampedProgress(bytesTransferred: 50, totalBytes: 100), 0.5); XCTAssertEqual(Transfer.clampedProgress(bytesTransferred: -1, totalBytes: 100), 0); XCTAssertEqual(Transfer.clampedProgress(bytesTransferred: 200, totalBytes: 100), 1) }
    func testRemainingTime() { XCTAssertEqual(Transfer.remainingTime(totalBytes: 1000, transferredBytes: 400, bytesPerSecond: 100), 6); XCTAssertNil(Transfer.remainingTime(totalBytes: 1000, transferredBytes: 400, bytesPerSecond: 0)) }
    func testAPIErrorMapping() { let error = APIClient.mapHTTPError(status: 403, data: Data(#"{"error":"local management is available only from this device"}"#.utf8)); XCTAssertEqual(error, .httpStatus(403, "local management is available only from this device")) }
    func testEventEnvelopeDecoding() throws { let decoder = JSONDecoder(); decoder.dateDecodingStrategy = .iso8601; let value = try decoder.decode(WebSocketEventEnvelope.self, from: Data(#"{"type":"DeviceDiscovered","device":{"deviceId":"abc"},"timestamp":"2026-07-31T12:00:00Z"}"#.utf8)); XCTAssertEqual(value.type, "DeviceDiscovered"); XCTAssertNotNil(value.payload) }
}
