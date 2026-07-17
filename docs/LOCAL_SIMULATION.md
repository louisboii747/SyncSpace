# Two-device local lab

The lab is the fastest way to use the complete implemented flow without two
physical computers. It runs two real SyncSpace processes, not two UI mocks.

Build the current embedded frontend first:

```powershell
cd frontend
npm ci
npm run build
cd ..
```

## Interactive lab

```powershell
go run ./backend/cmd/syncspace dev start
```

Open:

- Device A: <http://127.0.0.1:8384>
- Device B: <http://127.0.0.1:8385>

The command creates independent cryptographic identities and data roots, starts
peer TLS on `18384`/`18385`, compares the derived pairing code on both services,
confirms both sides, and persists mutual trust. It then waits until `Ctrl+C`.

From another terminal, exercise a real transfer:

```powershell
go run ./backend/cmd/syncspace test-transfer
```

The destination is `.syncspace-dev/device-b/received/tiny.txt`. The command
requires mutual trust, stages bytes through Device A, accepts them on Device B,
and checks SHA-256 and persistent history.

## One-command acceptance run

```powershell
go run ./backend/cmd/syncspace dev verify
```

This performs the build, two-process startup, health checks, embedded-frontend
checks, signed pairing, pinned-TLS transfer, destination checksum, and history
checks, then stops both processes. It is also the CI smoke test.

## Wrapper scripts

| Action | PowerShell | macOS/Linux |
| --- | --- | --- |
| Start both | `.\scripts\dev-start.ps1` | `sh scripts/dev-start` |
| Start A alone | `.\scripts\dev-device-a.ps1` | `sh scripts/dev-device-a` |
| Start B alone | `.\scripts\dev-device-b.ps1` | `sh scripts/dev-device-b` |
| Seed files | `.\scripts\dev-seed-files.ps1` | `sh scripts/dev-seed-files` |
| Verify transfer against a running lab | `.\scripts\dev-transfer-test.ps1` | `sh scripts/dev-transfer-test` |
| Reset lab data | `.\scripts\dev-reset.ps1` | `sh scripts/dev-reset` |

Start A and B wrappers in separate terminals when process-level logs need to be
inspected independently. Custom interactive ports are also supported:

```powershell
go run ./backend/cmd/syncspace dev start --port-a 9000 --port-b 9002
```

The corresponding peer TLS ports are the selected management ports plus
`10000`.

## Reset safely

Stop every lab process, then run:

```powershell
go run ./backend/cmd/syncspace dev reset
```

The reset command resolves the repository-local root, requires its final path
component to be `.syncspace-dev`, and removes only that lab directory. It does
not touch the normal per-user SyncSpace data directory.

Developer fixture endpoints remain internal test infrastructure. They are not
the acceptance path and are intentionally not exposed as product controls in
the Diagnostics UI.
