# SyncSpace

SyncSpace is a local-first device transfer service. Each device owns its
identity, discovers nearby peers over mDNS, pairs through a user-verified
cryptographic handshake, and transfers files directly over identity-pinned TLS.
There is no account, cloud storage, relay, analytics service, or internet
fallback in the current product.

The runnable product today is a Go engine with a responsive React control
surface embedded into the same executable. It provides Home, Transfers,
Devices, History, Settings, and Diagnostics views. Linux and Windows users can
download that complete runtime from a GitHub Release without installing Go or
Node.js.

## Install a Windows release

GitHub Releases provide one ready-to-run Windows application for x64 and one
for Arm64. Download `syncspace-VERSION-windows-ARCH.exe` and run it; there are
no companion, CLI, or archive assets. Windows builds are currently unsigned,
so GitHub may show a SmartScreen warning. See
[SyncSpace for Windows](windows/README.md) for exact commands and limitations.

## Install a Linux release

GitHub Releases provide native DEB and RPM packages for x86-64 and 64-bit ARM.
Download the package for the computer, then install it with `apt` or `dnf`.

```sh
# Debian or Ubuntu, x86-64 example
sudo apt install ./syncspace_VERSION_amd64.deb

# Fedora or RHEL family, x86-64 example
sudo dnf install ./syncspace-VERSION-1.x86_64.rpm
```

Open **SyncSpace** from the application menu or run `syncspace`. A dedicated
native Wails window owns the private backend lifecycle; it does not open a
browser or display a localhost address. The first launch still requires the
privacy policy to be accepted before LAN activity begins.

See [Install SyncSpace on Linux](docs/linux-packages.md) for architecture
selection, provenance verification, dependencies, upgrades, and uninstall
instructions. GitHub Releases is not an APT/DNF
repository, so upgrades are downloaded and verified explicitly.

## Start SyncSpace from source

Source-build requirements: Go 1.26.4 or newer, Node.js 22 or newer, and npm.

From the repository root, build the frontend, then build and start the
self-contained native desktop application:

```powershell
cd frontend
npm ci
npm run build
cd ..
go build -o build/bin/syncspace.exe ./backend/cmd/desktop
.\build\bin\syncspace.exe
```

On Linux, omit `.exe`. The desktop window prepares:

- the loopback-only management API and embedded frontend on port `8384`;
- the TLS 1.3 LAN peer listener on port `8385`;
- pairing, transfer workers, WebSockets, SQLite, and diagnostics.

Stop it with `Ctrl+C`. On first launch SyncSpace creates a device identity and
data directory under the operating system's user configuration directory.

## Apple platform foundation

Native SwiftUI foundations for iOS and macOS live in [`apple`](apple/README.md).
They share API models, async client state, and Bonjour discovery and can be
edited on Windows. Xcode on macOS is required to generate, sign, simulate, and
archive the final applications.

## First launch and privacy

The first page is the privacy policy, version `2026-07-1`. Read it and choose
**Accept and continue** before using the rest of SyncSpace. Acceptance records
the policy version and UTC timestamp in `settings.json`. SyncSpace asks again if
the shipped policy version changes or its local data is reset.

This is enforced by the Go backend, not only by the page. Until the current
policy is accepted:

- mDNS advertising and discovery do not run;
- pairing and transfer management calls return `403 Forbidden`;
- peer pairing and transfer routes return `403 Forbidden`;
- transfer workers remain stopped.

The local UI, health check, device identity, settings, and policy endpoints stay
available so the policy can be read and accepted. Choosing **Decline** keeps
SyncSpace paused; it does not silently enable networking.

The current policy is available later under **Settings > Privacy** and in
[PRIVACY.md](PRIVACY.md).

## Develop the frontend with hot reload

Use two terminals. Keep the Go server running in the first:

```powershell
go run ./backend/cmd/server
```

Run Vite in the second:

```powershell
cd frontend
npm ci
npm run dev
```

