# SyncSpace

SyncSpace is a local-first device transfer service. Each device owns its identity,
discovers nearby peers over mDNS, pairs through a user-verified cryptographic
handshake, and transfers files directly over identity-pinned TLS. There is no
account, cloud storage, relay, analytics service, or internet fallback in the
current product.

The runnable product today is a Go engine with a responsive React control
surface embedded into the same executable. It provides Home, Transfers,
Devices, History, Settings, and Diagnostics views.

## Start SyncSpace

Requirements: Go 1.26.4 or newer, Node.js 22 or newer, and npm.

From the repository root, build the frontend once and start the server:

```powershell
cd frontend
npm ci
npm run build
cd ..
go run ./backend/cmd/server
```

Open <http://127.0.0.1:8384>. This one command starts:

- the loopback-only management API and embedded frontend on port `8384`;
- the encrypted LAN peer service on port `8385`;
- mDNS discovery, pairing, transfer workers, WebSockets, SQLite, and diagnostics.

Stop it with `Ctrl+C`. On first launch SyncSpace creates a device identity and
data directory under the operating system's user configuration directory.

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

The local lab builds and starts two isolated real backend processes, performs
the same signed pairing protocol used on a LAN, and keeps both running:

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
2. Open `http://127.0.0.1:8384` locally on each computer.
3. Open **Devices**. Discovery can show a device, but never trusts it.
4. Choose **Pair securely** on one device. Compare the six-digit code and the
   identity fingerprint shown on both computers, then confirm on both.
5. Open **Transfers** on the sender, select the verified device, and choose or
   drop files/folders. The browser streams the selection into private local
   staging before it enters the durable queue.
6. Approve the incoming transfer on the receiver. Choose its destination and
   conflict policy. Progress, pause/resume, cancel/retry, verification, and
   history remain available after restart.

**Settings** persists appearance, reduced motion, the default receive folder,
the default conflict policy, and notification preference. **Devices** can block,
unblock, or forget a paired identity. **Diagnostics** can check health, refresh
discovery, inspect runtime paths and logs, and export a redacted-by-design ZIP
for support; review local filenames and paths before sharing it.

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
two-process encrypted transfer smoke test.

## Current scope

Implemented now: mDNS discovery, stable identity, verified pairing, durable
trust/block/forget, encrypted authenticated file/folder transfer, incoming
approval, pause/resume/cancel/retry, crash recovery, conflict handling, history,
settings, diagnostics, local browser staging, versioned management APIs,
WebSockets, SQLite migrations, and the embedded responsive React experience.

Not yet implemented: packaged native desktop shells, installers/updaters/tray
integration, buildable Android/iOS/iPadOS/macOS clients, native share sheets or
file providers, QR/manual-IP pairing, clipboard/notes sync, camera workflows,
accessibility certification, relay/remote transfer, or the later collaboration
features in the product brief. The platform folders document intended native
directions; they are not currently runnable applications.

The detailed command reference is in [Development](docs/development.md), the
lab walkthrough is in [Local simulation](docs/LOCAL_SIMULATION.md), and the
full acceptance sequence is in [Testing](docs/TESTING.md).

## License

Apache 2.0
