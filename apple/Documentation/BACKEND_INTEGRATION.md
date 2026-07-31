# Go backend integration

The backend defaults to TCP `8384`. The local management surface is available both at its direct routes and under `/api/v1` through router re-dispatch.

## Confirmed management routes

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/device/self` | Local device model |
| GET | `/api/v1/devices` | Discovered devices |
| POST | `/api/v1/discovery/refresh` | Request refresh; returns 202 |
| GET | `/api/v1/pairing/requests` | Active requests |
| POST | `/api/v1/pairing/request` | Body `{ "deviceId": "..." }` |
| POST | `/api/v1/pairing/accept` | Body `{ "requestId": "..." }` |
| POST | `/api/v1/pairing/reject` | Body `{ "requestId": "..." }` |
| GET/DELETE | `/api/v1/pairing/trusted-devices[/:deviceId]` | List or forget trust |
| GET/POST | `/api/v1/transfers` | List or queue native filesystem paths |
| POST | `/api/v1/transfers/:id/{accept,reject,pause,resume,cancel,retry}` | Transfer state actions |
| GET | `/api/v1/ws/discovery`, `/api/v1/ws/pairing`, `/api/v1/ws/transfers` | Event streams after re-dispatch |

The direct WebSocket paths are `/ws/discovery`, `/ws/pairing`, and `/ws/transfers`. Events use `{ "type", <feature payload>, "timestamp" }`. Discovery types include `DeviceDiscovered`, `DeviceUpdated`, `DeviceOffline`, `DeviceRemoved`; pairing includes `PairingRequested`, `PairingAccepted`, `PairingRejected`, `TrustedDeviceRemoved`; transfer includes `QueueUpdated`, `Progress`, `Pause`, `Resume`, `Complete`, `Failure`, `Verification`.

Device payloads use `deviceId`, `deviceName`, `deviceType`, `platform`, `localIp`, `port`, `appVersion`, `lastSeen`, and `online`. Transfer IDs use `uuid`; backend progress is an integer percentage, speed is bytes/second, and ETA is `etaSeconds`. Pairing requests use `requestId`, `deviceId`, `deviceName`, `verificationCode`, `requestedAt`, `expiresAt`, `state`, and `direction`.

## Current blocker

All management routes and sockets above call `localOnly()`, which rejects non-loopback remote addresses. Therefore an Apple client cannot currently manage a Go backend on a Windows LAN host. Do not bypass this with the peer `/v1/pairing` or `/v1/transfers` routes: those are authenticated device-to-device protocol surfaces, not a general UI API. A future backend change needs security review and authenticated authorization before exposing management remotely.

Clipboard and notes management routes do not exist. Their centralized Swift endpoint entries are marked `TODO`. Native file selection also needs a defined upload contract: `/transfers` accepts filesystem paths local to the backend, while `/transfers/staging` is a loopback browser staging flow.

## Testing and inspection

On Windows, production-like startup is:

```powershell
cd frontend
npm ci
npm run build
cd ..
go run ./backend/cmd/server
```

Inspect a local response with `Invoke-RestMethod http://127.0.0.1:8384/api/v1/device/self`. Use backend logs and browser developer tools to inspect responses. The repository's strongest two-device smoke test is `go run ./backend/cmd/syncspace dev verify`.

After an authenticated remote management design exists, permit TCP 8384 through the Windows firewall only on the private LAN profile, listen on a LAN-accessible interface, and test both IPv4 and bracketed IPv6. `.local` resolution depends on mDNS availability and may be affected by client isolation. When fields change, update explicit Swift `CodingKeys`, add a captured JSON fixture test, and retain tolerant decoding for optional versioned fields.
