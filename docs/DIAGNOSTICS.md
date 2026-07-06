# Diagnostics

The React Diagnostics page is available from the sidebar. It reports the local
device ID/name/platform, backend URL, frontend transfer-WebSocket state,
discovery state, known/trusted devices, active work, queue, storage/database
paths, protocol version, health checks, recent logs, and last errors.

Actions on the page can run a health check, refresh discovery, queue a real
seed transfer, create an interrupted-transfer simulation, clear test data, and
download a diagnostics ZIP. Simulation actions are disabled outside developer
mode.

## Health endpoint

```sh
curl http://127.0.0.1:8384/health
go run ./backend/cmd/syncspace doctor --url http://127.0.0.1:8384
```

`GET /health` checks that SQLite responds, the transfer storage directory can
create/sync/delete a probe file, WebSocket brokers are available, discovery is
running, the transfer service is composed, device identity validates, the trust
store is readable, and the frontend request reached the backend. It returns
HTTP 200 with `status: ok`, or HTTP 503 with `status: degraded`.

## Runtime snapshot

`GET /diagnostics` is loopback-only and returns the complete page snapshot.
Paths and logs can reveal local usernames or filenames, so this endpoint is not
exposed to LAN clients.

`GET /diagnostics/export` downloads a ZIP containing:

- `diagnostics.json`: health and runtime state
- `logs.txt`: the bounded in-memory human-readable log preview

Export from the CLI:

```sh
go run ./backend/cmd/syncspace export-diagnostics --output diagnostics.zip
```

The bundle does not include SQLite contents or transferred file bytes. Review
paths, device names, addresses, and log attributes before sharing it.

## Logging

The server writes structured human-readable records to stdout for discovery
selection/restarts, pairing decisions, WebSocket connect/disconnect, transfer
lifecycle/progress, retries, verification failures, storage failures, and
recovery. The in-memory diagnostics buffer keeps the newest 500 entries; the
page shows 80 recent entries and 20 recent errors.
