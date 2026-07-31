import SwiftUI
import SyncSpaceKit

struct IOSHomeView: View {
    @EnvironmentObject private var state: SyncSpaceAppState
    private let columns = [GridItem(.flexible()), GridItem(.flexible())]
    var body: some View { ScrollView { VStack(alignment: .leading, spacing: 20) {
        HStack { VStack(alignment: .leading) { Text("SyncSpace").font(.largeTitle.bold()); Text(state.localDevice?.name ?? "No local device loaded").foregroundStyle(.secondary) }; Spacer(); ConnectionBadge(state: state.connectionState) }
        LazyVGrid(columns: columns) { metric("Nearby", state.discoveredDevices.count, "dot.radiowaves.left.and.right"); metric("Trusted", state.trustedDevices.count, "checkmark.shield"); metric("Active", state.transferManager.active.count, "arrow.left.arrow.right"); metric("Unread", state.recentNotes.filter { !$0.isRead }.count, "note.text") }
        Text("Quick actions").font(.title2.bold())
        Button { Task { await state.refreshDiscovery() } } label: { Label("Discover devices", systemImage: "arrow.clockwise").frame(maxWidth: .infinity, alignment: .leading).padding() }.buttonStyle(.borderedProminent)
        ForEach([("Send clipboard", "doc.on.clipboard"), ("Send file", "paperclip"), ("Create note", "square.and.pencil")], id: \.0) { action in Button(action: {}) { Label(action.0, systemImage: action.1).frame(maxWidth: .infinity, alignment: .leading) }.buttonStyle(.bordered).disabled(true) }
        if state.connectionState == .failed { ContentUnavailableView("Backend unavailable", systemImage: "network.slash", description: Text("Check Settings. Remote access is currently blocked by the Go backend's loopback-only management policy.")) }
    }.padding() }.navigationTitle("Home").refreshable { await state.load() } }
    private func metric(_ title: String, _ value: Int, _ icon: String) -> some View { VStack(alignment: .leading, spacing: 8) { Image(systemName: icon).foregroundStyle(.tint); Text("\(value)").font(.title.bold()); Text(title).foregroundStyle(.secondary) }.frame(maxWidth: .infinity, alignment: .leading).padding().background(.regularMaterial, in: RoundedRectangle(cornerRadius: 16)) }
}
