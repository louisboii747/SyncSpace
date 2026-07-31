import Foundation
#if !canImport(Combine)
public protocol ObservableObject: AnyObject {}
@propertyWrapper public struct Published<Value> { public var wrappedValue: Value; public init(wrappedValue: Value) { self.wrappedValue = wrappedValue } }
#endif

/// Shared, UI-independent foundations used by the native iOS and macOS clients.
public enum SyncSpaceKit {
    public static let defaultPort = 8384
    public static let backendAPIVersion = "v1"
}
