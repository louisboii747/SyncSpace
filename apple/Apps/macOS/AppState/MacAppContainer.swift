import SwiftUI
import SyncSpaceKit

@MainActor final class MacAppContainer: ObservableObject {
    let state: SyncSpaceAppState
    init(mockMode: Bool = false) { let store = UserDefaultsSettingsStore(); let settings = store.load(); if mockMode { state = SyncSpaceAppState(settings: settings, devices: MockDeviceService(), pairing: MockPairingService(), notes: NoteService(store: InMemoryNoteStore(notes: MockData.notes)), transferManager: TransferManager(service: MockTransferService(), initial: MockData.transfers), settingsStore: store) } else { let api = APIClient(configuration: settings.serverConfiguration); state = SyncSpaceAppState(settings: settings, devices: DeviceService(api: api), pairing: PairingService(api: api), notes: NoteService(), transferManager: TransferManager(service: FileTransferService(api: api)), settingsStore: store) } }
}
