param(
    [Parameter(Mandatory = $true)]
    [string]$WorkspaceServerRoot,

    [Parameter(Mandatory = $true)]
    [string]$StdoutLog,

    [Parameter(Mandatory = $true)]
    [string]$StderrLog
)

$nodeCmd = Get-Command node.exe -ErrorAction SilentlyContinue
if (-not $nodeCmd) {
    $nodeCmd = Get-Command node -ErrorAction SilentlyContinue
}

if (-not $nodeCmd) {
    throw "Cannot find node.exe in PATH."
}

$serverEntry = Join-Path $WorkspaceServerRoot "server/index.mjs"
if (-not (Test-Path $serverEntry)) {
    throw "Workspace server entry not found: $serverEntry"
}

$process = Start-Process `
    -FilePath $nodeCmd.Source `
    -ArgumentList "--experimental-sqlite", $serverEntry `
    -WorkingDirectory $WorkspaceServerRoot `
    -RedirectStandardOutput $StdoutLog `
    -RedirectStandardError $StderrLog `
    -PassThru

Write-Output $process.Id
