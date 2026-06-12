param(
    [string]$BaselineVersion = "1.1.0",
    [string]$TargetVersion = "1.1.5",
    [string]$TestRoot = "C:\AntBrowserUpdateTest",
    [int]$RunnerWaitSeconds = 25,
    [switch]$SkipPublish
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$outputDir = Join-Path $repoRoot "publish\output"
$baselineDir = Join-Path $TestRoot "baseline"
$targetDir = Join-Path $TestRoot "target"
$installRoot = Join-Path $env:LOCALAPPDATA "Programs\Ant Browser"
$stateRoot = Join-Path $env:LOCALAPPDATA "Ant Browser"
$statePath = Join-Path $stateRoot "app-update\state.json"
$manifestPath = Join-Path $targetDir "app-update-stable.json"
$targetZip = Join-Path $targetDir "AntBrowser-$TargetVersion-windows-amd64.zip"
$baselineInstaller = Join-Path $baselineDir "AntBrowser-Setup-$BaselineVersion.exe"
$targetInstaller = Join-Path $targetDir "AntBrowser-Setup-$TargetVersion.exe"
$extractDir = Join-Path $targetDir "extracted"
$harnessDir = Join-Path $repoRoot "backend\cmd\app-update-e2e"
$harnessPath = Join-Path $harnessDir "main.go"

function Write-Step {
    param([string]$Text)
    Write-Host ""
    Write-Host "== $Text =="
}

function Require-File {
    param([string]$Path, [string]$Label)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Label missing: $Path"
    }
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
    $exitCode = 0
    if (Test-Path variable:LASTEXITCODE) {
        $exitCode = $global:LASTEXITCODE
    }
    if ($exitCode -ne 0) {
        throw "$FilePath $($Arguments -join ' ') failed with exit code $exitCode"
    }
}

function Stop-AntBrowser {
    foreach ($name in @("ant-chrome", "xray", "sing-box")) {
        Get-Process $name -ErrorAction SilentlyContinue | Stop-Process -Force
    }
    Start-Sleep -Milliseconds 500
}

function Copy-ReleaseArtifacts {
    param(
        [string]$Version,
        [string]$Destination
    )
    New-Item -ItemType Directory -Force $Destination | Out-Null
    Copy-Item -LiteralPath (Join-Path $outputDir "AntBrowser-Setup-$Version.exe") -Destination $Destination -Force
    Copy-Item -LiteralPath (Join-Path $outputDir "AntBrowser-$Version-windows-amd64.zip") -Destination $Destination -Force
    Copy-Item -LiteralPath (Join-Path $outputDir "AntBrowser-$Version-windows-amd64.zip.sha256") -Destination $Destination -Force
    Copy-Item -LiteralPath (Join-Path $outputDir "app-update-stable.json") -Destination $Destination -Force
    Copy-Item -LiteralPath (Join-Path $outputDir "app-update-stable.json.sha256") -Destination $Destination -Force
}

function Publish-Version {
    param([string]$Version, [string]$Destination)
    Write-Step "Publish $Version"
    Remove-Item -Recurse -Force $outputDir -ErrorAction SilentlyContinue
    & (Join-Path $repoRoot "bat\publish.ps1") -Target "W" -Version $Version
    Invoke-Native -FilePath "python" -Arguments @(
        (Join-Path $repoRoot "tools\app-update\verify-app-update-package.py"),
        (Join-Path $outputDir "app-update-stable.json"),
        (Join-Path $outputDir "AntBrowser-$Version-windows-amd64.zip"),
        "windows-amd64"
    )
    Copy-ReleaseArtifacts -Version $Version -Destination $Destination
}

function Write-Harness {
    Write-Step "Write app-update e2e harness"
    New-Item -ItemType Directory -Force $harnessDir | Out-Null
    @"
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ant-chrome/backend/internal/appupdate"
)

func dump(label string, value any) {
	data, _ := json.MarshalIndent(value, "", "  ")
	fmt.Println("===", label, "===")
	fmt.Println(string(data))
}

func main() {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		panic("LOCALAPPDATA is empty")
	}

	installRoot := filepath.Join(localAppData, "Programs", "Ant Browser")
	stateRoot := filepath.Join(localAppData, "Ant Browser")
	currentExe := filepath.Join(installRoot, "ant-chrome.exe")
	manifestPath := ``$ManifestPath``

	if _, err := os.Stat(currentExe); err != nil {
		panic(err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		panic(err)
	}

	layout := appupdate.NewLayout(installRoot, stateRoot)
	manager := appupdate.Manager{
		LocalAppVersion: "$BaselineVersion",
		Layout: layout,
		Platform: appupdate.WindowsBackend{
			CurrentExePath: currentExe,
			CurrentAppVersion: "$BaselineVersion",
		},
		ManifestProvider: appupdate.DefaultManifestProvider(func() appupdate.ManifestSourceResolution {
			return appupdate.ManifestSourceResolution{
				URL: manifestPath,
				Source: "local-e2e",
			}
		}),
	}

	ctx := context.Background()
	check, err := manager.Check(ctx)
	if err != nil {
		panic(err)
	}
	dump("check", check)

	download, err := manager.Download(ctx)
	if err != nil {
		panic(err)
	}
	dump("download", download)

	apply, err := manager.Apply(ctx)
	if err != nil {
		panic(err)
	}
	dump("apply", apply)
	fmt.Println("Apply runner spawned. This process will exit now.")
	time.Sleep(500 * time.Millisecond)
}
"@ | Set-Content -LiteralPath $harnessPath -Encoding UTF8
}

