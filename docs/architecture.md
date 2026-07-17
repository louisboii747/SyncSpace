# Architecture

SyncSpace is a local-first, per-device system. There is no central account,
cloud database, relay, or internet fallback in the implemented data path.

## Runtime topology

One normal process owns two listeners:

```text
local browser/native client
        |
        | HTTP + WebSocket, loopback only (default 127.0.0.1:8384)
        v
management API + embedded React UI
        |
        +-- identity / settings / SQLite / transfer workers
        +-- mDNS discovery and untrusted peer registry
        |
        | TLS 1.3 peer protocol (default 0.0.0.0:8385)
        v
paired SyncSpace device on the LAN
```

The loopback listener serves `/api/v1/*` management routes and the frontend.
Legacy unversioned management paths are retained during migration. The LAN
listener serves `/v1/pairing/*` and `/v1/transfers/*`; local-only middleware
prevents it from reaching management routes.

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
- `backend/cmd/server`: composition root and two-listener lifecycle.
- `backend/cmd/syncspace`: doctor, lab, verification, and diagnostics workflows.

## Identity and pairing

An installation creates a stable Ed25519 identity. The public key fingerprint
is advertised as a short mDNS hint; the complete key is supplied and signed in
the pairing protocol. Discovery metadata alone is never authoritative.

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

## Persistence and migrations

`schema_migrations` records ordered database upgrades. Current migrations cover
legacy trusted devices, durable transfer/session/chunk/history state, and the
cryptographic trust columns. Stores call the same central migrator, so an old
database upgrades before either pairing or transfer code uses it.

Settings use a separate validated JSON document written through a temporary
file and atomic rename. Identity public metadata and private material are kept
in separate files.

## Resource limits

- Whole files are streamed with bounded buffers.
- Default chunks are 4 MiB and negotiate up to a 16 MiB maximum.
- Four chunk workers run per active transfer by default.
- Two outbound transfers run concurrently by default.
- Retry backoff adds and later relaxes adaptive send delay.
- WebSocket consumers receive full projections and can reconnect for a fresh
  snapshot if they fall behind.

## Platform state

The shared Go/React product runs from source on Windows, macOS, and Linux where
its dependencies are supported. Windows has DPAPI private-key protection. The
current non-Windows adapter uses strict file permissions, not Keychain or
Secret Service. The `android`, `ios`, `macos`, and `windows` directories are
architecture placeholders, not complete native applications. Native shells,
background services, share extensions, installers, tray integration, and
updaters remain separate deliverables.
