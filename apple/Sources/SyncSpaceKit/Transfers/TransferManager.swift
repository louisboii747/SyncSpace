import Foundation
#if canImport(Combine)
import Combine
#endif

public protocol FileTransferServiceProtocol: Sendable {
    func transfers() async throws -> [Transfer]; func cancel(id: String) async throws -> Transfer; func retry(id: String) async throws -> Transfer
}
public struct FileTransferService: FileTransferServiceProtocol {
    private let api: any APIClientProtocol; public init(api: any APIClientProtocol) { self.api = api }
    public func transfers() async throws -> [Transfer] { try await api.send(.transfers, response: [Transfer].self) }
    public func cancel(id: String) async throws -> Transfer { try await api.send(.transferAction(id, "cancel"), response: Transfer.self) }
    public func retry(id: String) async throws -> Transfer { try await api.send(.transferAction(id, "retry"), response: Transfer.self) }
}
public protocol TransferStore: Sendable { func load() async throws -> [Transfer]; func save(_ transfers: [Transfer]) async throws }
public actor InMemoryTransferStore: TransferStore { private var values: [Transfer] = []; public init() {}; public func load() -> [Transfer] { values }; public func save(_ transfers: [Transfer]) { values = transfers } }

@MainActor public final class TransferManager: ObservableObject {
    @Published public private(set) var transfers: [Transfer]
    private let service: any FileTransferServiceProtocol; private let store: any TransferStore
    public init(service: any FileTransferServiceProtocol, store: any TransferStore = InMemoryTransferStore(), initial: [Transfer] = []) { self.service = service; self.store = store; transfers = initial }
    public var active: [Transfer] { transfers.filter { ![.completed, .cancelled, .failed].contains($0.status) } }
    public var completed: [Transfer] { transfers.filter { [.completed, .cancelled, .failed].contains($0.status) } }
    public func refresh() async throws { transfers = try await service.transfers(); try await store.save(transfers) }
    public func update(id: String, bytes: Int64, speed: Double) { guard let index = transfers.firstIndex(where: { $0.id == id }) else { return }; transfers[index].bytesTransferred = max(0, min(bytes, transfers[index].fileSize)); transfers[index].transferSpeed = max(0, speed); transfers[index].estimatedTimeRemaining = Transfer.remainingTime(totalBytes: transfers[index].fileSize, transferredBytes: transfers[index].bytesTransferred, bytesPerSecond: speed) }
    public func cancel(id: String) async throws { replace(try await service.cancel(id: id)) }
    public func retry(id: String) async throws { replace(try await service.retry(id: id)) }
    private func replace(_ transfer: Transfer) { if let index = transfers.firstIndex(where: { $0.id == transfer.id }) { transfers[index] = transfer } else { transfers.append(transfer) } }
}
