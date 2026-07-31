# Permissions and entitlements

These are likely Xcode settings, not active capabilities merely because snippets exist here.

## iOS

- Add `NSLocalNetworkUsageDescription`: `SyncSpace searches for and connects to trusted nearby devices so you can explicitly share files, notes, and clipboard text on your local network.`
- If native Bonjour is implemented, add `NSBonjourServices` with the backend-advertised type, currently `_syncspace._tcp`.
- SwiftUI `fileImporter` grants scoped access to selected files; copy or read while access is valid.
- Request notification authorization only after the user enables it.
- Background `URLSession` requires a real target configuration and lifecycle testing; it is not enabled by this scaffold.

```xml
<key>NSLocalNetworkUsageDescription</key>
<string>SyncSpace searches for and connects to trusted nearby devices so you can explicitly share content on your local network.</string>
<key>NSBonjourServices</key>
<array><string>_syncspace._tcp</string></array>
```

## macOS

Enable App Sandbox, outgoing network connections, and incoming connections only if the app itself must listen. Enable user-selected file read/write access. Persisted access requires security-scoped bookmarks and stale-bookmark handling. Configure Local Network and notifications in the actual signed target. Minimize entitlements and validate the sandboxed archive, not only a debug run.

Security review remains required for pairing authentication, TLS/certificate pinning, impersonation resistance, replay protection, token/keychain storage, and Apple Local Network permission behavior. The clients must not claim end-to-end encryption merely because the backend uses authenticated pairing and TLS-backed peer transfers.
