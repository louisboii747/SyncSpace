import XCTest
@testable import SyncSpaceCore

final class ModelsTests: XCTestCase {
    func testDeviceDecodesExistingAPIShape() throws {
        let data = #"{"id":"device-1","name":"Mac","online":true,"appVersion":"1.0.3"}"#.data(using: .utf8)!
        let device = try JSONDecoder().decode(Device.self, from: data)
        XCTAssertEqual(device.id, "device-1")
        XCTAssertEqual(device.appVersion, "1.0.3")
    }
}
