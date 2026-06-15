# SyncSpace

SyncSpace is a cross-platform device synchronization platform designed to make moving information between devices effortless.

The project aims to provide a seamless experience across Windows, Android, iOS, iPadOS, macOS, and eventually Linux, allowing devices to discover each other, exchange data, and stay synchronized through a fast and reliable local-first architecture.

Rather than relying entirely on cloud services, SyncSpace is being built around direct device-to-device communication, enabling features such as clipboard synchronization, file transfers, notes synchronization, and real-time communication across a user's personal device ecosystem.

## Planned Features

- Automatic device discovery using mDNS/Zeroconf
- Secure device pairing and management
- Cross-device clipboard synchronization
- Local network file transfers
- Notes synchronization
- Real-time device status updates
- Transfer history and activity tracking
- Local-first architecture with optional cloud features
- End-to-end encrypted communication
- Native desktop and mobile experiences
- Extensible architecture for future integrations and features

## Frontend Architecture

SyncSpace is designed around a shared synchronization engine and platform-native user interfaces.

Each supported operating system will provide a user experience tailored to its platform while communicating with the same underlying Go synchronization engine.

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

- Native desktop UI (currently under evaluation)

#### Linux

- Planned future support

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

SyncSpace is currently in active early development.

The project foundation has been established, including basic repository structure, multi-platform project organization, and the Go-based engine. Current development is focused on implementing local network device discovery and building the core communication layer that future synchronization features will rely upon.

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
- Transfer history
- Settings and device management

### Phase 3

- File transfers
- Enhanced security and encryption
- Cross-platform UI refinement

### Future

- Optional cloud synchronization
- Remote access capabilities
- Additional productivity tools and integrations

## License

Licensing information will be provided as the project matures.
