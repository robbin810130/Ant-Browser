//go:build !windows
// +build !windows

package backend

func cleanupWorkspaceBootstrapProcesses(string) (int, error) {
	return 0, nil
}
