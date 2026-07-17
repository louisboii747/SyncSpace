# SQLite schema and migrations

The shared database is `syncspace.db`. A central ordered migrator runs before
pairing or transfer stores use it. `schema_migrations` records applied versions,
and each migration is transactional and idempotent for supported legacy data.

Current migration groups:

1. legacy trusted-device storage;
2. durable transfers, files, chunks, queue/pause/failure/history mirrors, and
   indexes;
3. cryptographic trust columns for public keys, fingerprints, shared pairing
   credentials, blocking, and identity-change state.

Trust credentials and transfer bearer values are never returned by public JSON.
They are local authorization secrets in SQLite; protect the data directory with
normal user permissions and disk encryption.

`transfers` is the canonical session row. `transfer_files` stores portable
directory/file manifests. `chunks` stores offset, size, SHA-256, attempts, and
completion for each `(transfer, file, index)`. `queued_transfers`,
`paused_transfers`, `failed_transfers`, and `transfer_history` mirror durable
state for efficient recovery and UI queries.

`SaveTransfer` updates the canonical row, manifest, and exactly one state mirror
inside a transaction. A chunk is upserted only after the partial-file write is
synced. Timestamps are UTC Unix milliseconds; file sizes and offsets use signed
64-bit integers.

SQLite uses WAL and a busy timeout. Store constructors invoke the same migrator,
so upgrading an old checkout does not depend on initialization order.
