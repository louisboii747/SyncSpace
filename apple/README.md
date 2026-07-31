# SyncSpace for Apple platforms

This directory is an early native Apple-port foundation, initially scaffolded on Windows. It contains a shared `SyncSpaceKit` Swift package and separate SwiftUI source trees for iOS 17+ and macOS 14+. Those deployment targets can be lowered later after an explicit compatibility review.

It has not been built with Xcode. No iOS simulator, Apple SDK, signing, entitlement, or real-device result is implied. Start with [Documentation/XCODE_SETUP.md](Documentation/XCODE_SETUP.md) on a Mac.

## What exists

- Real foundations: Codable models aligned to Go payloads; IPv4/IPv6-aware configuration; async `URLSession` API client; WebSocket connection/reconnect stream; service protocols; dependency injection; settings validation; transfer state calculations; user-facing errors; privacy-conscious logging; SwiftUI application shells.
- Explicit mock mode: realistic devices, pairing, transfers, notes, clipboard items, and mock services. Launch with `--mock-mode` only for previews/UI work. Production never silently falls back to mocks.
- Placeholders: native Bonjour, clipboard and notes backend routes, native file upload/download workflow, notifications, background transfers, share extensions, and menu-bar mode. Unsupported actions are disabled in starter UI.

The Apple apps are clients of the existing Go backend; they do not reimplement discovery, trust, pairing cryptography, or transfer protocol logic. A crucial current limitation is that the Go management API and WebSockets are protected by `localOnly()`. A backend running on another LAN device will reject an iPhone or Mac client with HTTP 403. This scaffold preserves those routes for future authenticated remote access but does not weaken that boundary.

## Layout

```text
Package.swift                 Shared package manifest
Sources/SyncSpaceKit/         Models, networking, services, state, storage, mocks
Tests/SyncSpaceKitTests/      Platform-independent logic tests
Apps/iOS/                     iOS SwiftUI app target sources and adapters
Apps/macOS/                   macOS SwiftUI app target sources and adapters
Configuration/                Draft plist and entitlement inputs
Documentation/                Architecture, integration, security, roadmap, Xcode handoff
project.yml                   Optional XcodeGen target description
```

`project.yml` is a convenience input, not a generated `.xcodeproj`. Swift Package Manager has no third-party dependencies and CocoaPods is not required.
