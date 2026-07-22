import SwiftUI
import SyncSpaceCore

@main
struct SyncSpaceApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        WindowGroup {
            ContentView(model: model)
                .task { await model.start() }
        }
    }
}
