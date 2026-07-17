# Testing SyncSpace

Run all commands from the repository root.

## Complete acceptance sequence

```powershell
go test ./...
go vet ./...
cd frontend
npm ci
npm run check
cd ..
go run ./backend/cmd/syncspace dev verify
```

`npm run check` runs Vitest, TypeScript validation, and a production build. The
build regenerates the frontend assets embedded by Go.

`dev verify` builds and starts two isolated backend processes, checks both
management APIs and embedded entrypoints, completes signed X25519/Ed25519
pairing with the same verification code on both sides, sends a real staged file
over identity-pinned TLS, accepts it on the receiver, compares SHA-256, verifies
both persisted histories, and stops both processes.

## Backend coverage

```powershell
go test ./...
go vet ./...
go test -race ./...
```

Focused loops:

```powershell
go test ./backend/internal/services ./backend/internal/pairing
go test ./backend/internal/transfer ./backend/internal/database
go test ./backend/internal/api ./backend/internal/websocket
go test ./backend/internal/discovery ./backend/internal/settings
```

Tests cover identity persistence and private-key separation, certificate
pinning, signed pairing, dual confirmation, code agreement, expiry/rejection,
replay rejection, identity-hint changes, durable trust, migration upgrades,
authenticated offers, resumable and out-of-order chunks, retry/pause/cancel,
disk and checksum failures, conflict policies, restart recovery, local API
origin enforcement, settings validation, and diagnostics.

## Frontend coverage

```powershell
cd frontend
npm ci
npm test
npm run lint
npm run build
```

Vitest renders the actual React views and validates loading/empty states,
device trust projections, transfer controls, progress/history, diagnostics, API
parsing, and WebSocket event handling. The type build checks all page and API
contracts.

## Manual two-computer acceptance

1. Run `npm ci && npm run build` inside `frontend/` on each checkout.
2. Run `go run ./backend/cmd/server` on both computers on the same LAN.
3. Open `http://127.0.0.1:8384` locally on both. Confirm the peer appears only
   as discovered, not trusted.
4. Pair from one side. Confirm the six-digit code and fingerprint match on both
   screens, approve both, and check that **Verified** appears.
5. Transfer a small file and a nested folder. Approve the inbound offer and
   verify content at the destination.
6. Pause/resume a larger transfer, restart one process during queued work, and
   verify recovery/history.
7. Block the peer and confirm new offers fail; unblock it and confirm transfer
   works; forget it and confirm a new pairing is required.
8. Change Settings, restart, and confirm persistence. Export Diagnostics and
   review it for sensitive paths before sharing.

Windows can independently confirm a received file with:

```powershell
Get-FileHash <path> -Algorithm SHA256
```

On macOS/Linux use `sha256sum <path>` (or `shasum -a 256 <path>` on macOS).

## Browser QA status

Automated DOM tests and production builds run in CI. A manual browser pass
should cover desktop width, a narrow mobile viewport, keyboard focus, reduced
motion, system/light/dark appearance, long filenames, pairing expiry/errors,
offline devices, large queues, and incoming destination overflow. Record any
environment where a real browser was unavailable rather than claiming a visual
pass.

## CI

`.github/workflows/test.yml` runs on Linux and performs backend tests/vet/build,
frontend dependency install/tests/type validation/build, and the encrypted
two-process `dev verify` smoke test on every push and pull request.
