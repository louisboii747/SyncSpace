# Development

## Prerequisites

- Go 1.26.4 or newer
- Node.js 22 or newer
- npm (included with Node.js)

From the repository root, install frontend packages once with `cd frontend &&
npm ci`. No global JavaScript packages are required.

## Run one normal device

```sh
go run ./backend/cmd/server
```

The backend listens on `0.0.0.0:8384`, persists its identity and SQLite data in
the operating system's user configuration directory, advertises over mDNS, and
serves the embedded React application. Rebuild embedded frontend assets after a
frontend change:

```sh
cd frontend
npm run build
cd ..
go run ./backend/cmd/server
```

For live frontend work, run `npm run dev` in `frontend/` and the Go server in a
second terminal. Vite listens on `127.0.0.1:5173` and proxies REST and WebSocket
traffic to Device A at `127.0.0.1:8384`.

## Configuration

| Variable | Meaning | Default |
| --- | --- | --- |
| `SYNCSPACE_HOST` | HTTP listen host | `0.0.0.0` |
| `SYNCSPACE_PORT` | HTTP and advertised peer port | `8384` |
| `SYNCSPACE_DATA_DIR` | Identity, SQLite, staging, and transfer root | OS config directory |
| `SYNCSPACE_APP_VERSION` | Version exposed to peers | build version |
| `SYNCSPACE_DEV_MODE` | Enables local simulator routes | disabled |
| `SYNCSPACE_STATIC_PEERS` | JSON array merged with mDNS discovery | empty |

Developer mode does not bypass trust. Static and simulated devices must still
be approved through the pairing service before transfers are accepted.

## Useful CLI

```sh
go run ./backend/cmd/syncspace doctor
go run ./backend/cmd/syncspace dev start
go run ./backend/cmd/syncspace dev verify
go run ./backend/cmd/syncspace dev seed
go run ./backend/cmd/syncspace dev simulate-device --scenario flaky --trusted
go run ./backend/cmd/syncspace test-transfer
go run ./backend/cmd/syncspace export-diagnostics --output diagnostics.zip
go run ./backend/cmd/syncspace dev reset
```

See [TESTING.md](TESTING.md) for acceptance checks and
[LOCAL_SIMULATION.md](LOCAL_SIMULATION.md) for scripts and scenario behavior.

## Engineering rules

- Discovery reports presence; it never grants trust.
- Pairing management and developer controls remain loopback-only.
- Never mark a transfer complete before whole-file SHA-256 verification.
- Keep protocol models, persistence, API docs, mDNS capability fields, and UI
  projections aligned when a wire field changes.
- Add deterministic tests for failures and recovery; do not depend on physical
  devices in the default test suite.
