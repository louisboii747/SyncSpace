import Foundation

public struct AppError: Identifiable, Equatable, Sendable {
    public let id = UUID(); public let title: String; public let message: String; public let technicalDetail: String; public let recoverySuggestion: String?
    public init(_ error: Error, title: String = "SyncSpace needs attention") {
        self.title = title; technicalDetail = String(reflecting: error)
        if let api = error as? APIError { message = api.errorDescription ?? "An unexpected backend error occurred."; recoverySuggestion = api.recoverySuggestion }
        else { message = error.localizedDescription; recoverySuggestion = "Try again. If the problem continues, enable debug logging and inspect the backend logs." }
    }
}
