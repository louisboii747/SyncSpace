# Architecture

## Boundaries

`SyncSpaceKit` owns transport models, server configuration, HTTP/WebSocket clients, service protocols, app settings, persistence interfaces, observable application state, transfer state, errors, logging, and explicit mock fixtures. It deliberately avoids SwiftUI, UIKit, and AppKit.

The iOS target owns tab/navigation presentation, sheets and alerts, touch-oriented actions, `UIPasteboard`, file import, sharing, and notification permission. The macOS target owns split views, tables, commands, `NSPasteboard`, panels, sandbox file access, and future `MenuBarExtra` work.

```text
 iOS SwiftUI target ─┐                  ┌─ URLSession HTTP ─ Go management API
                     ├─ SyncSpaceKit ───┤
 macOS SwiftUI target┘  app state       ├─ WebSocket streams ─ Go event brokers
                        service APIs    └─ stores/adapters ─ UserDefaults + future database
                             │
                    UIKit/AppKit adapters
```

## State and dependency flow

Each app composition root chooses live or explicit mock dependencies. `SyncSpaceAppState` coordinates initial loading and discovery; feature services retain their own business boundaries. `TransferManager` owns session transfer projections and derived progress/speed/remaining time. SwiftUI renders observable state and invokes intent methods; it does not construct URLs or decode backend payloads.

HTTP flow is view intent → app state/service → `APIClientProtocol` → central `APIEndpoint` → decoded model or `APIError` → user-facing `AppError`. WebSocket flow will be socket stream → extensible event envelope → typed feature reducer → app state. The envelope intentionally preserves unknown payload shapes until each Go broker schema is integrated.

Basic settings use `UserDefaults`. Verification material, tokens, file contents, and message contents must not be placed there. `NoteStore`, `TransferStore`, and `TrustedDeviceCache` allow later SwiftData/Core Data/SQLite implementations without changing views.

`NativeBonjourDiscoveryService` is a non-default documented stub. A future implementation can isolate `Network.framework` and `NWBrowser`; backend-driven discovery stays the default. Mock mode requires `--mock-mode` and must never be enabled as an error fallback.
