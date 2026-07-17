# Diagnostics

The sidebar Diagnostics page reads live state from the local process: device
identity, version/protocol, management URL, WebSocket state, discovery and trust
counts, active queue, storage/database paths, component health, bounded recent
logs, and recent errors.

The page can run a health check, request an mDNS refresh, and download a ZIP.
Developer failure simulators remain backend test fixtures and are not presented
as product functionality.

## Health

```powershell
go run ./backend/cmd/syncspace doctor --url http://127.0.0.1:8384
```

Equivalent endpoint: `GET /api/v1/health`. It checks SQLite, writable transfer
storage, WebSocket brokers, discovery, transfer composition, identity validity,
the trust store, and the embedded frontend. A degraded result uses HTTP 503.

## Snapshot and export

`GET /api/v1/diagnostics` returns the local runtime snapshot. It is not
available through the LAN peer listener.

Export from the UI or CLI:

```powershell
go run ./backend/cmd/syncspace export-diagnostics --output diagnostics.zip
```

The ZIP contains `diagnostics.json` and a bounded `logs.txt`. It excludes
private keys, shared pairing keys, database contents, staged/received file
bodies, and partial chunks. Paths, device names, IP addresses, and error text
can still be personal; inspect the bundle before sharing it.

The server writes structured human-readable lifecycle records to stdout. The
in-memory buffer retains only the newest bounded set used by the Diagnostics
page and export.
