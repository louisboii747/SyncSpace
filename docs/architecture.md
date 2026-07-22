# Architecture

SyncSpace is a local-first, per-device system. There is no central account,
cloud database, relay, or internet fallback in the implemented data path.

## Runtime topology

The desktop process owns one companion backend process and its two listeners:

```text
native Wails window with embedded React assets
        |
        | HTTP + WebSocket, loopback only (default 127.0.0.1:8384)
        v
loopback management API
        |
        +-- identity / settings / SQLite / transfer workers
        +-- mDNS discovery and untrusted peer registry
        |
        | TLS 1.3 peer protocol (default 0.0.0.0:8385)
        v
paired SyncSpace device on the LAN
```

The Wails asset server renders the bundled frontend without a localhost page.
The loopback listener serves `/api/v1/*` management routes for the private
desktop transport and development tools. Only the exact Wails document origins
are allowed cross-origin; arbitrary browser origins remain rejected.
Legacy unversioned management paths are retained during migration. The LAN
listener serves `/v1/pairing/*` and `/v1/transfers/*`; local-only middleware
prevents it from reaching management routes.

Both sockets are bound during process startup so failures are reported
immediately. Binding the peer socket is not permission to use it: until privacy
policy `2026-07-1` is accepted, backend middleware rejects operational local and
peer routes. The runtime supervisor also keeps discovery and transfer workers
stopped.

## Privacy-gated startup

Normal startup follows this order:

1. Resolve paths, load or create identity, load settings, migrate SQLite, and
   compose the local and peer HTTP servers.
2. Serve the local UI plus the policy, settings, identity, and health endpoints.
3. If the accepted policy version is missing or stale, keep mDNS and transfer
   workers stopped and reject discovery, pairing, diagnostics, WebSocket,
   staging, and transfer operations with `403`.
4. After explicit acceptance, start transfer recovery/workers. Start mDNS
   browsing and advertising only when `discoverable` is also true.
5. React to later settings changes without restarting: withdraw or restore mDNS
   for discoverability changes and reject new offers when incoming transfers
   are disabled.

The UI is therefore not the security boundary. A local client that calls an
operational endpoint directly receives the same policy decision. The developer
lab waits for each isolated policy endpoint, accepts the current policy through
the real local API, and only then requires both processes to report healthy;
normal server startup never auto-accepts.

## Ownership boundaries

- `backend/internal/services`: persistent device identity and platform secret
  protection.
- `backend/internal/discovery`: physical-adapter selection, mDNS advertising,
  untrusted peer presence, identity hints, and capability projection.
- `backend/internal/pairing`: signed key exchange, verification state, shared
  credential derivation, replay control, durable trust, block/forget.
- `backend/internal/transfer`: manifests, offers, authentication, queues,
  streaming, resume maps, chunks, partial-file commits, integrity, history.
- `backend/internal/database`: ordered idempotent SQLite migrations.
- `backend/internal/settings`: validated and atomically persisted preferences.
- `backend/internal/api` and `backend/internal/websocket`: local and peer delivery
  adapters.
- `frontend`: React/TypeScript management client.
- `backend/internal/frontend`: compiled asset embedding.
- `backend/cmd/desktop`: Wails window, single-instance lock, window-state
  persistence, and companion-backend lifecycle.
- `backend/cmd/server`: composition root and two-listener lifecycle.
- `backend/cmd/syncspace`: doctor, lab, verification, and diagnostics workflows.

## Identity and pairing

An installation creates a stable Ed25519 identity. The public key fingerprint
is advertised as a short mDNS hint; the complete key is supplied and signed in
the pairing protocol. Discovery metadata alone is never authoritative.

The identity model deliberately separates three labels:

- `deviceId` is a secure random UUID persisted in `identity.json`. It survives
  hostname and display-name changes and is not based on a MAC address or private
  hardware identifier.
- `hostname` is the operating-system hostname captured in identity metadata and
  advertised so a person can recognise the physical computer.
- `deviceName` is the editable SyncSpace display name. It defaults from the
  hostname or a platform fallback, then persists in `settings.json`.

Changing `deviceName` refreshes mDNS and updates names used in future pairing
messages and transfer offers. It does not rotate the Ed25519 key, change the
stable device ID, or invalidate durable trust. A discovered name or hostname is
presentation data; trust remains bound to the paired public key.

mDNS TXT data includes the device ID, display name, hostname, platform/type,
app version, peer port, protocol/capability values, availability, and a short
identity hint. It never includes pairing credentials, transfer tokens, file
paths, usernames, or file content.

Pairing performs an ephemeral X25519 exchange. Each side signs its contribution
with Ed25519, validates the discovered identity hint, derives the same shared
key with HKDF-SHA-256, and displays a six-digit short authentication string.
Both local users must confirm before trust persists. Proof messages contain
timestamps and nonces and are HMAC-authenticated; used nonces are rejected.

The durable record binds device ID, full public key, fingerprint, and shared
pairing credential. A changed key is surfaced as an identity change and is not
silently trusted. Blocking keeps the identity record but denies authorization;
forgetting deletes local trust.

