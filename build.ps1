<#
.SYNOPSIS
    Baut soa-dashboard-jobs.exe fuer Windows amd64.
.DESCRIPTION
    Ersetzt die Paketierung ueber zeit/pkg im Node-Original. Es wird kein
    Node und kein Download eines Node-Binaries benoetigt.
.PARAMETER Output
    Zieldatei. Standard: soa-dashboard-jobs.exe
.PARAMETER Version
    Versionskennung fuer /checkalive. Standard: git describe.
#>
param(
    [string]$Output = "soa-dashboard-jobs.exe",
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

if (-not $Version) {
    $Version = (git describe --tags --always --dirty 2>$null)
    if (-not $Version) { $Version = "dev" }
}

Write-Host "Baue $Output (Version $Version)"

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

go vet ./...
go test ./...
go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $Output .

Write-Host "Fertig: $Output"
