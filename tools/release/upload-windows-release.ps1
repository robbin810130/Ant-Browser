param(
    [Parameter(Mandatory = $true)]
    [Alias("ReleaseVersion")]
    [string]$Version,
    [string]$Channel = "test",
    [string]$OutputDir = "publish\output",
    [string]$RemoteRoot = "/opt/1688shop/releases/windows",
    [switch]$AllowOverwrite
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$resolvedOutputDir = Join-Path $repoRoot $OutputDir

function Require-Env {
    param([string]$Name)

    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "required environment variable missing: $Name"
    }
    return $value.Trim()
}

function Require-File {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "required artifact missing: $Path"
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

function Protect-PrivateKeyFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    if ($env:OS -eq "Windows_NT") {
        $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
        $acl = Get-Acl -LiteralPath $Path
        $acl.SetAccessRuleProtection($true, $false)
        $rule = New-Object System.Security.AccessControl.FileSystemAccessRule($identity, "FullControl", "Allow")
        $acl.SetAccessRule($rule)
        Set-Acl -LiteralPath $Path -AclObject $acl
        return
    }

    Invoke-Native -FilePath "chmod" -Arguments @("600", $Path)
}

$hostName = Require-Env "WINDOWS_RELEASE_SSH_HOST"
$port = Require-Env "WINDOWS_RELEASE_SSH_PORT"
$user = Require-Env "WINDOWS_RELEASE_SSH_USER"
$keyText = Require-Env "WINDOWS_RELEASE_SSH_KEY"

$tempKey = Join-Path $env:TEMP "windows-release-key-$([guid]::NewGuid().ToString('N')).pem"
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($tempKey, $keyText.Replace("`r`n", "`n"), $utf8NoBom)
Protect-PrivateKeyFile -Path $tempKey

try {
    $target = "$user@$hostName"
    $remoteDir = "$RemoteRoot/$Channel/$Version"
    $sshBaseArgs = @("-i", $tempKey, "-p", $port, "-o", "StrictHostKeyChecking=accept-new")
    $artifacts = @(
        "AntBrowser-Setup-$Version.exe",
        "AntBrowser-$Version-windows-amd64.zip",
        "AntBrowser-$Version-windows-amd64.zip.sha256",
        "app-update-stable.json",
        "app-update-stable.json.sha256",
        "release-report.json",
        "release-report.md"
    )

    foreach ($artifact in $artifacts) {
        Require-File (Join-Path $resolvedOutputDir $artifact)
    }

    $overwriteFlag = if ($AllowOverwrite) { "1" } else { "0" }
    $prepareRemote = "set -eu; if [ -e '$remoteDir' ] && [ '$overwriteFlag' != '1' ]; then echo 'remote release directory exists: $remoteDir' >&2; exit 23; fi; mkdir -p '$remoteDir'"
    Invoke-Native -FilePath "ssh" -Arguments ($sshBaseArgs + @($target, $prepareRemote))

    foreach ($artifact in $artifacts) {
        $localPath = Join-Path $resolvedOutputDir $artifact
        $remotePath = "${target}:$remoteDir/$artifact"
        Invoke-Native -FilePath "scp" -Arguments @(
            "-i", $tempKey,
            "-P", $port,
            "-o", "StrictHostKeyChecking=accept-new",
            $localPath,
            $remotePath
        )
    }

    $verifyRemote = "set -eu; cd '$remoteDir'; sha256sum AntBrowser-$Version-windows-amd64.zip app-update-stable.json > remote-sha256.txt; cat remote-sha256.txt"
    Invoke-Native -FilePath "ssh" -Arguments ($sshBaseArgs + @($target, $verifyRemote))

    Write-Host "[OK] uploaded Windows release $Version to $remoteDir"
}
finally {
    Remove-Item -LiteralPath $tempKey -Force -ErrorAction SilentlyContinue
}
