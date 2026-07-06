$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
$code = 0
try { & go run ./backend/cmd/syncspace dev device-b @args; $code = $LASTEXITCODE } finally { Pop-Location }
if ($code -ne 0) { throw "Device B exited with code $code" }
