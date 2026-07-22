# Install SyncSpace on Linux

GitHub Releases provide native DEB and RPM packages for x86-64 and Arm64. Each
package contains the Wails desktop executable, its private Go backend, the
embedded React assets, desktop metadata, icon, and licence. It does not install
a system service, launch a browser, or require Node.js or Go at runtime.

## Requirements

The desktop shell uses GTK 3 and WebKitGTK 4.1. Debian/Ubuntu packages declare
`libwebkit2gtk-4.1-0`, which pulls the distribution's matching GTK runtime
(including Ubuntu's `t64` transition package); RPM packages declare `gtk3` and
`webkit2gtk4.1`. The package manager installs these automatically when they are
available in the configured distribution repositories.

## Verify and install

Download the package for the machine plus `SHA256SUMS` from the same release.

```sh
sha256sum --check --ignore-missing SHA256SUMS

# Debian or Ubuntu
sudo apt install ./syncspace_VERSION_amd64.deb

# Fedora or a compatible RPM distribution
sudo dnf install ./syncspace-VERSION-1.x86_64.rpm
```

GitHub provenance can also be checked with:

```sh
gh attestation verify ./syncspace_VERSION_amd64.deb \
  --repo louisboii747/syncspace
```

## Launch and runtime data

Open **SyncSpace** from the desktop application menu or run:

```sh
syncspace
```

Only one window and backend run for the user. A second launch raises the
existing window. Closing the window stops its companion backend. The local
management listener remains loopback-only; the separate encrypted peer port is
the only service exposed to the LAN.

State is stored below `${XDG_CONFIG_HOME:-$HOME/.config}/SyncSpace`, backend
logs below `${XDG_CACHE_HOME:-$HOME/.cache}/SyncSpace/logs`, and received files
in `$HOME/Downloads/SyncSpace` by default. Package upgrades and removal do not
delete those user-owned files.

## Upgrade or remove

Install a newer downloaded package with the same `apt install ./...` or
`dnf install ./...` command after closing SyncSpace. Remove only the installed
application with:

```sh
sudo apt remove syncspace
# or
sudo dnf remove syncspace
```

Delete the configuration or received files separately only when intentionally
resetting the identity and transfer history.

## Package contents

| Path | Purpose |
| --- | --- |
| `/usr/bin/syncspace` | Native Wails desktop application |
| `/usr/libexec/syncspace/syncspace-server` | Lifecycle-owned Go backend |
| `/usr/share/applications/syncspace.desktop` | Desktop menu entry |
| `/usr/share/icons/hicolor/scalable/apps/syncspace.svg` | Application icon |
| `/usr/share/metainfo/syncspace.metainfo.xml` | AppStream metadata |
