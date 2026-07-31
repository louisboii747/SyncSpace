# Xcode handoff

This scaffold was created on Windows and has not been compiled with Apple SDKs.

1. Install the latest stable Xcode on a Mac, open it once, accept the licence, and install iOS platform components.
2. Create a workspace named `SyncSpaceApple` inside `apple/` (or generate the starter targets with XcodeGen from `project.yml`, then inspect all settings).
3. Add the local package at the `apple` directory and link the `SyncSpaceKit` product to both apps.
4. Create an iOS App target named `SyncSpace` using SwiftUI, then add `Apps/iOS` sources only.
5. Create a macOS App target named `SyncSpace` using SwiftUI, then add `Apps/macOS` sources only.
6. Choose unique bundle IDs, for example `app.syncspace.ios` and `app.syncspace.macos`, and assign the correct development team.
7. Set iOS 17 and macOS 14 deployment versions initially. Lower them only after API availability review.
8. Confirm each app has exactly one `@main` file and that platform source membership does not cross targets.
9. Add the Local Network description and optional Bonjour declaration described in `ENTITLEMENTS.md`.
10. Configure macOS App Sandbox network and user-selected-file entitlements. Add iOS notification/background capabilities only when implemented.
11. Add production app icon asset catalogs, signing certificates, and provisioning profiles.
12. Build `SyncSpaceKit`, resolve strict-concurrency or SDK availability diagnostics, and run its tests.
13. Run the iOS target in Simulator, then on a physical iPhone to validate Local Network permission and real LAN behavior.
14. Run the sandboxed macOS target and validate clipboard/file adapters.
15. Use `--mock-mode` only for UI development. Remove it for integration testing.
16. Run the Go backend and first test `/device/self`; then discovery, pairing, WebSockets, and transfers as each contract becomes remotely available.

## Network checklist

The Mac/iPhone and Windows PC must be on the same network; Wi-Fi client isolation must be off. Windows Firewall must allow TCP 8384 on the intended private profile. The backend must listen on a LAN-accessible interface, not only `127.0.0.1`. Enter the Windows machine's LAN IPv4, IPv6, DNS, or working `.local` name. `localhost` in an Apple app means the Apple device or simulator itself, never the Windows PC.

Even with correct networking, the current backend intentionally returns 403 to remote management clients. Design and implement an authenticated LAN management boundary before expecting live Apple integration. Do not simply remove `localOnly()`.

## First three actions on the Mac

1. Open Terminal, `cd` to `SyncSpace/apple`, and run `swift test`.
2. Create/open `SyncSpaceApple.xcworkspace`, add the local package, and create the two targets with the membership above.
3. Build each target, fix genuine Xcode diagnostics, then run the iOS app with `--mock-mode` before attempting backend integration.
