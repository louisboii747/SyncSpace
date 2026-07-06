# Transfer development guide

Run focused tests while iterating:

```sh
go test ./backend/internal/transfer
go test ./backend/internal/api ./backend/internal/websocket
cd frontend && npm run check
```

Run acceptance checks before committing:

```sh
gofmt -w backend
go test ./...
go vet ./...
cd frontend && npm run check
```

Tests cover folders, multiple selections, portable path validation, 100 GB
chunk arithmetic, corrupt chunks, out-of-order upload, compression, whole-file
verification, conflict policy, pause/resume/cancel/retry, crash recovery,
SQLite state mirrors, stable 1,200-item queues, API access control, and an
end-to-end HTTP sender/receiver transfer.

When changing protocol fields, update the mDNS capability advertisement, Go
models, protocol document, API document, persistence migration, and native UI
contract together. Never infer authorization from discovery and never mark a
transfer complete before whole-file SHA-256 verification succeeds.

## Frontend workflow

Run `npm install` once in `frontend`, then `npm run dev` alongside the Go server.
Vite proxies local REST and WebSocket requests to `127.0.0.1:8384`. `npm run
build` type-checks the application and writes hashed production assets to
`backend/internal/frontend/dist`; the Go binary embeds that directory. Rebuild
the frontend whenever its source changes before compiling a release binary.