function Assert-UpdateSucceeded {
    Write-Step "Verify app-update result"
    Require-File -Path $statePath -Label "app-update state.json"
    $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
    if ([string]$state.localAppVersion -ne $TargetVersion) {
        throw "localAppVersion mismatch: expected $TargetVersion, got $($state.localAppVersion)"
    }
    if ($state.PSObject.Properties.Name -contains "lastError" -and $null -ne $state.lastError) {
        $lastErrorCode = ""
        $lastErrorMessage = ""
        if ($state.lastError.PSObject.Properties.Name -contains "code") {
            $lastErrorCode = [string]$state.lastError.code
        }
        if ($state.lastError.PSObject.Properties.Name -contains "message") {
            $lastErrorMessage = [string]$state.lastError.message
        }
        if ($lastErrorCode.Trim() -ne "" -or $lastErrorMessage.Trim() -ne "") {
            $lastError = ($state.lastError | ConvertTo-Json -Depth 10 -Compress)
            throw "app-update lastError is not empty: $lastError"
        }
    }

    Remove-Item -Recurse -Force $extractDir -ErrorAction SilentlyContinue
    Expand-Archive -LiteralPath $targetZip -DestinationPath $extractDir -Force
    $installedExe = Join-Path $installRoot "ant-chrome.exe"
    $expectedExe = Join-Path $extractDir "ant-chrome.exe"
    Require-File -Path $installedExe -Label "installed ant-chrome.exe"
    Require-File -Path $expectedExe -Label "target zip ant-chrome.exe"
    $installedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $installedExe).Hash
    $expectedHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $expectedExe).Hash
    if ($installedHash -ne $expectedHash) {
        throw "installed exe hash mismatch: expected $expectedHash, got $installedHash"
    }

    Require-File -Path (Join-Path $installRoot "data\app.db") -Label "data\app.db"
    foreach ($preserved in @("runtime", "diagnostics")) {
        if (-not (Test-Path -LiteralPath (Join-Path $installRoot $preserved) -PathType Container)) {
            throw "preserved directory missing: $preserved"
        }
    }
    Require-File -Path (Join-Path $installRoot "config.yaml") -Label "config.yaml"

    Write-Host "[OK] Windows app-update e2e passed"
    Write-Host "Installed exe sha256: $installedHash"
    Write-Host "State: $($state | ConvertTo-Json -Depth 10 -Compress)"
}

Write-Step "Preflight"
if ([System.Environment]::OSVersion.Platform -ne [System.PlatformID]::Win32NT) {
    throw "windows-app-update-e2e.ps1 must run on Windows"
}
Require-Command "go"
Require-Command "python"
Require-File -Path (Join-Path $repoRoot "bat\publish.ps1") -Label "bat\publish.ps1"
New-Item -ItemType Directory -Force $TestRoot | Out-Null

if (-not $SkipPublish) {
    Publish-Version -Version $BaselineVersion -Destination $baselineDir
    Publish-Version -Version $TargetVersion -Destination $targetDir
}

Require-File -Path $baselineInstaller -Label "baseline installer"
Require-File -Path $targetInstaller -Label "target installer"
Require-File -Path $manifestPath -Label "target app-update manifest"
Require-File -Path $targetZip -Label "target app-update zip"

Write-Step "Install baseline $BaselineVersion"
Stop-AntBrowser
Remove-Item -Recurse -Force (Join-Path $stateRoot "app-update") -ErrorAction SilentlyContinue
Invoke-Native -FilePath $baselineInstaller -Arguments @("/S")
Stop-AntBrowser
Require-File -Path (Join-Path $installRoot "ant-chrome.exe") -Label "baseline ant-chrome.exe"
if (Test-Path -LiteralPath (Join-Path $installRoot "data\app.db") -PathType Leaf) {
    $beforeDataHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $installRoot "data\app.db")).Hash
} else {
    $beforeDataHash = ""
}

Write-Step "Configure local manifest"
[Environment]::SetEnvironmentVariable("DESKTOP_APP_UPDATE_MANIFEST_URL", $manifestPath, "User")
$env:DESKTOP_APP_UPDATE_MANIFEST_URL = $manifestPath

Write-Harness
Write-Step "Run Check -> Download -> Apply"
Push-Location $repoRoot
try {
    Invoke-Native -FilePath "go" -Arguments @("run", ".\backend\cmd\app-update-e2e")
}
finally {
    Pop-Location
}

Write-Step "Wait for runner"
Start-Sleep -Seconds $RunnerWaitSeconds
Stop-AntBrowser

if ($beforeDataHash -ne "") {
    $afterDataHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $installRoot "data\app.db")).Hash
    if ($beforeDataHash -ne $afterDataHash) {
        throw "data\app.db hash changed: before $beforeDataHash, after $afterDataHash"
    }
}

Assert-UpdateSucceeded
