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

`dev verify` builds and starts two isolated backend processes, waits for each
policy endpoint, accepts privacy policy `2026-07-1` for both temporary profiles
through the real local API, and then requires both health checks and embedded
entrypoints to pass. It completes signed X25519/Ed25519 pairing with the same
verification code on both sides, sends a real staged file over identity-pinned
TLS, accepts it on the receiver, compares SHA-256, verifies both persisted
histories, and stops both processes. A normal server start does not auto-accept
the policy.

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

The settings/API suites also cover policy version acceptance, attempts to forge
acceptance through ordinary settings writes, schema-v1 migration without silent
acceptance, the pre-acceptance route gate for versioned and retained legacy
aliases, and persisted discovery/incoming controls.

Migration/store coverage checks schema v4's `device_hostname` and
`device_platform` columns and verifies that both optional values survive a
durable transfer round trip.

Receiver acceptance coverage also creates an offer larger than the destination
volume's reported free space and verifies `ErrInsufficientStorage` before the
transfer enters `Receiving`; the API maps that error to HTTP `507`.

### Race detector requirement

`go test -race ./...` requires CGO and a working C compiler. Check the host
before treating the command as a quality-gate result:

```powershell
go env CGO_ENABLED
go env CC
$env:CGO_ENABLED = "1"
go test -race ./...
```

If Go reports that `-race requires cgo` or the compiler is unavailable, the race
suite did not run; that is a documented environment limitation, not a pass.
Install a supported C toolchain or run the command on a CGO-enabled Linux/Windows
builder. The current GitHub Actions workflow runs ordinary tests, vet, builds,
and the end-to-end smoke test, but does not currently include `go test -race`.

## Frontend coverage

```powershell
cd frontend
npm ci
npm test
npm run lint
npm run build
```

The current Vitest suite covers the backend loading state, device name/hostname
and pairing projections, compatibility and empty states, progress and terminal
states, direction-appropriate actions, sender review, incoming manifest and
executable warnings, privacy-gate rendering, diagnostics rendering, transfer
formatting/browser-path helpers, and transfer WebSocket event parsing. The
TypeScript/production build checks the complete page and API contracts.

The view tests primarily render markup. Policy acceptance/version-renewal
interactions, actual add/remove selection behaviour, Settings persistence,
history clearing, notification permission, and reconnect timing still need
interaction-focused component tests. Keep those cases in the manual checklist
below and do not describe the frontend UX as fully automated until they exist.

## Manual two-computer acceptance

1. Run `npm ci && npm run build` inside `frontend/` on each checkout.
2. Run `go run ./backend/cmd/server` on both computers on the same LAN.
3. Open `http://127.0.0.1:8384` locally on both. Before accepting, confirm the
   policy page is shown and the other computer is neither advertised nor
   discovered. Accept version `2026-07-1` on both.
4. Confirm each device card shows a friendly display name and the actual
   hostname separately.
5. Pair from one side. Confirm the six-digit code and fingerprint match on both
   screens, approve both, and check that **Verified** appears.
6. Rename one device in Settings. Confirm the nearby card updates while the
   hostname, stable device ID, and trusted relationship remain unchanged.
7. Transfer a small file, a nested folder, and a harmless file whose name ends
   in a script/executable extension. Confirm the sender review shows the right
   destination, paths, total, and warning. Add and remove an item, then confirm
   nothing is staged until **Send files** is chosen. Confirm the receiver shows
   sender display name, hostname, platform, trusted badge, item count, total,
   scrollable paths, warning, destination, and conflict choice. Also load a
   legacy offer without hostname/platform and confirm the UI omits those details
   rather than inventing values. Approve the transfer and verify content at the
   destination.
8. From the sender, pause/resume a larger transfer and retry a failed transfer.
   Confirm the receiver offers accept/decline/cancel but does not claim to
   support coordinated pause or retry. Restart one process during queued work
   and verify recovery/history.
9. Turn incoming offers off and confirm a new offer receives a clear rejection.
   Turn discoverability off and confirm the mDNS advertisement disappears;
   restore both controls before continuing.
10. Block the peer and confirm new offers fail; unblock it and confirm transfer
   works; forget it and confirm a new pairing is required.
11. Change the default receive directory, restart, and confirm it persists. Clear
    history and confirm received files remain. Export Diagnostics and review it
    for sensitive paths before sharing.

Windows can independently confirm a received file with:

```powershell
Get-FileHash <path> -Algorithm SHA256
```

On macOS/Linux use `sha256sum <path>` (or `shasum -a 256 <path>` on macOS).

## Browser QA status

The current Vitest suite and production build run in CI. A manual browser pass
should cover desktop width, a narrow mobile viewport, keyboard focus, reduced
motion, system/light/dark appearance, long filenames, pairing expiry/errors,
offline devices, large queues, send-review and incoming-destination overflow,
friendly copy, and keyboard use through each modal. Record any environment
where a real browser was unavailable rather than claiming a visual pass.

Manual transfer results should note current scope: browser-staged folders omit
empty directories, transfer metadata does not preserve modification timestamps
or permissions, and the embedded UI has no native open/reveal/copy-path action.

## CI

`.github/workflows/ci.yml` runs on Linux and performs backend formatting,
tests/vet/build, frontend dependency install/tests/type validation/build, the
encrypted two-process `dev verify` smoke test, and an amd64 DEB/RPM package
smoke test on pushes and pull requests targeting `main`.

## Linux package acceptance

On a Linux packaging host, build both formats and run the read-only verifier:

```sh
packaging/linux/build-packages.sh \
  --version 1.4.0 \
  --arch amd64 \
  --format all \
  --output dist
packaging/linux/verify-package.sh dist/syncspace_1.4.0_amd64.deb
packaging/linux/verify-package.sh dist/syncspace-1.4.0-1.x86_64.rpm
```

Repeat on an ARM64 Linux host for `arm64`, producing an `arm64` DEB and
`aarch64` RPM, or rely on the release matrix's native ARM64 runner. Verification
must inspect metadata, version, architecture, payload paths, ownership,
permissions, the self-contained native desktop executable, desktop entry, and icon.
It must not need to install the package or start a root service.

The tag-triggered `.github/workflows/release-linux.yml` workflow repeats the
normal frontend and Go gates, builds every package twice to check deterministic
output, verifies both architectures, exercises each extracted native server
through its real health endpoint, generates one `SHA256SUMS`, runs
`sha256sum --check`, and creates GitHub artifact attestations before publishing.
A manual workflow dispatch validates artifacts but must not publish a release.

Before announcing a release, install a DEB and an RPM on clean supported
systems and confirm:

1. installation does not start SyncSpace;
2. the menu entry and `syncspace` open one native window without a browser;
3. a second launch focuses the existing window and creates no second backend;
4. privacy acceptance is still required before LAN activity;
5. closing the window stops its embedded backend;
6. installing a newer package preserves identity and transfer state; and
7. package removal leaves per-user application data and received files intact.

See [Linux packages](linux-packages.md) for the end-user commands and
[Releasing](releasing.md) for the complete asset and provenance contract.
