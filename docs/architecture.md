# Architecture

SyncSpace is a local-first system. Every device runs the Go engine locally;
there is no cloud, account server, relay, or internet fallback in the transfer
path.

## Boundaries

`backend/internal/discovery` owns untrusted LAN presence and capability
advertising. `backend/internal/pairing` owns explicit durable trust decisions.
`backend/internal/transfer` owns transfer sessions, manifests, streaming,
resume state, queue order, integrity, history, and receiver disk commits.
`backend/internal/api` and `backend/internal/websocket` are delivery adapters.
`frontend` contains the React/TypeScript control surface, and
`backend/internal/frontend` embeds its production build.
`backend/cmd/server` is the dependency-injection composition root.

Discovery never grants trust. A transfer offer is accepted only when its device
ID is already trusted, its source IP matches the current discovery record, and
the local user explicitly approves the incoming transfer. Local management APIs
are loopback-only. Peer protocol APIs are LAN-accessible and session-token
protected after the offer.

## Transfer lifecycle

Outbound work is durably queued before background workers begin. Preparation
walks each selection, rejects symbolic links and non-regular special files,
builds a portable relative-path manifest, and streams every file through
SHA-256 without loading it into memory. The sender then negotiates a receiver
offer, waits for approval, asks which chunks already exist, and sends missing
chunks through a bounded worker pool.

The receiver writes chunks by offset into private `.part` files. Each chunk is
hashed before it is written and synced before its completion record commits.
Finalization checks that every expected chunk exists, streams every partial file
through SHA-256, and only then moves or copies it into the approved destination.
Verification failures retain partial files and chunk records for retry.

Crash recovery requeues interrupted outbound work and returns an interrupted
receiver verification to `Receiving`. Completed chunk rows make restarts and
network reconnects idempotent.

## Resource model

- File bodies are streamed with bounded 1 MiB hashing/copy buffers.
- A chunk is bounded by the negotiated maximum (16 MiB; 4 MiB default).
- Four chunk workers are used per active transfer by default.
- Two outbound transfers run concurrently by default.
- Retry backoff automatically introduces and then relaxes an adaptive send
  delay when the receiver or network reports congestion.
- SQLite uses WAL and durable state mirrors for queue, paused, failed, and
  history views.

The embedded React application is a production management client, not a peer
transfer transport. Browser security intentionally hides absolute local paths,
so dropped files stream into an isolated loopback staging session before its
top-level roots enter the ordinary durable queue. Native clients can submit
filesystem paths directly and avoid that local copy. Shared UI behavior and
event semantics are specified in [transfer-ui.md](transfer-ui.md).
