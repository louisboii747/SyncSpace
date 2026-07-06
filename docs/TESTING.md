# Testing SyncSpace

All commands below run from the repository root unless stated otherwise.

## Complete local acceptance check

```sh
go test ./...
go vet ./...
cd frontend && npm run check && cd ..
go run ./backend/cmd/syncspace dev verify
```

`dev verify` is the end-to-end test. It builds the server, creates isolated
Device A and Device B data roots, starts both backend processes, waits for both
health endpoints, explicitly pairs both directions, stages and queues a file,
confirms both embedded React entrypoints, accepts it on Device B, waits for
100%, compares SHA-256, confirms the persisted transfer state, then stops both
devices. It requires no physical device.

## Backend tests

```sh
go test ./...
go vet ./...
```

Focused loops:

```sh
go test ./backend/internal/transfer
go test ./backend/internal/devsim ./backend/internal/diagnostics
go test ./backend/internal/api ./backend/internal/websocket
```

The suite covers tiny and multi-file/folder transfers, a large compressed file,
out-of-order chunks, cancellation, pause/resume, interrupted and flaky peers,
retry, rejection/acceptance, disk-full faults, corrupt chunks, final checksum
mismatch, overwrite/rename behavior, SQLite history, and restart recovery.

## Frontend tests and validation

```sh
cd frontend
npm ci
npm run lint
npm test
npm run build
```

Vitest renders the real React views and validates discovery lists, transfer
queues, progress changes, failed/completed states, diagnostics, loading/empty
states, and WebSocket event parsing. `npm run build` also regenerates the assets
embedded by the Go server.

## Run two local devices interactively

```sh
go run ./backend/cmd/syncspace dev start
```

Open Device A at `http://127.0.0.1:8384` and Device B at
`http://127.0.0.1:8385`. Each has a fixed unique device ID and its own
`identity.json`, `syncspace.db`, staging root, receive root, and transfer store
under `.syncspace-dev/device-a` or `.syncspace-dev/device-b`.

To run them in separate terminals instead:

```powershell
.\scripts\dev-device-a.ps1
.\scripts\dev-device-b.ps1
```

On macOS/Linux use `sh scripts/dev-device-a` and `sh scripts/dev-device-b`.

## Send a transfer in the running lab

```sh
go run ./backend/cmd/syncspace test-transfer
```

Seed data can be created separately with `go run
./backend/cmd/syncspace dev seed`. The destination is
`.syncspace-dev/device-b/received/tiny.txt`.

## Test with two physical devices

1. Build the frontend with `cd frontend && npm ci && npm run build` on each
   machine, then return to the repository root.
2. Run `go run ./backend/cmd/server` on both machines while they are on the same
   trusted LAN.
3. Open `http://127.0.0.1:8384` on each machine and confirm the other device is
   shown under Devices.
4. Choose **Trust device** independently on both machines. Discovery alone does
   not grant trust.
5. On the sender, choose the peer and queue a file. On the receiver, approve the
   incoming transfer and choose a destination path.
6. Confirm both UIs report Completed, the received file exists, and its SHA-256
   matches (`Get-FileHash <path> -Algorithm SHA256` on PowerShell or `sha256sum
   <path>` on macOS/Linux).

Peer transport is not yet encrypted/authenticated beyond the current trust and
session-token checks; use a LAN you control.

## Reset test data

Stop the local lab first, then run:

```sh
go run ./backend/cmd/syncspace dev reset
```

This command only removes the repository-local `.syncspace-dev` directory and
refuses unexpected paths. The Diagnostics page's **Clear test data** action
clears completed/cancelled history and removes simulated peers and their trust
records without deleting normal device identity.

## Logs and diagnostics

Backend logs are human-readable on stdout. Open the Diagnostics page for a
bounded log preview and last-error list, run `go run
./backend/cmd/syncspace doctor`, or export a ZIP with:

```sh
go run ./backend/cmd/syncspace export-diagnostics --output diagnostics.zip
```

See [DIAGNOSTICS.md](DIAGNOSTICS.md) for endpoint details and redaction notes.

## CI

`.github/workflows/test.yml` runs backend tests/vet/build, frontend type
validation/tests/build, and `dev verify` on Linux. The smoke test uses two real
backend processes but only loopback networking and deterministic static peer
records, which avoids mDNS timing as an acceptance dependency.
