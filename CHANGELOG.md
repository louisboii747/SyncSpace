# Changelog

## Unreleased - Next Generation foundation

### Added

- Persistent Ed25519 device identities with fingerprints and Windows DPAPI key
  protection.
- Signed X25519/HKDF pairing with matching six-digit verification, proof of key
  possession, dual confirmation, expiry, cooldown, and replay resistance.
- Durable cryptographic trust records with block, unblock, forget, and identity
  change handling.
- TLS 1.3 peer listener, paired-key certificate pinning, authenticated transfer
  offers, and replay-protected session establishment.
- Central ordered SQLite migrations shared by pairing and transfer stores.
- Versioned `/api/v1` management surface with loopback and exact-origin
  enforcement.
- Atomic persisted settings for appearance, reduced motion, receive directory,
  conflict handling, and notifications.
- Home, verified pairing, identity-aware Devices, and persistent Settings
  experiences in the embedded React application.
- A cryptographically real two-device development and CI verification flow.
- A versioned first-run privacy policy whose acceptance controls discovery,
  pairing, transfer workers, and peer protocol access in the backend.
- Separate operating-system hostname and editable SyncSpace display-name
  metadata, with live mDNS refresh and stable trust identities.
- A deliberate send-review step with add/remove/clear controls, folder
  summaries, and executable-file warnings before browser staging begins.

### Changed

- Management/UI and peer-protocol traffic now use separate listeners.
- mDNS advertises the encrypted peer port and identity fingerprint hint.
- The local lab uses independent real identities and pinned encrypted transfer
  instead of placeholder trust.
- Diagnostics product controls focus on real health, discovery, logs, paths,
  and export; failure simulators remain internal test fixtures.
- Documentation now describes exact startup/use commands and distinguishes the
  runnable shared product from unimplemented native platform clients.
- The embedded interface now uses task-focused, plain-language screens and a
  restrained desktop-utility visual system instead of marketing-style panels.
- Failed transfers are part of history and are removed by Clear history along
  with completed and cancelled records; received files remain untouched.
- New installs save incoming files under `Downloads/SyncSpace`, and system
  notifications that can contain filenames are opt-in.

### Security

- Private keys and pairing shared credentials are excluded from public JSON.
- Offer timestamps/nonces and pairing proof nonces are checked for replay.
- Cross-origin requests from a different localhost port are rejected.
- Before the current privacy policy is accepted, local transfer/pairing
  mutations and every peer-facing route return a clear denial, while the local
  policy, settings, identity, health, and frontend remain available.
