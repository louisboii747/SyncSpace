# Security

## Report a vulnerability

Do not open a public issue for a suspected vulnerability that exposes keys,
allows unauthorized file access, bypasses pairing, or compromises another
device. Contact the project maintainer privately with the affected revision,
reproduction steps, impact, and any logs with personal paths removed.

## Trust boundaries

- mDNS discovery is attacker-controlled presence data. It never creates trust.
- The local management UI/API is available only through a loopback listener.
- The LAN listener exposes only peer pairing and transfer protocols.
- A paired identity is authorized by its pinned Ed25519 public key, derived
  shared credential, and local block state—not by device name, UUID, or IP
  alone.
- Incoming transfer bytes remain partial and untrusted until chunk and final
  SHA-256 verification succeeds.

## Implemented controls

- Stable per-installation Ed25519 identity and fingerprint.
- User-scoped Windows DPAPI protection for the private key; restrictive
  mode-`0600` storage on other current platforms.
- Signed ephemeral X25519 pairing, HKDF-SHA-256 shared-key derivation, a
  six-digit verification code, and confirmation on both devices.
- Pairing rate limits, expiry, timestamps, HMAC proofs, and nonce replay cache.
- TLS 1.3 peer transport with Ed25519 certificate-key pinning.
- HMAC-authenticated initial transfer offers and one-use offer nonces.
- Random per-transfer bearer credentials inside the pinned TLS channel.
- Discovery-source IP matching, durable block/forget state, and key-change
  warnings.
- Loopback validation, exact-origin host/port checks, and cross-site mutation
  rejection for local browser APIs.
- Bounded request bodies, identifier/path validation, safe portable manifests,
  symlink/special-file rejection, private partials, and atomic persistence.
- Diagnostics bundles exclude database contents, private keys, pairing shared
  keys, and transferred file bodies.

## Known limitations

- A malicious process already running as the same operating-system user can
  attempt to access the loopback management service. There is not yet a
  per-install local API token, IPC transport, or native-shell capability token.
- macOS Keychain, Linux Secret Service, Android Keystore, and Apple Keychain
  adapters are not implemented. Non-Windows private-key security currently
  depends on user-directory and file permissions.
- Trust revocation is local. Forgetting or blocking a device does not send a
  guaranteed revocation notification to a peer that is offline.
- The pairing UI supports code and fingerprint comparison, but not QR or a
  separate out-of-band channel.
- This repository does not yet ship signed installers, sandbox entitlements,
  hardened native clients, automatic updates, or mobile background services.
- Availability attacks on the local network, including mDNS flooding and
  connection exhaustion below application limits, are not completely solved.

Use SyncSpace on networks and operating-system accounts you control. Keep the
host firewall enabled and expose only the peer TCP port required for local
transfer. Never expose the management listener through a reverse proxy.

## Review checklist for protocol changes

Any change to identity, pairing, discovery identity fields, TLS, offer
authentication, transfer manifests, storage paths, or migrations requires tests
for malformed input, replay/expiry, authorization failure, persistence across
restart, and secret redaction. Run the sequence in [docs/TESTING.md](docs/TESTING.md).
