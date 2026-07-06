# SyncSpace

SyncSpace is a cross-platform device synchronization platform designed to make moving information between devices effortless.

The project aims to provide a seamless experience across Windows, Android, iOS, iPadOS, macOS, and eventually Linux, allowing devices to discover each other, exchange data, and stay synchronized through a fast and reliable local-first architecture.

Rather than relying entirely on cloud services, SyncSpace is being built around direct device-to-device communication, enabling features such as clipboard synchronization, file transfers, notes synchronization, and real-time communication across a user's personal device ecosystem.

## Features and direction

- Automatic device discovery using mDNS/Zeroconf
- Secure device pairing and management
- Cross-device clipboard synchronization
- Local network file transfers with persistent queues, pause/resume, retries,
  folder manifests, compression, and SHA-256 verification
- Notes synchronization
- Real-time device status updates
- Transfer history and activity tracking
- Local-first architecture with optional cloud features
- End-to-end encrypted communication
- Native desktop and mobile experiences
- Extensible architecture for future integrations and features

## Frontend Architecture

SyncSpace now includes a responsive React and TypeScript control surface embedded
directly in the Go server. Open `http://127.0.0.1:8384` to discover and trust
devices, drag files or folders into the persistent queue, approve incoming
transfers, monitor live speed and ETA, pause/resume/cancel/retry, and inspect
history. The interface and all management APIs are loopback-only.

The web surface is the shared desktop baseline. Platform-native clients can
still provide deeper operating-system integrations while consuming the same
REST and WebSocket contracts.

### Current Direction

#### Android

- Kotlin
- Jetpack Compose

#### iOS & iPadOS

- Swift
- SwiftUI

#### macOS

- Swift
- SwiftUI

#### Windows

- Embedded React control surface

#### Linux

- Embedded React control surface

This approach allows SyncSpace to provide a native experience on every platform while sharing the same synchronization, networking, capabilities across the entire ecosystem.

### Backend

- Go
- Gin
- Gorilla WebSocket
- Zeroconf (mDNS)
- SQLite

### Platforms

- Windows
- Android
- iOS
- iPadOS
- macOS
- Linux (planned)

### Communication

- WebSockets
- Local network communication
- Device-to-device synchronization

## Project Status

SyncSpace is currently in active development.

The backend includes automatic local-network discovery, explicit persistent
pairing decisions, and a restart-safe local file transfer engine. Transfers use
bounded parallel chunk streaming, explicit inbound approval, resume maps,
per-chunk and whole-file SHA-256 verification, conflict policies, history, REST,
and live WebSocket events. Discovery presence never grants trust automatically.

The embedded React frontend provides the complete shared transfer experience,
including streaming browser staging for drag-and-drop and folder selection.
Native client behavior remains defined in the [transfer UI
contract](docs/transfer-ui.md). Authenticated key exchange and encrypted peer
transport remain required before use on an untrusted LAN.

## Vision

The long-term goal for SyncSpace is to provide a unified platform for communication and productivity that feels native on every supported operating system while maintaining a fast, reliable, and privacy-focused experience.

## Roadmap

### Phase 1

- Device discovery
- Device registry
- Secure pairing
- Real-time communication layer

### Phase 2

- Clipboard synchronization
- Notes synchronization
- Transfer history and reliable local file-transfer engine (backend complete)
- Settings and device management

### Phase 3

- Deeper platform-native file-transfer integrations
- Enhanced security and encryption
- Cross-platform UI refinement

### Future

- Optional cloud synchronization with user accounts
- Remote access capabilities
- Additional productivity tools and integrations

## License

Apache 2.0
