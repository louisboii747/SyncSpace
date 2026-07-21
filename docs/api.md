# REST and WebSocket API

The local management API is versioned at `/api/v1` and is accepted only from a
loopback client with a matching browser origin. Existing unversioned aliases are
retained for migration. The peer API remains `/v1` on the TLS listener.

## Privacy gate

Policy version `2026-07-1` must be accepted before operational routes are
available. Before acceptance, the backend allows only:

- `GET /api/v1/health`
- `GET /api/v1/device/self`
- `GET /api/v1/settings`
- `GET /api/v1/privacy-policy`
- `POST /api/v1/privacy-policy/accept`

Other local discovery, pairing, diagnostics, WebSocket, staging, and transfer
routes return `403`. Peer `/v1/pairing/*` and `/v1/transfers/*` routes are gated
the same way. This is backend middleware; clients cannot bypass it by hiding the
first-launch page. Retained unversioned management aliases follow the same
method allowlist and cannot be used to change settings before acceptance.

## Local management

### Runtime

- `GET /api/v1/health`
- `GET /api/v1/diagnostics`
- `GET /api/v1/diagnostics/export`
- `GET /api/v1/device/self`
- `GET /api/v1/devices`
- `POST /api/v1/discovery/refresh`
- `GET /api/v1/ws/discovery`

The device projection keeps human naming and security identity separate:

```json
{
  "deviceId": "stable-random-uuid",
  "deviceName": "Louis' PC",
  "hostname": "LOUIS-PC",
  "deviceType": "desktop",
  "platform": "windows",
  "localIp": "192.168.1.249",
  "port": 8385,
  "online": true,
  "transferCapability": true,
  "supportedProtocolVersion": 1
}
```

`deviceName` is editable. `hostname` is the operating-system name stored with
the device identity. `deviceId` remains stable across display-name changes and
does not derive from the MAC address.

### Pairing and trust

- `GET /api/v1/pairing/requests`
- `GET /api/v1/pairing/requests/{requestId}`
- `POST /api/v1/pairing/request` with `{"deviceId":"..."}`
- `POST /api/v1/pairing/accept` with `{"requestId":"..."}`
- `POST /api/v1/pairing/reject` with `{"requestId":"..."}`
- `GET /api/v1/pairing/trusted-devices`
- `POST /api/v1/pairing/trusted-devices/{deviceId}/block`
- `POST /api/v1/pairing/trusted-devices/{deviceId}/unblock`
- `DELETE /api/v1/pairing/trusted-devices/{deviceId}`
- `GET /api/v1/ws/pairing`

The request projection includes direction, state, verification code, complete
remote fingerprint, and local/remote confirmation state. Trust responses expose
public keys/fingerprints but never the derived shared pairing credential.

### Transfers

- `POST /api/v1/transfers`
- `GET /api/v1/transfers` and `GET /api/v1/transfers/{id}`
- `POST /api/v1/transfers/{id}/accept`, `/reject`, `/pause`, `/resume`,
  `/cancel`, or `/retry`
- `DELETE /api/v1/transfers/history`
- `GET /api/v1/ws/transfers`

The pause/resume/retry routes expose local backend state transitions. The
current peer protocol does not coordinate receiver-initiated pause or retry back
to the sender, so the embedded UI presents those actions only for outgoing
transfers. Receivers can accept, reject, or cancel through the supported flow.

Queue body:

```json
{"deviceId":"...","paths":["C:\\example.txt"],"conflictPolicy":"rename"}
```

Incoming acceptance requires an absolute local `destinationPath` plus
`prompt`, `rename`, or `overwrite` conflict policy.

Acceptance creates/resolves the destination and checks its reported free space
against the transfer's expected byte count before moving to `Receiving`. If the
volume reports less space than required, the route returns `507` with a compact
error containing required and available bytes. Platforms that cannot report
free space do not receive a fabricated value.

When `incomingTransfersEnabled` is false, `POST /v1/transfers/offers` returns
`403` with `{"error":"this device is not accepting new transfer offers"}`
before a new offer or file body is accepted. Existing transfer state is not
deleted by that setting.

