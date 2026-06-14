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
        foreach ($process in @(Get-Process $name -ErrorAction SilentlyContinue)) {
            try {
                Stop-Process -Id $process.Id -Force -ErrorAction Stop
            }
            catch {
                Write-Warning "could not stop residual process $name pid=$($process.Id): $($_.Exception.Message)"
            }
        }
    }
    Start-Sleep -Milliseconds 500
}

function Get-TrimmedText {
    param([AllowNull()][string]$Value)
    if ($null -eq $Value) {
        return ""
    }
    return $Value.Trim()
}

function Resolve-WindowsChromeRoot {
    $configured = Get-TrimmedText $env:ANT_BROWSER_WINDOWS_CHROME_ROOT
    if ($configured -ne "") {
        if (-not [System.IO.Path]::IsPathRooted($configured)) {
            $configured = Join-Path $repoRoot $configured
        }
        return $configured
    }

    if ((Get-TrimmedText $env:GITHUB_ACTIONS) -eq "true") {
        return "C:\AntBrowserReleaseResources\chrome"
    }

    return Join-Path $repoRoot "chrome"
}

function Resolve-WindowsChromeRequirement {
    $configured = Get-TrimmedText $env:ANT_BROWSER_REQUIRE_WINDOWS_CHROME
    if ($configured -ne "") {
        return ($configured -eq "1")
    }

    return ((Get-TrimmedText $env:GITHUB_ACTIONS) -eq "true")
}

function Test-WindowsChromeCore {
    param([string]$ChromeRoot)

    $rootExecutable = Join-Path $ChromeRoot "chrome.exe"
    if (Test-Path -LiteralPath $rootExecutable -PathType Leaf) {
        return $true
    }

    if (-not (Test-Path -LiteralPath $ChromeRoot -PathType Container)) {
        return $false
    }

    foreach ($entry in (Get-ChildItem -LiteralPath $ChromeRoot -Force)) {
        if (-not $entry.PSIsContainer) {
            continue
        }
        if (Test-Path -LiteralPath (Join-Path $entry.FullName "chrome.exe") -PathType Leaf) {
            return $true
        }
    }

    return $false
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

Write-Step "Check Windows browser core"
$chromeRoot = Resolve-WindowsChromeRoot
if (Resolve-WindowsChromeRequirement) {
    if (-not (Test-WindowsChromeCore -ChromeRoot $chromeRoot)) {
        throw "required Windows browser core missing: $chromeRoot"
    }
}
else {
    Write-Host "Windows browser core is optional for this run: $chromeRoot"
}

Write-Host ""
Write-Host "[OK] Windows release preflight passed"