Pairing sessions themselves are intentionally short-lived and in memory. A
restart during pairing requires starting the handshake again; completed trust
survives restart.

## Transfer lifecycle

The sender builds a portable manifest, rejects symlinks and special files, and
streams source hashing without loading whole files into memory. Its initial
offer is HMAC-authenticated with the paired shared key, method/path/device ID,
body digest, timestamp, and one-use nonce. The receiver also validates current
discovery IP and durable trust.

After explicit receiver approval, a random transfer bearer credential is used
inside the TLS 1.3 channel. The sender pins the peer certificate's Ed25519 key
to the paired public key. Resume maps identify existing chunks, and a bounded
worker pool sends only missing chunks with optional gzip.

Before approval changes the state to `Receiving`, the receiver creates/resolves
the chosen destination and compares the transfer size with filesystem-reported
free space. A known shortfall fails acceptance with HTTP `507`; an unavailable
platform reading is not presented as a made-up capacity.

The receiver writes by offset into private `.part` files. It validates each
chunk digest before marking it present. Completion requires all chunks plus a
streamed whole-file SHA-256 check before the partial is moved into the approved
destination. Crash recovery and SQLite chunk state make retries idempotent.

## Browser staging

Browsers do not expose absolute source paths. File and folder selections are
therefore streamed to a private loopback staging session. Sealing that session
promotes its top-level roots into the same durable transfer queue used by native
path submissions. Unfinished staging expires automatically. Peer devices never
access the staging API.

The browser supplies file bodies and relative paths, not a complete filesystem
manifest. An empty directory has no file body and is therefore absent from a
browser-staged folder. Browser staging also does not preserve modification
timestamps or filesystem permissions. Native path submission can represent
empty directories, but the current transfer model still does not carry
timestamps or permissions across devices.

## Embedded UI workflow

The React interface is a task-focused control surface for the backend, not a
separate source of transfer truth. Its current send sequence is:

1. choose an online, compatible, paired device;
2. choose or drop files/folders;
3. review the display name, hostname, destination platform, relative paths,
   individual sizes, total size, and executable/script extension warnings;
4. add more files/folders, remove individual files, or clear the selection while
   it is still unstaged;
5. confirm before browser staging and queue creation begin;
6. follow backend byte counts through transfer and SHA-256 verification.

The incoming review shows the authenticated sender display name, optional
hostname/platform, explicit trusted status, manifest summary and item count, a
bounded scrollable per-file preview, total size, executable/script warnings,
destination, and conflict policy before any file body is accepted. Missing
hostname/platform values from legacy offers are omitted. Completion is rendered
only after the backend publishes the verified terminal state.
Friendly UI status text may translate backend state names, but it does not
invent progress or hide the underlying failure.

The backend is authoritative after a refresh or WebSocket reconnect. Device,
pairing, transfer, history, settings, and privacy data are reloaded from the
loopback API.

## Persistence and migrations

`schema_migrations` records ordered database upgrades. Current migrations cover
legacy trusted devices, durable transfer/session/chunk/history state,
cryptographic trust columns, and optional sender hostname/platform fields on
durable transfers. Stores call the same central migrator, so an old database
upgrades before either pairing or transfer code uses it.

Settings use a separate validated JSON document written through a temporary
file and atomic rename. Identity public metadata and private material are kept
in separate files.

Settings schema v2 adds the editable device name, discoverability, incoming-
offer control, policy version, and policy acceptance timestamp. General settings
writes preserve privacy acceptance; only the exact-version acceptance operation
can change it. New profiles default to `<home>/Downloads/SyncSpace`, created
when an inbound transfer is accepted, while migrations preserve an existing
chosen path. The safe default conflict policy is rename.

## Resource limits

- Whole files are streamed with bounded buffers.
- Default chunks are 4 MiB and negotiate up to a 16 MiB maximum.
- Four chunk workers run per active transfer by default.
- Two outbound transfers run concurrently by default.
- Retry backoff adds and later relaxes adaptive send delay.
- WebSocket consumers receive full projections and can reconnect for a fresh
  snapshot if they fall behind.

## Current limitations

- Pause, resume, and retry controls in the embedded UI are sender-side. A
  receiver can accept, decline, or cancel, but coordinated receiver pause/retry
  is not implemented end to end.
- Extension-based executable/script warnings are shown on send and receive
  review. SyncSpace does not inspect file content for malware and never opens or
  executes a received file automatically.
- The current desktop bridge does not yet expose opening a received
  file, revealing it in a file manager, copying its path, or opening the local
  data directory.
- Destination conflicts are resolved by the receiver. Sender review
  cannot know the remote filesystem's conflicts in advance.

## Platform state

The shared Go/React desktop product uses Wails on Windows and Linux. Windows has
DPAPI private-key protection; the current non-Windows adapter uses strict file
permissions, not Keychain or Secret Service. Android, iOS, and macOS clients,
native share extensions, signed installers, tray integration, and updaters
remain separate deliverables.
