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

# $ErrorActionPreference greift nur bei PowerShell-eigenen (terminierenden)
# Fehlern, nicht bei einem von Null verschiedenen Exit-Code eines nativen
# Kommandos wie go.exe. Invoke-Step prueft daher nach jedem Schritt explizit
# $LASTEXITCODE und bricht andernfalls ab - sonst wuerde ein fehlgeschlagener
# vet/test/build-Schritt stillschweigend uebergangen und trotzdem eine .exe
# erzeugt bzw. "Fertig" ausgegeben.
function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Action
    )
    & $Action
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Abgebrochen: $Name fehlgeschlagen (Exit-Code $LASTEXITCODE)"
        exit $LASTEXITCODE
    }
}

if (-not $Version) {
    $Version = (git describe --tags --always --dirty 2>$null)
    if (-not $Version) { $Version = "dev" }
}

Write-Host "Baue $Output (Version $Version)"

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

Invoke-Step "go vet" { go vet ./... }
Invoke-Step "go test" { go test ./... }
Invoke-Step "go build" { go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $Output . }

Write-Host "Fertig: $Output"
