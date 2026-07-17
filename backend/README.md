# SyncSpace backend

The Go backend is the runnable SyncSpace engine. It owns device identity,
discovery, cryptographic pairing, durable trust, encrypted transfers, settings,
diagnostics, SQLite migrations, WebSockets, and the embedded React build.

From the repository root:

```powershell
go run ./backend/cmd/server
```

Open <http://127.0.0.1:8384>. Management and frontend traffic is loopback-only.
Encrypted peer pairing and transfer traffic uses TLS 1.3 on `0.0.0.0:8385` by
default. mDNS advertises the peer listener, not the management listener.

Build current frontend assets before compiling a distributable server:

```powershell
cd frontend
npm ci
npm run build
cd ..
go build ./backend/cmd/server
```

Local clients should use `/api/v1/*` and `/api/v1/ws/*`. Peer devices use
`/v1/pairing/*` and `/v1/transfers/*` only through the identity-pinned TLS
listener. Unversioned management routes remain temporarily available for
compatibility.

Key environment variables are `SYNCSPACE_HOST`, `SYNCSPACE_PORT`,
`SYNCSPACE_PEER_HOST`, `SYNCSPACE_PEER_PORT`, `SYNCSPACE_DATA_DIR`, and
`SYNCSPACE_APP_VERSION`. `SYNCSPACE_HOST` must resolve to loopback. Developer
fixtures require `SYNCSPACE_DEV_MODE=true`; static peers are rejected otherwise.

See [Development](../docs/development.md), [Architecture](../docs/architecture.md),
[API](../docs/api.md), [Transfer protocol](../docs/transfer-protocol.md), and
[Security](../SECURITY.md).
