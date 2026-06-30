param(
    [Parameter(Mandatory = $true)]
    [string]$ReleaseVersion,
    [string]$RemoteRoot = "/opt/1688shop/releases/windows"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$Version = $ReleaseVersion.Trim()

if ($Version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$') {
    throw "invalid release version: $ReleaseVersion"
}
if ($RemoteRoot -notmatch '^/[0-9A-Za-z._/-]+$') {
    throw "invalid remote root: $RemoteRoot"
}

function Require-Env {
    param([string]$Name)

    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "required environment variable missing: $Name"
    }
    return $value.Trim()
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
        [int]$RetryDelaySeconds = 5,
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

function Invoke-RemoteBashScript {
    param(
        [Parameter(Mandatory = $true)][string]$TempKey,
        [Parameter(Mandatory = $true)][string]$Port,
        [Parameter(Mandatory = $true)][string[]]$SshOptions,
        [Parameter(Mandatory = $true)][string]$Target,
        [Parameter(Mandatory = $true)][string]$RemoteScriptPath,
        [Parameter(Mandatory = $true)][string]$Script,
        [string]$Description = "remote bash script"
    )

    $localScriptPath = Join-Path $env:TEMP "windows-promotion-remote-$([guid]::NewGuid().ToString('N')).sh"
    $normalizedScript = $Script.Trim().Replace("`r`n", "`n").Replace("`r", "`n") + "`n"
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($localScriptPath, $normalizedScript, $utf8NoBom)

    try {
        $scpArgs = @(
            "-i", $TempKey,
            "-P", $Port
        ) + $SshOptions + @(
            $localScriptPath,
            "${Target}:$RemoteScriptPath"
        )
        Invoke-NativeWithRetry `
            -FilePath "scp" `
            -Arguments $scpArgs `
            -MaxAttempts 3 `
            -RetryDelaySeconds 5 `
            -Description "Upload $Description"

        $sshArgs = @(
            "-i", $TempKey,
            "-p", $Port
        ) + $SshOptions + @(
            $Target,
            "bash '$RemoteScriptPath'"
        )
        Invoke-Native -FilePath "ssh" -Arguments $sshArgs
    }
    finally {
        Remove-Item -LiteralPath $localScriptPath -Force -ErrorAction SilentlyContinue
        $cleanupArgs = @(
            "-i", $TempKey,
            "-p", $Port
        ) + $SshOptions + @(
            $Target,
            "rm -f '$RemoteScriptPath'"
        )
        & "ssh" @cleanupArgs
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

try {
    $target = "$user@$hostName"
    $promotionId = [guid]::NewGuid().ToString("N")
    $remoteScriptPath = "/tmp/maka-promote-$Version-$promotionId.sh"
    $sshOptions = @(
        "-o", "BatchMode=yes",
        "-o", "ConnectTimeout=15",
        "-o", "ServerAliveInterval=5",
        "-o", "ServerAliveCountMax=6",
        "-o", "StrictHostKeyChecking=accept-new"
    )

    $remotePromotion = @"
set -eu
test_dir='$RemoteRoot/test/$Version'
stable_root='$RemoteRoot/stable'
stable_dir="`$stable_root/$Version"
staging_dir="`$stable_root/.promoting-$Version-$promotionId"
stable_alias="`$stable_root/app-update-stable.json"
stable_zip="`$stable_root/MakaBrowser-$Version-windows-amd64.zip"
stable_installer="`$stable_root/MakaBrowser-Setup-$Version.exe"
alias_tmp="`$stable_root/.app-update-stable-$Version-$promotionId.tmp"
zip_alias_tmp="`$stable_root/.MakaBrowser-$Version-windows-amd64-$promotionId.tmp"
installer_alias_tmp="`$stable_root/.MakaBrowser-Setup-$Version-$promotionId.tmp"
published=0

cleanup() {
  if [ "`$published" -ne 1 ]; then
    rm -rf "`$staging_dir"
  fi
  rm -f "`$alias_tmp" "`$zip_alias_tmp" "`$installer_alias_tmp"
}
trap cleanup EXIT

require_file() {
  if [ ! -f "`$1" ]; then
    echo "missing required release artifact: `$1" >&2
    exit 31
  fi
}

verify_release() {
  release_dir="`$1"
  require_file "`$release_dir/MakaBrowser-$Version-windows-amd64.zip"
  require_file "`$release_dir/MakaBrowser-$Version-windows-amd64.zip.sha256"
  require_file "`$release_dir/MakaBrowser-Setup-$Version.exe"
  require_file "`$release_dir/app-update-stable.json"
  require_file "`$release_dir/app-update-stable.json.sha256"
  (
    cd "`$release_dir"
    tr -d '\r' < "MakaBrowser-$Version-windows-amd64.zip.sha256" | sha256sum -c -
    tr -d '\r' < "app-update-stable.json.sha256" | sha256sum -c -
  )
}

publish_aliases() {
  release_dir="`$1"
  rm -f "`$zip_alias_tmp" "`$installer_alias_tmp" "`$alias_tmp"
  ln "`$release_dir/MakaBrowser-$Version-windows-amd64.zip" "`$zip_alias_tmp"
  ln "`$release_dir/MakaBrowser-Setup-$Version.exe" "`$installer_alias_tmp"
  cp "`$release_dir/app-update-stable.json" "`$alias_tmp"
  if [ -e "`$stable_zip" ] && [ "`$release_dir/MakaBrowser-$Version-windows-amd64.zip" -ef "`$stable_zip" ]; then
    rm -f "`$zip_alias_tmp"
  else
    mv -f "`$zip_alias_tmp" "`$stable_zip"
  fi
  if [ -e "`$stable_installer" ] && [ "`$release_dir/MakaBrowser-Setup-$Version.exe" -ef "`$stable_installer" ]; then
    rm -f "`$installer_alias_tmp"
  else
    mv -f "`$installer_alias_tmp" "`$stable_installer"
  fi
  mv -f "`$alias_tmp" "`$stable_alias"
}

if [ ! -d "`$test_dir" ]; then
  echo "missing test release: `$test_dir" >&2
  exit 32
fi

verify_release "`$test_dir"
mkdir -p "`$stable_root"

if [ -e "`$stable_dir" ]; then
  if [ ! -d "`$stable_dir" ]; then
    echo "stable release path is not a directory: `$stable_dir" >&2
    exit 33
  fi
  verify_release "`$stable_dir"
  cmp -s "`$test_dir/MakaBrowser-$Version-windows-amd64.zip" "`$stable_dir/MakaBrowser-$Version-windows-amd64.zip"
  cmp -s "`$test_dir/MakaBrowser-Setup-$Version.exe" "`$stable_dir/MakaBrowser-Setup-$Version.exe"
  cmp -s "`$test_dir/app-update-stable.json" "`$stable_dir/app-update-stable.json"
  publish_aliases "`$stable_dir"
  published=1
  echo "[OK] stable release already exists and matches test: $Version"
  exit 0
fi

rm -rf "`$staging_dir"
mkdir -p "`$staging_dir"
cp -a "`$test_dir/." "`$staging_dir/"
verify_release "`$staging_dir"
mv -f "`$staging_dir" "`$stable_dir"
publish_aliases "`$stable_dir"
published=1
echo "[OK] promoted $Version to stable"
"@

    Invoke-RemoteBashScript `
        -TempKey $tempKey `
        -Port $port `
        -SshOptions $sshOptions `
        -Target $target `
        -RemoteScriptPath $remoteScriptPath `
        -Script $remotePromotion `
        -Description "remote stable promotion script"
}
finally {
    Remove-Item -LiteralPath $tempKey -Force -ErrorAction SilentlyContinue
}
