//go:build windows
// +build windows

package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func cleanupWorkspaceBootstrapProcesses(installRoot string) (int, error) {
	installRoot = strings.TrimSpace(installRoot)
	if installRoot == "" {
		return 0, nil
	}
	if _, err := os.Stat(installRoot); err != nil {
		return 0, nil
	}

	agentEntry := filepath.Join(installRoot, "apps", "agent", "src", "server", "index.mjs")
	bridgeEntry := filepath.Join(installRoot, "installer", "windows", "scripts", "ant-runtime-bridge.mjs")

	psScript := `param([string]$AgentEntry, [string]$BridgeEntry)
$ErrorActionPreference = 'SilentlyContinue'
function Normalize-PathText([string]$value) {
  if ([string]::IsNullOrWhiteSpace($value)) { return '' }
  try { return [System.IO.Path]::GetFullPath($value).ToLowerInvariant() } catch { return $value.ToLowerInvariant() }
}
$agent = Normalize-PathText $AgentEntry
$bridge = Normalize-PathText $BridgeEntry
$targets = @($agent, $bridge) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
if ($targets.Count -eq 0) {
  Write-Output '0'
  exit 0
}
function Get-WorkspaceBootstrapProcesses {
  @(
    Get-CimInstance Win32_Process | Where-Object {
      $_.Name -and $_.Name.Equals('node.exe', [System.StringComparison]::OrdinalIgnoreCase) -and
      $_.CommandLine -and
      (($targets | Where-Object { $_.Length -gt 0 -and $_.CommandLine.ToLowerInvariant().Contains($_) }).Count -gt 0)
    }
  )
}
$found = @(Get-WorkspaceBootstrapProcesses | Sort-Object ProcessId -Descending)
foreach ($p in $found) {
  try { Stop-Process -Id $p.ProcessId -Force -ErrorAction Stop } catch {}
}
Start-Sleep -Milliseconds 400
$left = @(Get-WorkspaceBootstrapProcesses)
if ($left.Count -gt 0) {
  $names = ($left | ForEach-Object { $_.Name + '#' + $_.ProcessId }) -join ', '
  Write-Host ('still running: ' + $names)
  exit 1
}
Write-Output $found.Count
exit 0
`

	tempFile, err := os.CreateTemp("", "ant-workspace-cleanup-*.ps1")
	if err != nil {
		return 0, fmt.Errorf("创建 workspace 清理脚本失败: %w", err)
	}
	scriptPath := tempFile.Name()
	if _, err := tempFile.WriteString(psScript); err != nil {
		tempFile.Close()
		_ = os.Remove(scriptPath)
		return 0, fmt.Errorf("写入 workspace 清理脚本失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return 0, fmt.Errorf("关闭 workspace 清理脚本失败: %w", err)
	}
	defer os.Remove(scriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	powershellPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := os.Stat(powershellPath); err != nil {
		if fallbackPath, lookErr := exec.LookPath("powershell.exe"); lookErr == nil {
			powershellPath = fallbackPath
		} else {
			return 0, fmt.Errorf("未找到 powershell.exe")
		}
	}

	cmd := exec.CommandContext(
		ctx,
		powershellPath,
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", filepath.Clean(scriptPath),
		"-AgentEntry", agentEntry,
		"-BridgeEntry", bridgeEntry,
	)
	hideWindow(cmd)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return 0, fmt.Errorf("清理 workspace 残留进程超时")
	}
	message := strings.TrimSpace(string(output))
	if err != nil {
		if message == "" {
			message = err.Error()
		}
		return 0, fmt.Errorf("清理 workspace 残留进程失败: %s", message)
	}
	if message == "" {
		return 0, nil
	}
	count, parseErr := strconv.Atoi(strings.TrimSpace(message))
	if parseErr != nil {
		return 0, fmt.Errorf("解析 workspace 清理结果失败: %s", message)
	}
	return count, nil
}
