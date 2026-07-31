import SwiftUI
import SyncSpaceKit

struct IOSRootView: View {
    @EnvironmentObject private var state: SyncSpaceAppState
    var body: some View {
        TabView {
            NavigationStack { IOSHomeView() }.tabItem { Label("Home", systemImage: "house") }
            NavigationStack { IOSDevicesView() }.tabItem { Label("Devices", systemImage: "laptopcomputer.and.iphone") }
            NavigationStack { IOSTransfersView() }.tabItem { Label("Transfers", systemImage: "arrow.left.arrow.right") }
            NavigationStack { IOSNotesView() }.tabItem { Label("Notes", systemImage: "note.text") }
            NavigationStack { IOSSettingsView() }.tabItem { Label("Settings", systemImage: "gearshape") }
        }
        .task { if state.settings.autoConnect { await state.load() } }
        .alert(item: Binding(get: { state.lastError }, set: { _ in state.dismissError() })) { error in Alert(title: Text(error.title), message: Text([error.message, error.recoverySuggestion].compactMap { $0 }.joined(separator: "\n\n")), dismissButton: .default(Text("OK"))) }
    }
}

struct ConnectionBadge: View { let state: APIConnectionState; var body: some View { Label(state.rawValue.capitalized, systemImage: state == .connected ? "checkmark.circle.fill" : "wifi.exclamationmark").font(.caption.weight(.semibold)).foregroundStyle(state == .connected ? .green : .secondary).padding(.horizontal, 10).padding(.vertical, 6).background(.thinMaterial, in: Capsule()) } }
struct DeviceRow: View { let device: Device; var body: some View { HStack(spacing: 12) { Image(systemName: device.deviceType == "phone" ? "iphone" : "desktopcomputer").frame(width: 28).accessibilityHidden(true); VStack(alignment: .leading) { Text(device.name).font(.headline); Text(device.platform).font(.subheadline).foregroundStyle(.secondary) }; Spacer(); Circle().fill(device.isOnline ? .green : .gray).frame(width: 9).accessibilityLabel(device.isOnline ? "Online" : "Offline"); if device.isTrusted { Image(systemName: "checkmark.shield.fill").foregroundStyle(.blue).accessibilityLabel("Trusted") } } } }
struct TransferRow: View { let transfer: Transfer; var body: some View { VStack(alignment: .leading, spacing: 7) { HStack { Text(transfer.fileName).font(.headline); Spacer(); Text(transfer.status.rawValue).font(.caption).foregroundStyle(.secondary) }; ProgressView(value: transfer.progress); HStack { Text(transfer.deviceName ?? "Unknown device"); Spacer(); Text(ByteCountFormatter.string(fromByteCount: Int64(transfer.transferSpeed), countStyle: .file) + "/s") }.font(.caption).foregroundStyle(.secondary) } } }
