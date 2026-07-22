# SyncSpace for Windows

GitHub Releases provide one ready-to-run `.exe` for Windows x64 (`amd64`) and
one for Windows on Arm (`arm64`). Download the matching executable and run it.

`syncspace.exe` is the complete application: the native Wails window,
production React assets, and Go transfer backend are linked into the same file.
It shuts down the in-process runtime with the window and focuses the existing window when launched twice. It
stores mutable state in `%APPDATA%\SyncSpace`, protects the device identity key
with user-scoped Windows DPAPI, binds the management UI to loopback, and exposes
only the encrypted peer-transfer listener to the LAN. Windows may ask for
firewall permission for local-network discovery and incoming transfers.

The application is currently unsigned. Windows SmartScreen may therefore show
an unrecognised-app warning. Its GitHub build provenance can be checked with:

```powershell
gh attestation verify .\syncspace-VERSION-windows-amd64.exe --repo louisboii747/syncspace
```

The downloaded `.exe` is the whole app; there is no companion backend or CLI to
place beside it. Run it with `--version` to confirm the embedded release version.

## Build locally

From a Windows PowerShell terminal with Go, Node.js, and npm installed:

```powershell
.\packaging\windows\build-windows.ps1 -Version 1.4.0 -Architecture amd64
```

The artifacts are written to `dist\windows`. Use `-SkipFrontend` only after a
successful `npm run build` has refreshed the frontend embedded by Go.
