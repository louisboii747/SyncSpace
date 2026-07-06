$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
$code = 0
try { & go run ./backend/cmd/syncspace dev device-a @args; $code = $LASTEXITCODE } finally { Pop-Location }
if ($code -ne 0) { throw "Device A exited with code $code" }