Open <http://127.0.0.1:5173>. Vite proxies the versioned REST and WebSocket
requests to the backend at `127.0.0.1:8384`. Run `npm run build` before using
the embedded UI again; that command regenerates the assets compiled into Go.

## Try two devices on one computer

The local lab builds and starts two isolated real backend processes, accepts the
current policy for those temporary lab profiles, performs the same signed
pairing protocol used on a LAN, and keeps both running:

```powershell
go run ./backend/cmd/syncspace dev start
```

Open Device A at <http://127.0.0.1:8384> and Device B at
<http://127.0.0.1:8385>. Their encrypted peer listeners use ports `18384` and
`18385`. Press `Ctrl+C` in the lab terminal to stop both.

Run a complete non-interactive acceptance test with:

```powershell
go run ./backend/cmd/syncspace dev verify
```

It compares both pairing codes, confirms both sides, transfers a real file over
pinned TLS, verifies the destination SHA-256 and both histories, then stops.
Reset only the lab data with `go run ./backend/cmd/syncspace dev reset`.

## Use SyncSpace on two computers

1. Build and run the server on both computers connected to the same local
   network. Allow the SyncSpace peer port (`8385/TCP`) through the host firewall
   if the operating system asks.
2. Open the SyncSpace desktop window on each computer. Read and accept the
   privacy policy on both devices. Neither device is advertised before this.
3. Open **Devices**. Discovery can show a device, but never trusts it. Cards use
   the editable SyncSpace name as the main label and show the operating-system
   hostname separately so the computer is easier to recognise.
4. Choose **Pair device** on one computer. Compare the six-digit code and full
   identity fingerprint shown on both computers, then confirm on both.
5. Open **Transfers** on the sender and choose a device marked **Ready**. Choose
   files or a folder, or drop them into the window. Review the destination,
   relative file names, individual sizes, total size, and any executable or
   script warning. Add more files, add another folder, remove an item, or clear
   the selection as needed. Nothing is staged or sent until **Send files** is
   chosen.
6. Approve the incoming transfer on the receiver. Check the sender's display
   name, hostname, platform, trusted status, item count, total size, bounded
   per-file preview, and any executable/script warning; choose the save folder
   and name-conflict behaviour, then accept or decline. Progress comes from
   backend byte counts and completion is shown only after SHA-256 verification
   succeeds. Before acceptance, the receiver checks that the destination volume
   reports enough free space for the expected bytes. Older stored offers may not
   have hostname/platform details.

**Settings** lets you rename the device without changing its stable ID or trust
relationships. It also controls mDNS discoverability, whether new incoming
offers are allowed, appearance, reduced motion, the default receive folder,
the default conflict policy, and notifications. Turning discoverability off
stops the local mDNS discovery session and withdraws its advertisement. Turning
incoming offers off rejects new offers before file data is accepted. These are
separate controls and neither one deletes trusted devices or transfer history.

A new profile's default receive folder is `Downloads/SyncSpace` inside the
current user's home directory (`Downloads\SyncSpace` on Windows). Existing
profiles keep their chosen path during settings migration. SyncSpace creates the
folder when an incoming transfer is accepted. **Keep both files** is the safe
default for name conflicts; it renames an incoming file rather than overwriting
an existing one.

**Devices** can block, unblock, or forget a paired identity. **History** can
clear completed, cancelled, and failed records without deleting received files.
**Diagnostics** can check health, refresh discovery, inspect runtime paths and
logs, and export a redacted-by-design ZIP for support; review local filenames
and paths before sharing it.

## Device name, hostname, and stable identity

These values have different jobs:

- **Device name** is the friendly, editable label shown to people nearby. A new
  profile starts with the best available operating-system hostname or a
  platform fallback. It is stored in `settings.json`.
- **Hostname** is the operating-system computer name captured in the device
  identity. It is shown separately and is not changed by renaming SyncSpace.
- **Device ID** is a random UUID created once. It does not contain the MAC
  address or another hardware identifier. Renaming the device does not change
  it or invalidate an existing trusted-device record.
