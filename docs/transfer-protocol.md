# File transfer protocol

Protocol version: `1`

Transport is HTTP/1.1 or HTTP/2 over the discovered local address. The current
implementation uses HTTP URLs; it never follows redirects. Future authenticated
pairing must upgrade this transport to mutually authenticated encryption before
SyncSpace can claim protection against an active hostile LAN.

## Negotiation

The sender creates a random 256-bit session token and sends `POST
/v1/transfers/offers`. An offer contains the permanent sender ID, display name,
transfer UUID, protocol version, negotiated chunk size, compression flag, total
size, and a complete file/directory manifest. Each regular file has its own UUID,
portable relative path, byte size, SHA-256, chunk size, and chunk count.

The receiver validates all identifiers, totals, checksums, paths, protocol
limits, durable trust, live discovery record, and source IP before persisting an
unapproved `Queued` session. Repeating the same offer is idempotent. The sender
polls `GET /v1/transfers/{id}/status` with `Authorization: Bearer <token>` until
the user accepts or rejects it.

## Resume map

The status response contains transfer state, approval, and completed chunk
indexes grouped by file UUID. The sender omits those chunks. This works after a
process restart because both the session token and chunk state are durable.

## Chunk upload

`PUT /v1/transfers/{id}/files/{fileId}/chunks/{index}` uses:

- `Authorization: Bearer <token>`
- `Content-Type: application/octet-stream`
- `X-Chunk-SHA256: <lowercase SHA-256>`
- optional `Content-Encoding: gzip`

The uncompressed body must have exactly the size implied by the manifest.
Compression is attempted only for compressible media types and used only when
it makes the chunk smaller. The receiver applies decompression and output limits
to prevent oversized payloads. A corrupt chunk returns `422`; the sender retries
failed requests up to three times with backoff.

## Completion and cancellation

`POST /v1/transfers/{id}/complete` starts whole-file verification and commit.
The operation succeeds only after every file hash matches. `DELETE
/v1/transfers/{id}` propagates sender cancellation to the receiver. Partial data
is retained after failure, but cancelled/completed session cleanup is left to
retention policy so history remains inspectable.

## Portable paths

Wire paths always use `/`, are relative, normalized, and non-empty. Absolute
paths, `..`, backslashes, NUL, empty components, trailing dots/spaces, Windows
device names, symbolic links, and duplicate case-folded paths are rejected.
The receiver joins paths beneath a user-approved root and rechecks containment.

## State machine

`Queued`, `Preparing`, `Connecting`, `Negotiating`, `Sending`, `Receiving`,
`Paused`, `Resuming`, `Verifying`, `Completed`, `Cancelled`, and `Failed` are
durable public states. Incoming `Queued` plus `approvalRequired: true` means the
receiver is awaiting a decision.
