# SyncSpace backend

The backend is the local networking engine shared by SyncSpace clients. It
advertises `_syncspace._tcp.local.` over mDNS, maintains a live peer registry,
stores explicit local trust decisions, and runs the persistent local-network
file transfer engine.

## Run

```sh
go run ./backend/cmd/server
```

Then open `http://127.0.0.1:8384`. The embedded React application and every
management endpoint are restricted to loopback clients even though peer
protocol routes listen on the LAN.

The server listens on all interfaces at port `8384` by default. Its permanent
device UUID is created once in the operating system's user config directory.
Trusted devices are stored beside it in `syncspace.db` and survive restarts.

Environment variables:

- `SYNCSPACE_HOST`: bind host (default `0.0.0.0`)
- `SYNCSPACE_PORT`: API and advertised port (default `8384`)
- `SYNCSPACE_DATA_DIR`: identity storage directory
- `SYNCSPACE_APP_VERSION`: advertised version (default build version or `dev`)

Transfer data and restart-safe partial files live below the same data directory.
The engine defaults to two concurrent transfers, four chunk workers per
transfer, and 4 MiB chunks negotiated down to the receiver's advertised limit.

## Discovery API

- `GET /devices` returns the current peer registry.
- `GET /device/self` returns this installation's identity and current endpoint.
- `POST /discovery/refresh` requests an immediate mDNS restart and scan.
- `GET /ws/discovery` streams `DeviceDiscovered`, `DeviceUpdated`,
  `DeviceOffline`, and `DeviceRemoved` events. On connection, current peers are
  first replayed as `DeviceDiscovered` events, after which no polling is needed.

Peers become offline after 30 seconds without an advertisement and are removed
after two minutes. Network-interface changes and periodic refreshes restart both
broadcasting and browsing automatically. LAN address selection rejects common
virtual, tunnel, container, VPN, and link-local adapters and prefers active
physical Wi-Fi or Ethernet interfaces.

## Discovery and pairing

Discovery answers **which SyncSpace devices are currently visible**. mDNS data
is untrusted presence information: seeing a device never grants it access to
future clipboard, note, file, or remote-control features.

Pairing answers **which devices this installation explicitly trusts**. A device
enters the persistent trusted-device store only after a local pairing request is
accepted. Rejecting a request creates no trust, and deleting a trusted device
revokes the local decision. Pairing management and its WebSocket are restricted
to loopback clients on this device; native frontends should call them through
`127.0.0.1` or `::1`.

### Pairing API

- `GET /pairing/trusted-devices` lists durable trusted-device records.
- `POST /pairing/request` creates a five-minute request for a currently online
  discovered device. Body: `{"deviceId":"<device UUID>"}`.
- `POST /pairing/accept` accepts a pending request and stores trust. Body:
  `{"requestId":"<request UUID>"}`.
- `POST /pairing/reject` rejects a pending request without storing trust. Body:
  `{"requestId":"<request UUID>"}`.
- `DELETE /pairing/trusted-devices/:deviceId` removes local trust.
- `GET /ws/pairing` streams `PairingRequested`, `PairingAccepted`,
  `PairingRejected`, and `TrustedDeviceRemoved`. Current trusted devices are
  replayed as `PairingAccepted` events when a local client connects.

Example local flow:

```text
GET  /devices
POST /pairing/request  {"deviceId":"8f3d..."}
     -> {"requestId":"39b1...", "state":"pending", ...}

# After the user confirms the peer on this device:
POST /pairing/accept   {"requestId":"39b1..."}
GET  /pairing/trusted-devices
```

Run the same user-confirmed flow on the other device when mutual trust is
required. Future sync modules must consult the trusted-device store and must not
infer trust from `/devices`.

### Security status and crypto TODOs

- Pairing is explicit and persisted, but this milestone does not yet implement
  authenticated key exchange, proof of key possession, device certificates, or
  transport encryption.
- The stored 256-bit random `pairingKey` is a placeholder for the future public
  key or pairing credential. It is not currently an authenticated identity.
- The SQLite trust store is local plaintext. Platform keystores and encrypted
  credential-at-rest storage remain future work.
- A future protocol must add an out-of-band verification step (for example a
  short authentication code or QR code), replay protection, key rotation,
  revocation propagation, authenticated peer-to-peer acceptance, and TLS or an
  equivalent encrypted transport.
- Until that protocol exists, trusted state is a local authorization decision;
  it must not be treated as cryptographic proof that a network peer owns the
  claimed discovery UUID.

## File transfer

The transfer engine supports files, folders, multiple selections, durable queue
ordering, pause/resume/cancel/retry, restart recovery, adaptive retry
throttling, optional per-chunk gzip, SHA-256 chunk and whole-file verification,
conflict policies, and transfer history. File bodies stream directly between
disk and bounded chunks; whole files are never loaded into RAM.

Local clients use the loopback-only routes below:

- `POST /transfers`
- `GET /transfers` and `GET /transfers/:id`
- `POST /transfers/:id/accept`, `/reject`, `/pause`, `/resume`, `/cancel`, or
  `/retry`
- `DELETE /transfers/history`
- `GET /ws/transfers`

The browser UI additionally uses `/transfers/staging`: it creates an isolated
session, streams each selected file with `PUT /transfers/staging/:id/files`, and
promotes uploaded top-level roots with `POST /transfers/staging/:id/queue`.
Incomplete browser sessions expire after 24 hours. Native clients should queue
real filesystem paths directly and avoid the extra local staging copy.

Peer devices use `/v1/transfers` for offers, resume maps, chunk uploads,
completion, and cancellation. See the [protocol](../docs/transfer-protocol.md),
[API](../docs/api.md), and [database schema](../docs/database.md) for exact
contracts.

Incoming offers require durable trust, a live discovery record whose IP matches
the connection source, and explicit local approval. The current pairing layer
is not authenticated cryptographic identity and peer HTTP is not yet encrypted;
that pre-existing security milestone remains mandatory before deploying on a
hostile LAN.
