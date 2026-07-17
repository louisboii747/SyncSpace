# Development and command reference

Run all commands from the repository root unless a section says otherwise.

## Prerequisites

- Go 1.26.4 or newer
- Node.js 22 or newer
- npm, included with Node.js

Install the pinned frontend dependencies once:

```powershell
cd frontend
npm ci
cd ..
```

## Production-like local run

Build the React assets and start the one-device application:

```powershell
cd frontend
npm run build
cd ..
go run ./backend/cmd/server
```

Open <http://127.0.0.1:8384>. The Go process serves the compiled frontend, the
loopback management API, and WebSockets at that address. It also starts a
separate TLS 1.3 peer listener on `0.0.0.0:8385` and advertises that peer port
over mDNS. LAN clients cannot open the management API or frontend.

The production build is embedded at compile time from
`backend/internal/frontend/dist`. Re-run `npm run build` after a frontend
change and before building or running the embedded experience.

## Live frontend development

Terminal 1:

```powershell
go run ./backend/cmd/server
```

Terminal 2:

```powershell
cd frontend
npm run dev
```

Open <http://127.0.0.1:5173>. Vite hot-reloads React and proxies `/api` REST and
WebSocket traffic to the backend. It does not replace the backend: discovery,
pairing, SQLite, transfers, settings, and diagnostics still run in Go.

## Two-device lab

```powershell
go run ./backend/cmd/syncspace dev start
```

This builds one server binary and starts two isolated processes:

| Instance | Management and UI | Encrypted peer listener | Data |
| --- | --- | --- | --- |
| Device A | `127.0.0.1:8384` | `127.0.0.1:18384` | `.syncspace-dev/device-a` |
| Device B | `127.0.0.1:8385` | `127.0.0.1:18385` | `.syncspace-dev/device-b` |

Static loopback discovery is enabled only for this lab. Both processes still
use real identities, signed pairing, independent trust stores, TLS pinning,
offer authentication, transfer workers, and persistent SQLite state.

Useful commands:

```powershell
# Build, start, pair, transfer, verify SHA-256/history, and stop
go run ./backend/cmd/syncspace dev verify

# Check a running Device A
go run ./backend/cmd/syncspace doctor --url http://127.0.0.1:8384

# Create deterministic files under .syncspace-dev/seed
go run ./backend/cmd/syncspace dev seed

# Send the seeded tiny file while dev start is running
go run ./backend/cmd/syncspace test-transfer

# Export a local diagnostics bundle
go run ./backend/cmd/syncspace export-diagnostics --output diagnostics.zip

# Stop the lab first; then remove only .syncspace-dev
go run ./backend/cmd/syncspace dev reset
```

PowerShell and shell wrappers for start, per-device, seed, transfer-test, and
reset live in `scripts/`. See [LOCAL_SIMULATION.md](LOCAL_SIMULATION.md).

## Server configuration

| Variable | Meaning | Default |
| --- | --- | --- |
| `SYNCSPACE_HOST` | Management/UI bind address; must be loopback | `127.0.0.1` |
| `SYNCSPACE_PORT` | Management/UI port | `8384` |
| `SYNCSPACE_PEER_HOST` | Encrypted peer-protocol bind address | `0.0.0.0` |
| `SYNCSPACE_PEER_PORT` | Encrypted peer port advertised over mDNS | management port + 1 |
| `SYNCSPACE_DATA_DIR` | Identity, database, settings, staging, and transfer root | `<user-config>/SyncSpace` |
| `SYNCSPACE_APP_VERSION` | Version exposed through diagnostics/discovery | linked version or `dev` |
| `SYNCSPACE_DEV_MODE` | Enables isolated developer fixture routes | disabled |
| `SYNCSPACE_STATIC_PEERS` | JSON discovery fixtures; rejected unless developer mode is enabled | empty |

PowerShell example with repository-local data and different ports:

```powershell
$env:SYNCSPACE_DATA_DIR = "$PWD\.local-device"
$env:SYNCSPACE_PORT = "9000"
$env:SYNCSPACE_PEER_PORT = "9001"
go run ./backend/cmd/server
```

Do not set `SYNCSPACE_HOST` to a LAN address. The server rejects it by design;
LAN peer traffic belongs on `SYNCSPACE_PEER_HOST`.

## Runtime data

The data directory contains:

- `identity.json`: public device metadata and fingerprint;
- `identity.key`: protected private identity material (DPAPI ciphertext on
  Windows; mode-`0600` file on other current builds);
- `syncspace.db`: schema migrations, trusted identities, queues, chunks,
  resumable state, and history;
- `settings.json`: atomically written user settings;
- `transfers/`: private staging, partial receive files, and managed transfer
  data.

Never copy one live data directory to two devices. That would clone the same
device identity. Use a clean directory for each installation.

## Build binaries

```powershell
go build -o .tmp/syncspace-server.exe ./backend/cmd/server
go build -o .tmp/syncspace-cli.exe ./backend/cmd/syncspace
```

On macOS/Linux omit the `.exe` suffix. These commands build the current
headless/embedded-web product; they do not create an installer or native shell.

## Engineering invariants

- Discovery is untrusted presence and never grants access.
- Pairing requires proof of identity plus confirmation on both devices.
- Peer certificates must match the paired Ed25519 key.
- Management and developer controls remain loopback-only.
- A transfer is not complete until whole-file SHA-256 verification succeeds.
- Protocol models, migrations, API docs, discovery capability fields, and UI
  projections must change together.
- Default tests use isolated temporary data and never require physical devices.
