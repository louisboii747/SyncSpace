# Local simulation

## Two real local backend instances

Run both devices with one command:

```sh
go run ./backend/cmd/syncspace dev start
```

Run `cd frontend && npm ci && npm run build && cd ..` once after cloning so the
embedded UI reflects the current React source.

Device A uses port `8384`, ID `00000000-0000-4000-8000-00000000000a`, and
`.syncspace-dev/device-a`. Device B uses port `8385`, ID ending in `b`, and
`.syncspace-dev/device-b`. Static loopback peer records make both appear in
discovery even when the operating system does not loop mDNS back to itself.
Pairing remains explicit and is persisted independently in each SQLite file.

Custom ports are supported:

```sh
go run ./backend/cmd/syncspace dev start --port-a 9001 --port-b 9002
```

Available wrappers:

| Action | PowerShell | macOS/Linux |
| --- | --- | --- |
| Start both | `.\scripts\dev-start.ps1` | `sh scripts/dev-start` |
| Start A | `.\scripts\dev-device-a.ps1` | `sh scripts/dev-device-a` |
| Start B | `.\scripts\dev-device-b.ps1` | `sh scripts/dev-device-b` |
| Seed files | `.\scripts\dev-seed-files.ps1` | `sh scripts/dev-seed-files` |
| Verify transfer | `.\scripts\dev-transfer-test.ps1` | `sh scripts/dev-transfer-test` |
| Reset | `.\scripts\dev-reset.ps1` | `sh scripts/dev-reset` |

## Fake device simulator

Start Device A or the two-device lab, then create a peer:

```sh
go run ./backend/cmd/syncspace dev simulate-device --scenario flaky --trusted
```

Use `--url http://127.0.0.1:8385` to target Device B, `--name "Slow phone"`
to set a label, and `--trusted` to perform the normal local pairing flow.

Scenarios:

| Scenario | Behavior |
| --- | --- |
| `online` | Healthy transfer-protocol peer |
| `offline` | Listed offline and not transfer-capable |
| `trusted` | Created and paired through the real pairing service |
| `untrusted` | Discoverable but never implicitly trusted |
| `slow` | Adds deterministic latency to every protocol request |
| `flaky` | Fails each chunk's first attempt, then accepts retry |
| `rejected` | Rejects the initial transfer offer |
| `interrupted` | Returns a retryable failure for chunks |
| `disk_full` | Returns HTTP 507 while receiving chunks |
| `checksum_failure` | Accepts bytes, then rejects final verification |

The simulator implements the actual HTTP transfer protocol using local
in-process servers. It is available only when `SYNCSPACE_DEV_MODE=true`; all
management routes are loopback-only. It does not silently add trust.

## Direct simulator API

```sh
curl -X POST http://127.0.0.1:8384/dev/simulated-devices \
  -H 'Content-Type: application/json' \
  -d '{"name":"Flaky phone","scenario":"flaky","trusted":true}'
curl http://127.0.0.1:8384/dev/simulated-devices
```

Delete one with `DELETE /dev/simulated-devices/{deviceId}` or use **Clear test
data** on the Diagnostics page.

Discovery (`GET /devices`, `POST /discovery/refresh`, `/ws/discovery`) and
pairing APIs work normally in the lab. Clipboard sync is still a roadmap item;
there is no clipboard API to simulate or test in the current protocol.
