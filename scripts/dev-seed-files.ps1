$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
$code = 0
try { & go run ./backend/cmd/syncspace dev seed @args; $code = $LASTEXITCODE } finally { Pop-Location }
if ($code -ne 0) { throw "Seed command exited with code $code" }
