import SwiftUI
import SyncSpaceCore

struct ContentView: View {
    @ObservedObject var model: AppModel

    var body: some View {
        NavigationSplitView {
            List {
                Label("Nearby", systemImage: "antenna.radiowaves.left.and.right")
                Label("Transfers", systemImage: "arrow.left.arrow.right")
                Label("Settings", systemImage: "gearshape")
            }
            .navigationTitle("SyncSpace")
        } detail: {
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    Text("Your private transfer space").font(.largeTitle.bold())
                    if let device = model.snapshot.localDevice {
                        Label(device.name, systemImage: "checkmark.shield.fill")
                    }
                    if let error = model.errorMessage {
                        ContentUnavailableView("Engine unavailable", systemImage: "bolt.horizontal.circle", description: Text(error))
                    }
                    GroupBox("Nearby devices") {
                        ForEach(model.snapshot.devices) { device in
                            HStack {
                                Image(systemName: "laptopcomputer.and.iphone")
                                Text(device.name)
                                Spacer()
                                Circle().fill(device.online == false ? .secondary : .green).frame(width: 8, height: 8)
                            }.padding(.vertical, 4)
                        }
                    }
                    Text("Bonjour: \(model.browser.state)").font(.caption).foregroundStyle(.secondary)
                }.padding()
            }
            .refreshable { await model.refresh() }
        }
    }
}
