import SwiftUI
import SyncSpaceKit

@MainActor final class IOSAppContainer: ObservableObject {
    let state: SyncSpaceAppState
    init(mockMode: Bool = false) {
        let settingsStore = UserDefaultsSettingsStore(); let settings = settingsStore.load()
        if mockMode {
            let transfers = TransferManager(service: MockTransferService(), initial: MockData.transfers)
            state = SyncSpaceAppState(settings: settings, devices: MockDeviceService(), pairing: MockPairingService(), notes: NoteService(store: InMemoryNoteStore(notes: MockData.notes)), transferManager: transfers, settingsStore: settingsStore)
        } else {
            let api = APIClient(configuration: settings.serverConfiguration); let devices = DeviceService(api: api)
            state = SyncSpaceAppState(settings: settings, devices: devices, pairing: PairingService(api: api), notes: NoteService(), transferManager: TransferManager(service: FileTransferService(api: api)), settingsStore: settingsStore)
        }
    }
}
