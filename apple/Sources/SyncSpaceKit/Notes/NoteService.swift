import Foundation

public protocol NoteStore: Sendable { func list() async -> [Note]; func save(_ note: Note) async; func delete(id: String) async }
public actor InMemoryNoteStore: NoteStore { private var notes: [Note]; public init(notes: [Note] = []) { self.notes = notes }; public func list() -> [Note] { notes }; public func save(_ note: Note) { notes.removeAll { $0.id == note.id }; notes.insert(note, at: 0) }; public func delete(id: String) { notes.removeAll { $0.id == id } } }
public protocol NoteServiceProtocol: Sendable { func send(_ note: Note) async throws; func recent() async -> [Note]; func receive(_ note: Note) async; func markRead(id: String) async; func delete(id: String) async }
public actor NoteService: NoteServiceProtocol {
    private let store: any NoteStore; public init(store: any NoteStore = InMemoryNoteStore()) { self.store = store }
    public func send(_ note: Note) async throws { throw APIError.httpStatus(501, "The Go backend has no notes endpoint yet.") }
    public func recent() async -> [Note] { await store.list() }
    public func receive(_ note: Note) async { await store.save(note) }
    public func markRead(id: String) async { guard var note = await store.list().first(where: { $0.id == id }) else { return }; note.isRead = true; await store.save(note) }
    public func delete(id: String) async { await store.delete(id: id) }
}
