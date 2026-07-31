import SwiftUI

@main struct SyncSpaceMacApp: App {
    @StateObject private var container = MacAppContainer(mockMode: ProcessInfo.processInfo.arguments.contains("--mock-mode"))
    var body: some Scene { WindowGroup { MacRootView().environmentObject(container.state).frame(minWidth: 820, minHeight: 560) }.commands { SyncSpaceCommands(state: container.state) }; Settings { MacSettingsView().environmentObject(container.state).frame(width: 520, height: 480) } }
}
struct SyncSpaceCommands: Commands { let state: SyncSpaceAppState; var body: some Commands { CommandMenu("SyncSpace") { Button("Refresh Devices") { Task { await state.refreshDiscovery() } }.keyboardShortcut("r"); Button("New Note") {}.keyboardShortcut("n").disabled(true); Button("Send Clipboard") {}.keyboardShortcut("v", modifiers: [.command, .shift]).disabled(true); Divider(); Button(state.connectionState == .connected ? "Disconnect" : "Connect") { Task { await state.load() } }.keyboardShortcut("k", modifiers: [.command, .shift]) } } }
