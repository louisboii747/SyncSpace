# SyncSpace for Apple platforms

This directory is the native Swift and SwiftUI foundation for iOS 17+ and
macOS 14+. The shared `SyncSpaceCore` module contains protocol models, an async
API client, app state, and Bonjour discovery for the existing
`_syncspace._tcp` service. The two app targets share an adaptive SwiftUI shell.

The source can be edited in VS Code on Windows. Creating, signing, simulating,
and archiving Apple applications still requires macOS and Xcode.

## Open in Xcode

1. Install XcodeGen on the Mac: `brew install xcodegen`.
2. From this directory run `xcodegen generate`.
3. Open `SyncSpaceApple.xcodeproj` and select `SyncSpaceiOS` or
   `SyncSpacemacOS`.
4. Set the development team and replace the `app.syncspace` bundle prefix if
   required by the signing account.

`swift test` validates the shared model/API layer independently of Xcode.

## Runtime boundary

The Apple UI intentionally talks through `SyncSpaceAPIClient`; it does not
duplicate trust or transfer security in presentation code. The next native
milestone is an Apple engine adapter (a signed XCFramework or a native Swift
implementation) that hosts the same loopback API on-device. Bonjour discovery,
local-network privacy declarations, shared models, and the UI state boundary
are ready for that adapter. Until it is present, the starter UI reports the
local engine as unavailable instead of simulating transfers.
