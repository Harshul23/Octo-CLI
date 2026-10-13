//go:build !windows

package intelligence

import (
	"os/exec"
	"syscall"
	"time"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func gracefulStopShellProcess(cmd *exec.Cmd, waitDone <-chan struct{}, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	// Check if already finished
	select {
	case <-waitDone:
		return nil
	default:
	}

	pgid := cmd.Process.Pid
	if pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		select {
		case <-waitDone:
			return nil
		case <-time.After(timeout):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			select {
			case <-waitDone:
				return nil
			case <-time.After(1 * time.Second):
				return nil
			}
		}
	}
	_ = cmd.Process.Kill()
	select {
	case <-waitDone:
		return nil
	case <-time.After(timeout):
		return nil
	}
}
