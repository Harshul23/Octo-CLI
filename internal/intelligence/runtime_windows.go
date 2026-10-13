//go:build windows

package intelligence

import (
	"os/exec"
	"time"
)

func setProcessGroup(cmd *exec.Cmd) {
	// Process groups are not configured via Setpgid on Windows
}

func gracefulStopShellProcess(cmd *exec.Cmd, waitDone <-chan struct{}, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	select {
	case <-waitDone:
		return nil
	default:
	}
	_ = cmd.Process.Kill()
	select {
	case <-waitDone:
		return nil
	case <-time.After(timeout):
		return nil
	}
}