- **Cryptographic identity** is the Ed25519 key pair used to prove that the same
  SyncSpace installation has returned.

The public metadata and stable device ID live in `identity.json`; private key
material lives separately in `identity.key`. With the default data root these
files are under `%APPDATA%\SyncSpace` on Windows,
`~/Library/Application Support/SyncSpace` on macOS, and
`${XDG_CONFIG_HOME:-$HOME/.config}/SyncSpace` on Linux. `SYNCSPACE_DATA_DIR` replaces
that root when set. Windows protects `identity.key` with user-scoped DPAPI;
other current builds restrict it to mode `0600`.

## Security model

- Long-term Ed25519 device identities are created locally. Windows protects the
  private key with user-scoped DPAPI; other current builds use a mode-`0600`
  key file until their native keystore adapters exist.
- Pairing signs an ephemeral X25519 exchange with both device identities,
  derives a shared key with HKDF-SHA-256, and requires the same six-digit code
  to be approved on both devices.
- Peer traffic uses TLS 1.3 and pins the certificate key to the paired identity.
  Initial transfer offers are HMAC-authenticated with timestamps, nonces, and
  replay protection; transfer sessions then use random bearer credentials over
  the pinned channel.
- Management APIs and the React UI bind only to loopback. LAN listeners expose
  peer protocol routes, not the local management surface.
- Incoming files remain private partials until chunk and whole-file SHA-256
  verification succeeds.

See [SECURITY.md](SECURITY.md), [PRIVACY.md](PRIVACY.md), and the
[architecture guide](docs/architecture.md) for boundaries and known limits.

## Verify a change

```powershell
go test ./...
go vet ./...
cd frontend
npm run check
cd ..
go run ./backend/cmd/syncspace dev verify
```

CI repeats the backend tests/vet/build, frontend tests/type-check/build, and the
two-process encrypted transfer smoke test. Tagged release CI additionally
builds and inspects both DEB/RPM architectures, verifies the embedded version,
records GitHub build-provenance attestations, and attaches only ready-to-run or
ready-to-install applications to the matching GitHub Release.

## Current scope

Implemented now: the versioned first-launch privacy gate, mDNS discovery,
separate display name and hostname, stable identity, verified pairing, durable
trust/block/forget, encrypted authenticated file/folder transfer, sender and
receiver review, executable/script warnings, incoming approval, outgoing
pause/resume/retry, cancellation, crash recovery, conflict handling, history,
privacy and receive settings, diagnostics, streamed local staging, versioned
management APIs, WebSockets, SQLite migrations, and the embedded responsive
React experience.

Current transfer limits are deliberately visible:

- webview folder selection uploads files and relative paths, but cannot retain
  empty directories because the current selection bridge does not provide a body for
  them;
- modification timestamps and file permissions are not preserved by the
  current transfer manifest;
- the embedded UI does not yet provide native **Open**, **Reveal in folder**, or
  **Copy path** actions after receipt;
- pause/resume/retry controls are currently sender-side. A receiver can accept,
  decline, or cancel, but cannot coordinate pause or retry from its UI;
- the sender review is based on the selected file list and cannot predict
  destination conflicts before the receiver checks its filesystem.

The Linux DEB/RPM and Windows portable distributions now use a native Wails
desktop shell with single-instance ownership and a bundled React UI. Not yet
implemented: a signed Windows installer/updater, Linux repository-based automatic
updates, tray integration, buildable
Android/iOS/iPadOS/macOS clients, native share sheets or file providers,
QR/manual-IP pairing, clipboard/notes sync, camera workflows, accessibility
certification, relay/remote transfer, or the later collaboration features in
the product brief. The platform folders document intended native directions;
they are not currently runnable applications.

The detailed command reference is in [Development](docs/development.md), the
lab walkthrough is in [Local simulation](docs/LOCAL_SIMULATION.md), and the
full acceptance sequence is in [Testing](docs/TESTING.md).

## License

Apache 2.0
