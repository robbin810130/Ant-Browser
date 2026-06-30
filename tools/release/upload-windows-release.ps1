param(
    [Parameter(Mandatory = $true)]
    [string]$ReleaseVersion,
    [string]$Channel = "test",
    [string]$OutputDir = "publish\output",
    [string]$RemoteRoot = "/opt/1688shop/releases/windows",
    [switch]$AllowOverwrite
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$Version = $ReleaseVersion

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

function Invoke-NativeWithRetry {
    param(
        [string]$FilePath,
        [string[]]$Arguments = @(),
        [int]$MaxAttempts = 3,
        [int]$RetryDelaySeconds = 10,
        [string]$Description = "native command"
    )

    $lastExitCode = 0
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        Write-Host "$Description (attempt $attempt/$MaxAttempts)"
        & $FilePath @Arguments
        $lastExitCode = $LASTEXITCODE
        if ($lastExitCode -eq 0) {
            return
        }
        if ($attempt -lt $MaxAttempts) {
            Write-Warning "$Description failed with exit code $lastExitCode; retrying in $RetryDelaySeconds seconds"
            Start-Sleep -Seconds $RetryDelaySeconds
        }
    }

    throw "$FilePath $($Arguments -join ' ') failed with exit code $lastExitCode after $MaxAttempts attempts"
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

function Normalize-PrivateKeyText {
    param([Parameter(Mandatory = $true)][string]$Value)

    $normalized = $Value.Trim().Replace("`r`n", "`n").Replace("`r", "`n")
    if ($normalized -notmatch "`n" -and $normalized.Contains('\n')) {
        $normalized = $normalized.Replace('\n', "`n")
    }
    $header = "-----BEGIN OPENSSH PRIVATE KEY-----"
    $footer = "-----END OPENSSH PRIVATE KEY-----"
    if ($normalized -notmatch "`n" -and $normalized.StartsWith($header) -and $normalized.EndsWith($footer)) {
        $body = $normalized.Substring($header.Length, $normalized.Length - $header.Length - $footer.Length)
        $body = $body.Trim() -replace "\s+", ""
        $lines = @()
        for ($i = 0; $i -lt $body.Length; $i += 70) {
            $lines += $body.Substring($i, [Math]::Min(70, $body.Length - $i))
        }
        return ((@($header) + $lines + @($footer)) -join "`n") + "`n"
    }
    return $normalized.TrimEnd() + "`n"
}

$hostName = Require-Env "WINDOWS_RELEASE_SSH_HOST"
$port = Require-Env "WINDOWS_RELEASE_SSH_PORT"
$user = Require-Env "WINDOWS_RELEASE_SSH_USER"
$keyText = Require-Env "WINDOWS_RELEASE_SSH_KEY"

$tempKey = Join-Path $env:TEMP "windows-release-key-$([guid]::NewGuid().ToString('N')).pem"
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($tempKey, (Normalize-PrivateKeyText -Value $keyText), $utf8NoBom)
Protect-PrivateKeyFile -Path $tempKey

$published = $false
$stagingDir = ""
$sshOptions = @()

try {
    $target = "$user@$hostName"
    $channelDir = "$RemoteRoot/$Channel"
    $remoteDir = "$RemoteRoot/$Channel/$Version"
    $uploadId = [guid]::NewGuid().ToString("N")
    $stagingDir = "$channelDir/.uploading-$Version-$uploadId"
    $sshOptions = @(
        "-o", "BatchMode=yes",
        "-o", "ConnectTimeout=15",
        "-o", "ServerAliveInterval=5",
        "-o", "ServerAliveCountMax=6",
        "-o", "StrictHostKeyChecking=accept-new"
    )
    $sshBaseArgs = @("-i", $tempKey, "-p", $port) + $sshOptions
    $payloadArtifacts = @(
        "MakaBrowser-Setup-$Version.exe",
        "MakaBrowser-$Version-windows-amd64.zip",
        "MakaBrowser-$Version-windows-amd64.zip.sha256"
    )
    $manifestArtifacts = @(
        "app-update-stable.json",
        "app-update-stable.json.sha256",
        "release-report.json",
        "release-report.md"
    )
    $artifacts = $payloadArtifacts + $manifestArtifacts

    foreach ($artifact in $artifacts) {
        Require-File (Join-Path $resolvedOutputDir $artifact)
    }

    $overwriteFlag = if ($AllowOverwrite) { "1" } else { "0" }
    $requiredBytes = 0
    foreach ($artifact in $artifacts) {
        $requiredBytes += (Get-Item -LiteralPath (Join-Path $resolvedOutputDir $artifact)).Length
    }
    $requiredKb = [Math]::Ceiling(($requiredBytes * 2) / 1024)

    $prepareRemote = @"
set -eu
mkdir -p '$channelDir'
if [ -e '$remoteDir' ] && [ '$overwriteFlag' != '1' ]; then
  echo 'remote release directory exists: $remoteDir' >&2
  exit 23
fi
available_kb=`$(df -Pk '$channelDir' | awk 'NR==2 {print `$4}')
if [ -z "`$available_kb" ] || [ "`$available_kb" -lt $requiredKb ]; then
  echo "insufficient remote disk space in $channelDir: available_kb=`$available_kb required_kb=$requiredKb" >&2
  exit 24
fi
rm -rf '$stagingDir'
mkdir -p '$stagingDir'
"@ -replace "(`r`n|`n|`r)+", "; "
    Invoke-Native -FilePath "ssh" -Arguments ($sshBaseArgs + @($target, $prepareRemote))

    foreach ($artifact in $payloadArtifacts) {
        $localPath = Join-Path $resolvedOutputDir $artifact
        $remotePath = "${target}:$stagingDir/$artifact"
        $scpArgs = @(
            "-i", $tempKey,
            "-P", $port
        ) + $sshOptions + @(
            $localPath,
            $remotePath
        )
        Invoke-NativeWithRetry -FilePath "scp" -Arguments $scpArgs -MaxAttempts 3 -RetryDelaySeconds 10 -Description "Upload payload $artifact"
    }

    foreach ($artifact in $manifestArtifacts) {
        $localPath = Join-Path $resolvedOutputDir $artifact
        $remotePath = "${target}:$stagingDir/$artifact"
        $scpArgs = @(
            "-i", $tempKey,
            "-P", $port
        ) + $sshOptions + @(
            $localPath,
            $remotePath
        )
        Invoke-NativeWithRetry -FilePath "scp" -Arguments $scpArgs -MaxAttempts 3 -RetryDelaySeconds 10 -Description "Upload manifest $artifact"
    }

    $verifyRemote = "set -eu; cd '$stagingDir'; sha256sum MakaBrowser-$Version-windows-amd64.zip app-update-stable.json > remote-sha256.txt; cat remote-sha256.txt"
    Invoke-Native -FilePath "ssh" -Arguments ($sshBaseArgs + @($target, $verifyRemote))

    $publishRemote = "set -eu; if [ '$overwriteFlag' = '1' ]; then rm -rf '$remoteDir'; fi; if [ -e '$remoteDir' ]; then echo 'remote release directory exists: $remoteDir' >&2; exit 23; fi; mv -f '$stagingDir' '$remoteDir'"
    Invoke-Native -FilePath "ssh" -Arguments ($sshBaseArgs + @($target, $publishRemote))
    $published = $true

    Write-Host "[OK] uploaded Windows release $Version to $remoteDir"
}
finally {
    if ((-not $published) -and -not [string]::IsNullOrWhiteSpace($stagingDir)) {
        try {
            $cleanupTarget = "$user@$hostName"
            $cleanupArgs = @("-i", $tempKey, "-p", $port) + $sshOptions + @($cleanupTarget, "rm -rf '$stagingDir'")
            & "ssh" @cleanupArgs
        } catch {
            Write-Warning "failed to clean remote staging directory ${stagingDir}: $_"
        }
    }
    Remove-Item -LiteralPath $tempKey -Force -ErrorAction SilentlyContinue
}
