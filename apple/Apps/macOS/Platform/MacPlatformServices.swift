import Foundation
import SyncSpaceKit
#if os(macOS)
import AppKit

public struct MacClipboardProvider: PlatformClipboardProviding { public init() {}; public func readText() async -> String? { await MainActor.run { NSPasteboard.general.string(forType: .string) } }; public func writeText(_ text: String) async throws { await MainActor.run { NSPasteboard.general.clearContents(); NSPasteboard.general.setString(text, forType: .string) } } }
public enum MacPlatformTODO { public static let openPanel = "Use NSOpenPanel, start security-scoped access when required, and release it promptly."; public static let notifications = "Configure UserNotifications in the signed macOS target." }
#endif
