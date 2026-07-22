import Foundation
#if canImport(Combine) && canImport(Network)
import Combine

@MainActor
public final class AppModel: ObservableObject {
    @Published public private(set) var snapshot = AppSnapshot()
    @Published public private(set) var isLoading = false
    @Published public private(set) var errorMessage: String?
    public let browser = LocalNetworkBrowser()
    private let api: SyncSpaceAPIClient

    public init(api: SyncSpaceAPIClient = SyncSpaceAPIClient()) { self.api = api }

    public func start() async {
        browser.start()
        await refresh()
    }

    public func refresh() async {
        isLoading = true
        defer { isLoading = false }
        do {
            snapshot = try await api.snapshot()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
#endif
