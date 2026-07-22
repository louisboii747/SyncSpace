# SyncSpace for Windows

GitHub Releases provide portable builds for Windows x64 (`amd64`) and Windows
on Arm (`arm64`). Download the ZIP matching the computer, extract it, and run
`syncspace.exe`. The same executable is also published separately for users
who do not need the CLI or licence files.

`syncspace.exe` is the native Wails window and contains the production React
assets. It owns the adjacent `syncspace-server.exe` companion process, shuts it
down with the window, and focuses the existing window when launched twice. It
stores mutable state in `%APPDATA%\SyncSpace`, protects the device identity key
with user-scoped Windows DPAPI, binds the management UI to loopback, and exposes
only the encrypted peer-transfer listener to the LAN. Windows may ask for
firewall permission for local-network discovery and incoming transfers.

The portable build is currently unsigned. Windows SmartScreen may therefore
show an unrecognised-app warning. Verify the downloaded file against
`SHA256SUMS-windows` and, when using GitHub CLI, its build provenance:

```powershell
Get-FileHash .\syncspace-VERSION-windows-amd64.zip -Algorithm SHA256
gh attestation verify .\syncspace-VERSION-windows-amd64.zip --repo louisboii747/syncspace
```

Keep `syncspace.exe` and `syncspace-server.exe` together when using standalone
downloads; the portable ZIP already has the correct layout. Run
`syncspace.exe --version` or `syncspace-cli.exe --version` to confirm the
embedded release version. The CLI contains diagnostics and local development
commands; most users only need `syncspace.exe`.

## Build locally

From a Windows PowerShell terminal with Go, Node.js, and npm installed:

```powershell
.\packaging\windows\build-windows.ps1 -Version 1.4.0 -Architecture amd64
```

The artifacts are written to `dist\windows`. Use `-SkipFrontend` only after a
successful `npm run build` has refreshed the frontend embedded by Go.
