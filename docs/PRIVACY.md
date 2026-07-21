# Privacy implementation reference

The user-facing and in-app policy is [the canonical SyncSpace privacy
policy](../PRIVACY.md). Its current version is `2026-07-1`, effective 21 July
2026. Policy wording belongs in the backend's presentation-neutral policy model
and the root policy; this page records the technical contract so a second copy
of the policy does not drift.

## First-launch gate

Acceptance is stored in `settings.json` as `privacyPolicyVersion` and
`privacyAcceptedAt`. Acceptance is valid only when the stored version equals
the backend constant. A normal server start never accepts on the user's behalf.

Before acceptance, the backend permits only the local UI and these management
operations:

- `GET /api/v1/health`
- `GET /api/v1/device/self`
- `GET /api/v1/settings`
- `GET /api/v1/privacy-policy`
- `POST /api/v1/privacy-policy/accept`

Other discovery, pairing, diagnostics, WebSocket, staging, and transfer calls
return `403 Forbidden`. The runtime supervisor keeps mDNS browsing/advertising
and transfer workers stopped. The TLS peer listener may already be bound so the
process can start cleanly, but its pairing and transfer routes are gated and do
not accept protocol work.

The `dev start` and `dev verify` helpers are the only exception to the manual
workflow: after each isolated policy endpoint is reachable, the helper accepts
the current policy through the same local API, then requires both servers to be
healthy before automated pairing proceeds.

## Persistent controls

`deviceName`, `discoverable`, and `incomingTransfersEnabled` are ordinary
validated preferences. Changing the display name updates future discovery,
pairing, and transfer projections without changing the stable device ID or
trust keys. Turning discoverability off stops and withdraws mDNS service.
Turning incoming offers off rejects new peer offers before transfer bytes are
accepted.

General settings writes cannot forge, clear, or replace privacy acceptance.
Only the version-checked acceptance endpoint can change it. Resetting ordinary
settings preserves the device name and current policy acceptance.

## Local records

- `identity.json`: stable random device ID, hostname, public identity metadata,
  and fingerprint.
- `identity.key`: private identity material, protected by user-scoped DPAPI on
  Windows and by restrictive file permissions on other current platforms.
- `settings.json`: device name, discovery/incoming choices, receive preferences,
  and privacy acceptance.
- `syncspace.db`: trusted devices and transfer state/history.
- `transfers/`: browser staging, resumable partials, and managed transfer data.

See [Architecture](architecture.md) for runtime boundaries and [REST and
WebSocket API](api.md) for the exact local routes.
