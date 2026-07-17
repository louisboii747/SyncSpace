# REST and WebSocket API

The local management API is versioned at `/api/v1` and is accepted only from a
loopback client with a matching browser origin. Existing unversioned aliases are
retained for migration. The peer API remains `/v1` on the TLS listener.

## Local management

### Runtime

- `GET /api/v1/health`
- `GET /api/v1/diagnostics`
- `GET /api/v1/diagnostics/export`
- `GET /api/v1/device/self`
- `GET /api/v1/devices`
- `POST /api/v1/discovery/refresh`
- `GET /api/v1/ws/discovery`

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

Queue body:

```json
{"deviceId":"...","paths":["C:\\example.txt"],"conflictPolicy":"rename"}
```

Incoming acceptance requires an absolute local `destinationPath` plus
`prompt`, `rename`, or `overwrite` conflict policy.

Browser staging uses:

- `POST /api/v1/transfers/staging`
- `PUT /api/v1/transfers/staging/{id}/files?path=<relative-path>`
- `POST /api/v1/transfers/staging/{id}/queue`
- `DELETE /api/v1/transfers/staging/{id}`

Only top-level staged roots can be promoted. Unsafe/duplicate paths are
rejected, bodies stream through bounded buffers, and unfinished sessions expire
after 24 hours.

### Settings

- `GET /api/v1/settings`
- `PUT /api/v1/settings`
- `POST /api/v1/settings/reset`

The complete settings document contains `appearance` (`system`, `light`, or
`dark`), `reducedMotion`, an absolute `defaultDownloadDirectory`,
`conflictPolicy`, and `notificationsEnabled`. Writes validate the whole
document and persist atomically.

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

## Errors

Invalid identifiers, manifests, indexes, origins, or paths return `400` or
`403`; missing/bad credentials return `401`; untrusted or blocked identities
return `403`; missing resources return `404`; identity/state/conflict errors
return `409`; rate limits return `429`; checksum failures return `422`; storage
exhaustion may return `507`; unexpected failures return `500` without exposing
internal paths on peer routes.
