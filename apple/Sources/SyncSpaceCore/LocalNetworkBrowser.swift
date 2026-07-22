import Foundation
#if canImport(Network) && canImport(Combine)
import Network
import Combine

@MainActor
public final class LocalNetworkBrowser: ObservableObject {
	@Published public private(set) var endpoints: [String] = []
    @Published public private(set) var state = "Not started"
    private var browser: NWBrowser?

    public init() {}

    public func start() {
        guard browser == nil else { return }
        let browser = NWBrowser(for: .bonjour(type: "_syncspace._tcp", domain: "local."), using: .tcp)
        browser.stateUpdateHandler = { [weak self] next in
            Task { @MainActor in self?.state = String(describing: next) }
        }
        browser.browseResultsChangedHandler = { [weak self] results, _ in
			Task { @MainActor in self?.endpoints = results.map { String(describing: $0.endpoint) } }
        }
        browser.start(queue: DispatchQueue(label: "app.syncspace.bonjour"))
        self.browser = browser
    }

    public func stop() {
        browser?.cancel()
        browser = nil
        endpoints = []
        state = "Stopped"
    }
}
#endif