Browser staging uses:

- `POST /api/v1/transfers/staging`
- `PUT /api/v1/transfers/staging/{id}/files?path=<relative-path>`
- `POST /api/v1/transfers/staging/{id}/queue`
- `DELETE /api/v1/transfers/staging/{id}`

Only top-level staged roots can be promoted. Unsafe/duplicate paths are
rejected, bodies stream through bounded buffers, and unfinished sessions expire
after 24 hours.

The staging API has no empty-directory request and does not accept modification
timestamps or filesystem permissions. A browser-staged folder therefore keeps
file-relative paths but omits empty directories and those metadata fields.

### Settings

- `GET /api/v1/settings`
- `PUT /api/v1/settings`
- `POST /api/v1/settings/reset`

The complete settings document contains `appearance` (`system`, `light`, or
`dark`), `reducedMotion`, an absolute `defaultDownloadDirectory`,
`conflictPolicy`, `notificationsEnabled`, `deviceName`, `discoverable`, and
`incomingTransfersEnabled`. It also returns the read-only acceptance fields
`privacyPolicyVersion` and `privacyAcceptedAt`.

`PUT /api/v1/settings` validates and atomically persists the whole document.
It cannot forge, clear, or replace policy acceptance; those values are preserved
from the store. `POST /api/v1/settings/reset` restores ordinary defaults while
preserving the current device name and accepted policy record.

New profiles default the download directory to
`<home>/Downloads/SyncSpace`; migrations preserve an existing chosen path.
Conflict policy defaults to `rename`, and discovery and new incoming offers
default to enabled after policy acceptance. Updating `deviceName` refreshes the
mDNS advertisement and future authenticated pairing/transfer messages without
changing the stable device ID. Updating `discoverable` starts or stops mDNS
dynamically.

### Privacy policy

- `GET /api/v1/privacy-policy`
- `POST /api/v1/privacy-policy/accept` with `{"version":"2026-07-1"}`

The policy response contains `version`, `effectiveDate`, `summary`,
presentation-neutral `sections`, `accepted`, and `acceptedAt` when accepted.
Acceptance of a stale or unknown version returns `409`; the client must fetch
the current policy again.

## Peer pairing protocol

These routes are served through TLS 1.3 on the discovered peer port:

- `POST /v1/pairing/requests`
- `POST /v1/pairing/proof`

Messages carry signed key-exchange material or HMAC proof material, timestamps,
and nonces. Body limits are enforced. They are protocol endpoints, not browser
management endpoints.

## Peer transfer protocol

- `POST /v1/transfers/offers`
- `GET /v1/transfers/{id}/status`
- `PUT /v1/transfers/{id}/files/{fileId}/chunks/{index}`
- `POST /v1/transfers/{id}/complete`
- `DELETE /v1/transfers/{id}`

The offer uses pairing-key HMAC headers and a one-use nonce. Subsequent routes
require the per-transfer bearer credential. Every request is sent over a peer
certificate pinned to the trusted Ed25519 key. See
[transfer-protocol.md](transfer-protocol.md).

The offer projection includes the authenticated sender `deviceId`, `deviceName`,
and optional `deviceHostname`/`devicePlatform` presentation fields. Hostname and
platform persist in the durable transfer so the incoming review survives a
refresh. They are optional for compatibility with stored or older protocol-v1
offers; missing values are omitted from the review, never invented. Trust
status comes from successful offer authorisation against the local paired-device
record, not from a sender-supplied boolean.

## Errors

Invalid identifiers, manifests, indexes, origins, or paths return `400` or
`403`; missing/bad credentials return `401`; untrusted or blocked identities
return `403`; missing resources return `404`; identity/state/conflict errors
return `409`; rate limits return `429`; checksum failures return `422`; storage
exhaustion may return `507`; unexpected failures return `500` without exposing
internal paths on peer routes.

Current handlers return a compact JSON error such as `{"error":"message"}`.
Clients should use the HTTP status and show the message in plain language; a
nested machine-code error envelope is not implemented yet.
