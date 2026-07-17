# Privacy

SyncSpace's implemented transfer path is local-first:

- no account or sign-in;
- no cloud database or object storage;
- no relay or remote-access fallback;
- no analytics, advertising SDK, crash-report upload, or usage telemetry;
- no automatic upload of names, device data, diagnostics, or transferred files.

Devices advertise limited mDNS metadata on the current LAN so peers can be
found: device ID, display name, platform/type, app and protocol capabilities,
peer port, availability, and a short public-key fingerprint hint. Anyone on the
same broadcast network may be able to observe that advertisement. Discovery is
not trust and does not allow transfer access.

Local persistent data includes the device identity, trusted peer public
identities and pairing credentials, settings, transfer queue/history metadata,
browser staging, resumable partial files, and received files. The exact root is
shown in Diagnostics and can be overridden with `SYNCSPACE_DATA_DIR`.

Windows protects private identity material with user-scoped DPAPI. Other
current platforms use a private file restricted to the current user. File
contents are encrypted in transit between paired devices with TLS 1.3 but are
not encrypted by SyncSpace after they are committed to the destination; normal
operating-system disk encryption and access controls apply.

The Diagnostics ZIP is created only when the local user requests it. It omits
private keys, pairing shared keys, SQLite contents, and file bodies, but it can
contain device names, IP addresses, usernames embedded in paths, filenames, and
recent error context. Review the archive before sharing it.

Forgetting a device deletes its local trust record. Blocking preserves its
identity and denies new authorization. Clearing history removes eligible
transfer history records but does not delete files already received at a user
destination. Lab reset affects only `.syncspace-dev`.
