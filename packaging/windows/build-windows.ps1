[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$')]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [ValidateSet('amd64', 'arm64')]
    [string]$Architecture,

    [string]$OutputDirectory = 'dist\windows',
    [switch]$SkipFrontend
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$outputRoot = if ([IO.Path]::IsPathRooted($OutputDirectory)) {
    $OutputDirectory
} else {
    Join-Path $repoRoot $OutputDirectory
}
$outputRoot = [IO.Path]::GetFullPath($outputRoot)
$staging = Join-Path $env:TEMP ("syncspace-windows-{0}-{1}" -f $Architecture, [guid]::NewGuid())

try {
    if (-not $SkipFrontend) {
        Push-Location (Join-Path $repoRoot 'frontend')
        try {
            npm ci --ignore-scripts --no-audit --no-fund
            if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }
            npm run build
            if ($LASTEXITCODE -ne 0) { throw 'frontend build failed' }
        } finally {
            Pop-Location
        }
    }

    New-Item -ItemType Directory -Force -Path $outputRoot, $staging | Out-Null
    $releaseDirectory = Join-Path $staging 'SyncSpace'
    New-Item -ItemType Directory -Force -Path $releaseDirectory | Out-Null

    $previousGoos = $env:GOOS
    $previousGoarch = $env:GOARCH
    $previousCgo = $env:CGO_ENABLED
    $env:GOOS = 'windows'
    $env:GOARCH = $Architecture
    # Wails needs its native desktop implementation. CGO-disabled builds can
    # compile but exit without creating a window, so release builds are native.
    $env:CGO_ENABLED = '1'
    try {
        Push-Location $repoRoot
        try {
            $desktopFlags = "-H windowsgui -s -w -X main.buildVersion=$Version"
            go build -tags 'desktop,production' -trimpath -buildvcs=true -ldflags $desktopFlags -o (Join-Path $releaseDirectory 'syncspace.exe') ./backend/cmd/desktop
            if ($LASTEXITCODE -ne 0) { throw 'SyncSpace desktop build failed' }
        } finally {
            Pop-Location
        }
    } finally {
        $env:GOOS = $previousGoos
        $env:GOARCH = $previousGoarch
        $env:CGO_ENABLED = $previousCgo
    }

    $prefix = "syncspace-$Version-windows-$Architecture"
    Copy-Item (Join-Path $releaseDirectory 'syncspace.exe') (Join-Path $outputRoot "$prefix.exe") -Force

    Write-Host "Built Windows $Architecture release assets in $outputRoot"
} finally {
    if (Test-Path -LiteralPath $staging) {
        Remove-Item -LiteralPath $staging -Recurse -Force
    }
}
