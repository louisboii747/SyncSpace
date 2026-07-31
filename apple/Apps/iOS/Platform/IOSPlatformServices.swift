import Foundation
import SyncSpaceKit
#if os(iOS)
import UIKit
import UserNotifications

public struct IOSClipboardProvider: PlatformClipboardProviding { public init() {}; public func readText() async -> String? { await MainActor.run { UIPasteboard.general.string } }; public func writeText(_ text: String) async throws { await MainActor.run { UIPasteboard.general.string = text } } }
public enum IOSPlatformTODO { public static let fileImporter = "Add fileImporter to a real iOS target and retain access only while reading the selected URL."; public static let shareSheet = "Add a ShareLink or UIActivityViewController wrapper after defining the receiving contract."; public static let notifications = "Request UNUserNotificationCenter permission only when the user enables notifications." }
#endif
