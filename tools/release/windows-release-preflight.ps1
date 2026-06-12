param(
    [string]$BusinessServer = "http://192.168.210.169:4174",
    [string]$StaticServer = "http://192.168.210.169:18080"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Set-Location $repoRoot

function Write-Step {
    param([string]$Text)
    Write-Host ""
    Write-Host "== $Text =="
}

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "required command not found: $Name"
    }
}

function Invoke-Native {
    param(
        [string]$FilePath,
        [string[]]$Arguments = @()
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath $($Arguments -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Test-HttpOk {
    param([string]$Url)

    $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 10
    if ($response.StatusCode -lt 200 -or $response.StatusCode -ge 300) {
        throw "HTTP check failed: $Url -> $($response.StatusCode)"
    }
}

function Stop-ResidualProcesses {
    foreach ($name in @("ant-chrome", "xray", "sing-box")) {
        Get-Process $name -ErrorAction SilentlyContinue | Stop-Process -Force
    }
    Start-Sleep -Milliseconds 500
}

Write-Step "Check required commands"
foreach ($command in @("git", "go", "node", "npm", "python", "powershell")) {
    Require-Command $command
}
Require-Command "wails"
Require-Command "makensis"

Write-Step "Print tool versions"
Invoke-Native -FilePath "git" -Arguments @("--version")
Invoke-Native -FilePath "go" -Arguments @("version")
Invoke-Native -FilePath "node" -Arguments @("--version")
Invoke-Native -FilePath "npm" -Arguments @("--version")
Invoke-Native -FilePath "python" -Arguments @("--version")
Invoke-Native -FilePath "wails" -Arguments @("version")

Write-Step "Check repository cleanliness"
$status = (& git status --short)
if ($LASTEXITCODE -ne 0) {
    throw "git status failed"
}
$trackedDirty = @($status | Where-Object { $_ -notmatch '^\?\? node_modules/' })
if ($trackedDirty.Count -gt 0) {
    throw "working tree is not clean:`n$($trackedDirty -join [Environment]::NewLine)"
}

Write-Step "Stop residual runtime processes"
Stop-ResidualProcesses

Write-Step "Check service connectivity"
Test-HttpOk "$BusinessServer/api/health"
Test-HttpOk "$BusinessServer/api/client/health"
Test-HttpOk "$StaticServer/healthz"

Write-Step "Check required source files"
foreach ($path in @(
    "bat\publish.bat",
    "bat\publish.ps1",
    "tools\app-update\windows-app-update-e2e.ps1",
    "tools\app-update\verify-app-update-package.py",
    "publish\runtime-manifest.json",
    "publish\runtime-sources.json",
    "bin\xray.exe",
    "bin\sing-box.exe",
    "wails.json",
    "go.mod",
    "frontend\package-lock.json"
)) {
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot $path))) {
        throw "required file missing: $path"
    }
}

Write-Host ""
Write-Host "[OK] Windows release preflight passed"
