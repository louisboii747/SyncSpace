# SyncSpace privacy policy

**Policy version:** `2026-07-1`

**Effective date:** 21 July 2026

SyncSpace finds and transfers files between devices on your local network. This
policy explains what nearby devices can see, what SyncSpace keeps on this
computer, and the choices available to you.

## Before SyncSpace uses the local network

The current policy must be accepted before discovery, pairing, or file transfer
can be used. SyncSpace stores the accepted version and UTC timestamp locally.
It asks again only when the policy version changes or the local application data
is reset.

This is a backend-enforced gate. Before acceptance, SyncSpace does not advertise
through mDNS, browse for nearby devices, run transfer workers, or allow pairing
or transfer operations. The local interface and the small set of management
endpoints needed to read and accept the policy remain available. Declining the
policy keeps SyncSpace paused.

## Nearby device discovery

When discovery is on, SyncSpace looks for other SyncSpace devices on the same
local network and advertises enough information for them to recognise this
device.

Nearby devices may see:

- the editable SyncSpace device name;
- the operating-system hostname;
- a random SyncSpace device ID;
- device type and platform;
- app and protocol versions;
- the local network address and peer port;
- availability and supported transfer features;
- a short public-key fingerprint hint.

People or software on the same broadcast network may be able to observe this
advertisement. Discovery shows only that a device is nearby. It does not make
that device trusted, and it does not give that device permission to send files.
The advertisement does not contain pairing secrets, transfer tokens, usernames,
file paths, or file contents.

## File transfers

Files move directly between the devices taking part. Core transfers do not
upload a copy to a SyncSpace-operated cloud service.

- Only files and folders you choose are sent.
- New incoming offers are accepted only from paired devices and need your
  approval unless incoming offers have been disabled entirely.
- Executable and script names are called out before approval, but SyncSpace
  does not decide whether the content itself is safe.
- Received files are never opened or run automatically.
- Paired transfers use an authenticated TLS 1.3 connection whose peer identity
  is pinned to the trusted Ed25519 key.
- Files are committed to the chosen destination only after SHA-256 verification.

SyncSpace encrypts file content while it travels between paired devices. It does
not add its own encryption after a received file is committed to the destination;
normal operating-system disk encryption and access controls apply.

## Information kept on this device

SyncSpace stores the information it needs locally so identity, trust,
preferences, and transfer history survive a restart. This includes:

- a randomly generated device ID and protected cryptographic identity;
- the SyncSpace device name and operating-system hostname;
- settings and privacy-policy acceptance;
- trusted-device identities, pairing relationships, and blocked-device state;
- transfer history, including file names, relative paths, sizes, status,
  hashes, timestamps, and errors;
- the selected receive location;
- temporary browser staging and resumable partial files while work is pending;
- received files in the destination you approve.

File contents are not stored in the SQLite application database. Browser
selections are temporarily copied into private local staging because a browser
cannot provide native source paths. Received file content is written to private
partial files and moved to the chosen destination only after verification.

The data root is shown in Diagnostics and can be overridden with
`SYNCSPACE_DATA_DIR`. By default it is `%APPDATA%\SyncSpace` on Windows,
`~/Library/Application Support/SyncSpace` on macOS, and
`${XDG_CONFIG_HOME:-~/.config}/SyncSpace` on Linux.

Windows protects private identity material with user-scoped DPAPI. Other current
platforms store it in a file restricted to the current user. SyncSpace does not
use a MAC address, serial number, or another private hardware identifier as its
device ID.

## Network access and external services

SyncSpace uses local IP addresses and ports to discover devices and establish
direct connections. It does not inspect unrelated browser history, saved
passwords, private messages, contacts, or files you have not selected.

The current application has no account service, cloud database, relay, remote
access fallback, analytics, advertising, crash-report upload, automatic update
check, or usage telemetry. Core discovery and transfer therefore stay on the
local network. Development commands such as `npm ci` can contact their normal
package registries; that is separate from running SyncSpace.

## Diagnostics

The Diagnostics ZIP is created only when the local user requests it. It omits
private keys, derived pairing keys, transfer credentials, SQLite contents, and
file bodies. It can still contain device names, IP addresses, usernames embedded
in paths, filenames, and recent error context. Review the archive before sharing
it.

## Your choices

You can:

- rename this SyncSpace device without changing its stable device ID;
- turn local-network discoverability off;
- stop new incoming transfer offers;
- accept or decline an incoming transfer;
- cancel active transfers;
- block, unblock, or forget trusted devices;
- change the default receive folder and choose a destination when accepting;
- clear completed, cancelled, and failed transfer history without deleting
  received files;
- review this policy and its acceptance timestamp in Settings.

Turning discoverability off stops the local mDNS discovery session and
withdraws its advertisement. Turning incoming offers off returns a clear
rejection before new file data is accepted. These controls persist locally and
are independent of each other.

Forgetting a device removes its local trust record. Blocking keeps its identity
record but denies new authorisation. Clearing history removes eligible metadata
records and does not delete files already saved to a destination. The developer
lab reset command affects only `.syncspace-dev`.

## Current privacy-related limitations

- Browser folder staging cannot represent empty directories. The current
  transfer manifest also does not preserve file modification timestamps or
  permissions.
- The embedded web interface does not yet provide native open-file,
  reveal-in-folder, copy-path, or open-data-folder actions.
- Diagnostic export is designed to omit secrets and file bodies, but it is not
  guaranteed to remove every identifying filename or local path; manual review
  remains necessary.
