# REST and WebSocket API

Local management routes reject non-loopback clients.

## Health and diagnostics

- `GET /health` is loopback-only and returns component checks. A degraded
  result uses HTTP 503.
- `GET /diagnostics` returns the loopback-only runtime snapshot.
- `GET /diagnostics/export` returns a loopback-only ZIP bundle.
- `POST /diagnostics/test-transfer` queues a generated seed file to the first
  suitable trusted online peer.
- `POST /diagnostics/simulate-failure` creates and queues to an interrupted peer
  in developer mode.
- `POST /diagnostics/clear` clears test history and simulated peers/trust.

Developer mode also exposes loopback-only `GET|POST
/dev/simulated-devices` and `DELETE /dev/simulated-devices/{id}`. POST accepts
`name`, one of the documented simulator `scenario` values, and `trusted`.

## Transfer management

- `POST /transfers` queues `{"deviceId":"...","paths":["..."],
  "conflictPolicy":"prompt|overwrite|rename"}`.
- `GET /transfers` returns queue, active work, failures, and history.
- `GET /transfers/{id}` and `GET /transfers/{id}/status` return details.
- `POST /transfers/{id}/accept` accepts an inbound offer with
  `destinationPath` and `conflictPolicy`.
- `POST /transfers/{id}/reject` rejects an unapproved inbound offer.
- `POST /transfers/{id}/pause`, `/resume`, `/cancel`, and `/retry` perform the
  named state transition.
- `DELETE /transfers/history` deletes completed and cancelled history.
- `GET /ws/transfers` streams a snapshot followed by live events.

### Browser staging

Browser sandboxes do not expose absolute filesystem paths. The embedded React
client therefore uses these loopback-only streaming routes:

- `POST /transfers/staging` creates an isolated upload session.
- `PUT /transfers/staging/{id}/files?path=<portable-relative-path>` streams one
  file. `Content-Length` is required; duplicate and unsafe paths are rejected.
- `POST /transfers/staging/{id}/queue` accepts `deviceId`, `roots`, and
  `conflictPolicy`, seals the session, and creates a normal durable transfer.
- `DELETE /transfers/staging/{id}` removes an incomplete session.

Only top-level staged roots can be queued. Unfinished sessions expire after 24
hours. File bodies stream through a bounded 1 MiB buffer.

Transfer WebSocket event types are `QueueUpdated`, `Progress`, `Pause`,
`Resume`, `Complete`, `Failure`, and `Verification`. Every envelope contains the
full transfer projection and a UTC timestamp. Speed is bytes/second; ETA is
seconds. Slow clients are disconnected and can reconnect for a fresh snapshot.

## Peer protocol

- `POST /v1/transfers/offers`
- `GET /v1/transfers/{id}/status`
- `PUT /v1/transfers/{id}/files/{fileId}/chunks/{index}`
- `POST /v1/transfers/{id}/complete`
- `DELETE /v1/transfers/{id}`

All peer routes except the initial offer require the bearer session token. See
[transfer-protocol.md](transfer-protocol.md) for payload and integrity rules.

## Discovery additions

`GET /device/self`, `GET /devices`, discovery WebSocket events, and mDNS TXT
records expose available storage, platform, transfer support, transfer protocol
version, maximum chunk size, and compression support. Available storage may be
`-1` on a future platform without a native storage adapter.

## Errors

Invalid identifiers, manifests, indexes, or paths return `400`; missing or bad
session credentials return `401`; untrusted offers return `403`; missing
transfers return `404`; approval/state/conflict errors return `409`; checksum
failures return `422`; unexpected storage/database failures return `500` without
leaking internal paths to peer clients.
