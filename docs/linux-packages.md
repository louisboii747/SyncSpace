# Install SyncSpace on Linux

SyncSpace is distributed for systemd-based Linux distributions as native DEB
and RPM packages attached to GitHub Releases. Each package contains the Go
server, the embedded React interface, a small desktop launcher, a systemd
**user** service, and a desktop menu entry. Node.js and Go are not needed on a
computer that installs a release package.

The package deliberately does not install a root-owned network daemon. Every
Linux user gets a separate SyncSpace identity and data directory, and the
service is started only when that user launches or explicitly enables it. This
keeps the first-run privacy decision with the person using the application and
avoids creating one machine-wide identity shared by several accounts.

## Choose the correct package

Each stable release contains these four Linux packages:

| Computer | DEB package | RPM package |
| --- | --- | --- |
| x86-64 (`x86_64`) | `syncspace_VERSION_amd64.deb` | `syncspace-VERSION-1.x86_64.rpm` |
| 64-bit ARM (`aarch64`) | `syncspace_VERSION_arm64.deb` | `syncspace-VERSION-1.aarch64.rpm` |

There are no 32-bit packages. On Debian or Ubuntu, check with
`dpkg --print-architecture`. On an RPM-based distribution, check with
`uname -m`.

Prereleases use package-manager ordering in their filenames—for example,
`1.5.0~rc.1` in a DEB and `1.5.0-0.rc.1.1` in an RPM—while SyncSpace itself
still reports `1.5.0-rc.1`. Always use the filenames shown on that release.

