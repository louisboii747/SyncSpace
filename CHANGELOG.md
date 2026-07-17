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

### Changed

- Management/UI and peer-protocol traffic now use separate listeners.
- mDNS advertises the encrypted peer port and identity fingerprint hint.
- The local lab uses independent real identities and pinned encrypted transfer
  instead of placeholder trust.
- Diagnostics product controls focus on real health, discovery, logs, paths,
  and export; failure simulators remain internal test fixtures.
- Documentation now describes exact startup/use commands and distinguishes the
  runnable shared product from unimplemented native platform clients.

### Security

- Private keys and pairing shared credentials are excluded from public JSON.
- Offer timestamps/nonces and pairing proof nonces are checked for replay.
- Cross-origin requests from a different localhost port are rejected.
