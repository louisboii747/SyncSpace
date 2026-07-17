# SyncSpace frontend

The frontend is the React 19, TypeScript, and Vite management experience. It
uses only the local Go management API; peer file traffic is handled by Go.

For hot reload, first run `go run ./backend/cmd/server` at the repository root.
Then in this directory:

```powershell
npm ci
npm run dev
```

Open <http://127.0.0.1:5173>. Vite proxies `/api` HTTP and WebSocket traffic to
`127.0.0.1:8384`.

Validate and regenerate the build embedded by Go:

```powershell
npm run check
```

That runs Vitest, TypeScript validation, and `vite build`. Output is written to
`../backend/internal/frontend/dist` and compiled into the Go server. Running
only `npm run dev` does not update embedded assets.

Browser security does not reveal absolute source paths. File/folder selections
therefore stream into a private loopback staging session and are then promoted
into the normal durable transfer queue. The browser never carries peer protocol
credentials or connects directly to another device.
