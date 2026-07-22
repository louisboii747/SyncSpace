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
    $env:CGO_ENABLED = '0'
    try {
        Push-Location $repoRoot
        try {
            $serverFlags = "-s -w -X main.buildVersion=$Version"
            go build -trimpath -buildvcs=true -ldflags $serverFlags -o (Join-Path $releaseDirectory 'syncspace.exe') ./backend/cmd/server
            if ($LASTEXITCODE -ne 0) { throw 'syncspace.exe build failed' }
            $cliFlags = "-s -w -X main.buildVersion=$Version"
            go build -trimpath -buildvcs=true -ldflags $cliFlags -o (Join-Path $releaseDirectory 'syncspace-cli.exe') ./backend/cmd/syncspace
            if ($LASTEXITCODE -ne 0) { throw 'syncspace-cli.exe build failed' }
        } finally {
            Pop-Location
        }
    } finally {
        $env:GOOS = $previousGoos
        $env:GOARCH = $previousGoarch
        $env:CGO_ENABLED = $previousCgo
    }

    Copy-Item (Join-Path $repoRoot 'LICENSE') $releaseDirectory
    Copy-Item (Join-Path $repoRoot 'windows\README.md') (Join-Path $releaseDirectory 'README.txt')

    $prefix = "syncspace-$Version-windows-$Architecture"
    Copy-Item (Join-Path $releaseDirectory 'syncspace.exe') (Join-Path $outputRoot "$prefix.exe") -Force
    Copy-Item (Join-Path $releaseDirectory 'syncspace-cli.exe') (Join-Path $outputRoot "$prefix-cli.exe") -Force
    Compress-Archive -Path (Join-Path $releaseDirectory '*') -DestinationPath (Join-Path $outputRoot "$prefix.zip") -CompressionLevel Optimal -Force

    Write-Host "Built Windows $Architecture release assets in $outputRoot"
} finally {
    if (Test-Path -LiteralPath $staging) {
        Remove-Item -LiteralPath $staging -Recurse -Force
    }
}
