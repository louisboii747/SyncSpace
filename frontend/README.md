# SyncSpace frontend

The frontend is a React, TypeScript, and Vite application served from the local
Go backend. It consumes the loopback REST APIs and transfer WebSocket; it never
connects to a cloud service.

## Develop

```sh
npm install
npm run dev
```

Run the backend at `127.0.0.1:8384`. Vite proxies REST and WebSocket traffic to
that address.

## Verify and embed

```sh
npm run check
```

The command runs unit tests, TypeScript validation, and the production build.
Vite writes hashed assets to `backend/internal/frontend/dist`, which is embedded
in the Go server. Browser drag-and-drop streams files through the loopback-only
staging API because web security does not expose absolute filesystem paths.
