# SQLite schema

The shared database is `syncspace.db`. Transfer migrations are idempotent and
enable WAL plus a five-second busy timeout.

- `transfers` is the canonical session row: UUID, direction, sender/receiver
  device ID, display metadata, path selections, size, aggregate checksum,
  status, byte progress, speed, ETA, timestamps, attempts, priority, approval,
  conflict policy, chunk/compression/protocol negotiation, and session secrets.
- `transfer_files` stores directories and regular-file manifests, including
  source/destination paths, size, SHA-256, chunk size, and chunk count.
- `chunks` stores `(transfer, file, index)` completion, offset, size, SHA-256,
  attempt count, and update time.
- `queued_transfers` is the durable ordered work view.
- `paused_transfers` records paused sessions.
- `failed_transfers` records terminal error text and failure time.
- `transfer_history` records completed/cancelled terminal sessions.

`SaveTransfer` updates the canonical row, file manifest, and exactly one state
mirror in a transaction. Chunk completion is upserted only after the partial
file write has been synced. Timestamps are UTC Unix milliseconds; sizes and
offsets are signed 64-bit integers, so 100 GB and larger files do not overflow.

Session tokens currently live in the local plaintext database, consistent with
the existing placeholder pairing credential. Platform keystore wrapping is a
required security follow-up before hostile-device threat models are supported.
