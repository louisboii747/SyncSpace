// swift-tools-version: 5.10
import PackageDescription

let package = Package(
    name: "SyncSpaceKit",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [.library(name: "SyncSpaceKit", targets: ["SyncSpaceKit"])],
    targets: [
        .target(name: "SyncSpaceKit", path: "Sources/SyncSpaceKit"),
        .testTarget(name: "SyncSpaceKitTests", dependencies: ["SyncSpaceKit"], path: "Tests/SyncSpaceKitTests")
    ]
)