Download the package and `SHA256SUMS` from the matching entry on the
[SyncSpace Releases page](https://github.com/louisboii747/syncspace/releases).
Do not install a package renamed to look like a SyncSpace release.

## Verify the download

Place the downloaded package and `SHA256SUMS` in the same directory, then run:

```sh
sha256sum --check --ignore-missing SHA256SUMS
```

The package must report `OK`. The checksum detects a damaged or substituted
file, while the GitHub build attestation separately proves which repository and
workflow produced it. With the GitHub CLI installed, verify that provenance as
well:

```sh
gh attestation verify ./syncspace_VERSION_amd64.deb \
  --repo louisboii747/syncspace
```

Use the actual RPM filename when installing an RPM. SyncSpace currently uses
GitHub's signed artifact attestations rather than publishing detached OpenPGP
`.sig` files. A checksum downloaded from the same untrusted location is not, by
itself, proof of publisher identity; use the attestation when provenance
matters.

## Install and open SyncSpace

Debian, Ubuntu, and related systems:

```sh
sudo apt install ./syncspace_VERSION_amd64.deb
```

Fedora, RHEL, Rocky Linux, and related systems:

```sh
sudo dnf install ./syncspace-VERSION-1.x86_64.rpm
```

Substitute the ARM64 filename where appropriate. Using `apt install` or
`dnf install` is preferable to calling `dpkg` or `rpm` directly because the
package manager also resolves declared dependencies.

Open **SyncSpace** from the application menu, or run:

```sh
syncspace
```

`syncspace` and `syncspace open` start the per-user service, wait for its local
health check, and open the interface at <http://127.0.0.1:8384>. On a session
without a graphical browser, start it without trying to open a page and visit
that address yourself:

```sh
syncspace start
```

Read and accept the first-launch privacy policy before discovery, pairing, or
transfers can start. The package never accepts it during installation.

The management interface stays on loopback. Other devices communicate only
with the encrypted peer listener, which uses TCP port `8385` by default; mDNS
discovery uses the normal link-local UDP port `5353`. If a host firewall blocks
local-network traffic, allow mDNS and inbound TCP `8385` only on the trusted
local network. Never expose management port `8384` to the LAN or internet.

## Start automatically after sign-in

Installing or upgrading the package does not silently start or enable
SyncSpace. To run it whenever this user signs in:

```sh
syncspace enable
```

To stop it and turn that behaviour off:

```sh
syncspace disable
```

This is a user service, so do not add `sudo` to these commands. By default it
runs only while the user has an active login session. SyncSpace does not enable
systemd lingering or create a machine-wide identity.

The launcher wraps the user-service operations and does not use elevated
permissions:

```sh
syncspace start
syncspace stop
syncspace restart
syncspace status
syncspace logs
syncspace --version
syncspace --help
```

The equivalent advanced commands are `systemctl --user ... syncspace.service`
and `journalctl --user -u syncspace.service`. `syncspace logs` follows the user
journal until it is interrupted with `Ctrl+C`.

## Configure ports and storage

The service optionally reads:

```text
${XDG_CONFIG_HOME:-$HOME/.config}/syncspace/service.env
```

Create it from the packaged example:

```sh
config_root="${XDG_CONFIG_HOME:-$HOME/.config}"
mkdir -p "$config_root/syncspace"
cp /usr/share/doc/syncspace/service.env.example \
  "$config_root/syncspace/service.env"
chmod 600 "$config_root/syncspace/service.env"
```

Edit only the settings that are needed, then restart the service. This file
uses systemd environment-file syntax, not shell syntax; do not write `export`,
`~`, `$HOME`, or command substitutions in its values.

| Variable | Purpose | Packaged default |
| --- | --- | --- |
| `SYNCSPACE_HOST` | Local management bind; SyncSpace rejects non-loopback values | `127.0.0.1` |
| `SYNCSPACE_PORT` | Local interface and management API port | `8384` |
| `SYNCSPACE_PEER_HOST` | Encrypted peer listener bind | `0.0.0.0` |
| `SYNCSPACE_PEER_PORT` | Encrypted LAN pairing and transfer port | `8385` |
| `SYNCSPACE_DATA_DIR` | Identity, settings, SQLite, staging, and transfer state | `${XDG_CONFIG_HOME:-$HOME/.config}/SyncSpace` |

For example, a literal custom data path can be written as:

```text
SYNCSPACE_DATA_DIR=/home/alex/.local/state/syncspace
```

Then apply the change:

```sh
syncspace restart
```

Changing `SYNCSPACE_DATA_DIR` without moving the old directory creates a new
device identity. Do not point two computers or two accounts at a shared copy of
that directory. The receive directory is a separate setting in the SyncSpace
interface and defaults to `~/Downloads/SyncSpace`.

Each account has separate state, but TCP ports are shared by the whole
computer. If two logged-in users need SyncSpace running at the same time, give
the second account unique `SYNCSPACE_PORT` and `SYNCSPACE_PEER_PORT` values in
its own `service.env` before starting it. A port conflict is reported in that
user's service logs; SyncSpace never silently reuses another account's process.

## Upgrade safely

Download and verify the newer package, then install it over the existing one:

```sh
# DEB
sudo apt install ./syncspace_NEW_VERSION_amd64.deb

# RPM
sudo dnf upgrade ./syncspace-NEW_VERSION-1.x86_64.rpm
```

The package manager replaces application files but leaves per-user identity,
trusted devices, settings, transfer history, and received files alone. Restart
the user service so the running process uses the new executable. On a
multi-user computer, each user who is currently running SyncSpace must restart
their own service:

```sh
systemctl --user daemon-reload
syncspace restart
```

Check **Settings > Privacy** after an upgrade. SyncSpace asks again only when a
release ships a newer privacy-policy version. Database migrations run when the
new process starts. Downgrading across a database migration is not supported;
restore a compatible backup instead of installing an older package over newer
state.

GitHub Releases is an artifact distribution channel, not an APT or DNF
repository. Normal `apt upgrade` or `dnf upgrade` cannot discover a new
SyncSpace version automatically; download each desired release and verify it
before installing.

## Uninstall

First stop and disable the service for the current user:

```sh
syncspace disable
```

On a multi-user computer, repeat that as every account that enabled SyncSpace.
A system package cannot safely enter arbitrary user sessions or modify their
home directories, so removal does not forcibly stop another logged-in user's
already-running process. It exits when that user stops it or signs out.

Then remove the package:

```sh
# DEB
sudo apt remove syncspace

# RPM
sudo dnf remove syncspace
```

Uninstallation removes the executable, launcher, desktop entry, icon, service
unit, and packaged documentation. It intentionally preserves the user's data
under `${XDG_CONFIG_HOME:-$HOME/.config}/SyncSpace`, the receive folder, and any
custom `service.env`. This makes reinstalling or changing package format safe.

If the package is removed before `syncspace disable` is run, systemd can leave
a harmless per-user enable symlink behind because a system package must not
modify users' home directories. It points to the now-missing unit and becomes
active again if SyncSpace is reinstalled. Run the disable command before
removal when that is not desired.

Delete that user data only when permanently removing the device identity,
pairing trust, settings, partial transfers, and history is intended. Received
files under `~/Downloads/SyncSpace` are independent and are never removed by
the package.

## Installed files

| Path | Purpose |
| --- | --- |
| `/usr/libexec/syncspace/syncspace-server` | Versioned Go server with the embedded interface |
| `/usr/bin/syncspace` | User launcher |
| `/usr/lib/systemd/user/syncspace.service` | Per-user systemd unit |
| `/usr/share/applications/syncspace.desktop` | Application-menu entry |
| `/usr/share/icons/hicolor/scalable/apps/syncspace.svg` | Desktop icon |
| `/usr/share/metainfo/syncspace.metainfo.xml` | AppStream metadata |
| `/usr/share/doc/syncspace/service.env.example` | Optional configuration example |
| `/usr/share/man/man1/syncspace.1.gz` | `man syncspace` reference |
| `/usr/share/licenses/syncspace/LICENSE` | Apache 2.0 licence |

## Windows status

These releases intentionally contain Linux packages only. Windows can still be
run from source, but there is no supported Windows installer or updater yet.
The release workflow has a documented extension point for a future signed
Windows artifact; a Linux package must never be renamed or presented as a
Windows build.
