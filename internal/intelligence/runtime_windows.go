//go:build windows

package intelligence

import (
	"fmt"
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
	if cmd.Process.Pid > 0 {
		_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run()
	}
	_ = cmd.Process.Kill()
	select {
	case <-waitDone:
		return nil
	case <-time.After(timeout):
		return nil
	}
}
