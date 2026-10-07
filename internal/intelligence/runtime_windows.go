//go:build windows

package intelligence

import (
	"os/exec"
	"time"
)

func setProcessGroup(cmd *exec.Cmd) {
	// Process groups are not configured via Setpgid on Windows
}

func gracefulStopShellProcess(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
