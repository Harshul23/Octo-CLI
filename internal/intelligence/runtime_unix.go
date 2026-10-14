//go:build !windows

package intelligence

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/process"
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
	// Check the whole group until every member is gone or a zombie (which
	// owns no descriptors). The shell alone may exit before its descendants.
	waitGroup := func() error {
		deadline := time.Now().Add(timeout)
		for {
			members, err := process.Processes()
			if err != nil {
				return fmt.Errorf("list process group %d: %w", pgid, err)
			}
			alive := false
			for _, member := range members {
				id, err := syscall.Getpgid(int(member.Pid))
				if err != nil || id != pgid {
					continue
				}
				states, err := member.Status()
				if os.IsNotExist(err) {
					continue
				} // Exited between enumeration and status.
				if err != nil {
					return fmt.Errorf("check process %d status: %w", member.Pid, err)
				}
				zombie := false
				for _, state := range states {
					if state == process.Zombie {
						zombie = true
					}
				}
				if !zombie {
					alive = true
				}
			}
			if !alive {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for process group %d", pgid)
			}
			// This is a bounded termination wait, not a filesystem-cleanup retry.
			time.Sleep(10 * time.Millisecond)
		}
	}
	if pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		select {
		case <-waitDone:
			// The shell may exit before a descendant does. Kill any remaining
			// members of its process group as well.
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			return waitGroup()
		case <-time.After(timeout):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			select {
			case <-waitDone:
				return waitGroup()
			case <-time.After(1 * time.Second):
				return fmt.Errorf("timed out waiting for process %d after group termination", pgid)
			}
		}
	}
	_ = cmd.Process.Kill()
	select {
	case <-waitDone:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for process %d after termination", cmd.Process.Pid)
	}
}
