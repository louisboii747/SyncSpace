import SwiftUI

@main struct SyncSpaceIOSApp: App {
    @StateObject private var container = IOSAppContainer(mockMode: ProcessInfo.processInfo.arguments.contains("--mock-mode"))
    var body: some Scene { WindowGroup { IOSRootView().environmentObject(container.state) } }
}
