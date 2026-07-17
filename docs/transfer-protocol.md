# File transfer protocol

Protocol version: `1`.

Peer requests use HTTP over TLS 1.3 on the discovered peer address. The client
does not follow redirects and verifies that the certificate's Ed25519 public key
matches the durable paired key. The self-signed certificate is a transport
container; trust comes from pairing-key pinning, not a public CA.

## Authenticated offer

The sender creates a random transfer bearer credential, builds the complete
manifest, and sends `POST /v1/transfers/offers`. It authenticates the exact
method, path, sender device ID, timestamp, nonce, and body SHA-256 with HMAC
using the pairing-derived shared key. The receiver rejects skewed timestamps,
reused nonces, bad MACs, blocked/untrusted devices, identity-key mismatches, and
source IPs that do not match current discovery.

The offer includes sender/receiver IDs and names, transfer UUID, protocol
version, negotiated chunk/compression settings, total size, bearer credential,
and a file/directory manifest. Regular files include a UUID, portable relative
path, size, SHA-256, chunk size, and chunk count.

The receiver persists an approval-required session. Repeating a valid identical
offer is idempotent. A changed offer under the same ID is rejected.

## Approval and session authorization

Until the local user accepts or rejects, the sender polls
`GET /v1/transfers/{id}/status`. This and all post-offer operations include
`Authorization: Bearer <random-transfer-credential>` inside the pinned TLS
channel. The credential is scoped to that transfer and stored only in local
durable state required for restart/resume.

## Resume map and chunks

The status response contains state, approval, and completed chunk indexes by
file UUID. The sender omits existing chunks after reconnect or restart.

`PUT /v1/transfers/{id}/files/{fileId}/chunks/{index}` uses:

- `Authorization: Bearer <credential>`
- `Content-Type: application/octet-stream`
- `X-Chunk-SHA256: <lowercase SHA-256>`
- optional `Content-Encoding: gzip`

The uncompressed size must match the manifest. Output limits bound gzip
expansion. The receiver verifies the chunk before writing/syncing and committing
its completion row. Corruption returns `422`; retryable failures use bounded
backoff.

## Completion and cancellation

`POST /v1/transfers/{id}/complete` verifies that every expected chunk exists,
streams every partial through whole-file SHA-256, and only then commits files to
the user-approved destination. `DELETE /v1/transfers/{id}` propagates sender
cancellation. Failed partial data can remain for retry; terminal metadata remains
in history according to local retention actions.

## Portable paths

Wire paths use `/`, are relative, normalized, and non-empty. Absolute paths,
parent traversal, backslashes, NUL, empty components, trailing dots/spaces,
Windows device names, symlinks, special files, and duplicate case-folded paths
are rejected. The receiver joins beneath the approved root and rechecks
containment before committing.

## State machine

Durable public states are `Queued`, `Preparing`, `Connecting`, `Negotiating`,
`Sending`, `Receiving`, `Paused`, `Resuming`, `Verifying`, `Completed`,
`Cancelled`, and `Failed`. Incoming `Queued` with `approvalRequired: true`
means local user action is required.
