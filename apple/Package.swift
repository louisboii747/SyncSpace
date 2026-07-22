// swift-tools-version: 5.10
import PackageDescription

let package = Package(
    name: "SyncSpaceCore",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [.library(name: "SyncSpaceCore", targets: ["SyncSpaceCore"])],
    targets: [
        .target(name: "SyncSpaceCore", path: "Sources/SyncSpaceCore"),
        .testTarget(name: "SyncSpaceCoreTests", dependencies: ["SyncSpaceCore"], path: "Tests/SyncSpaceCoreTests")
    ]
)
