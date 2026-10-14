//go:build windows

package intelligence

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/shirou/gopsutil/v3/process"
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
	// Capture descendants before taskkill so we can confirm their termination,
	// not merely that the command shell's Wait has returned.
	root, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		return fmt.Errorf("inspect process tree %d: %w", cmd.Process.Pid, err)
	}
	tree := []*process.Process{root}
	for i := 0; i < len(tree); i++ {
		// Cache identity before termination; later checks must not confuse PID reuse
		// with a still-running descendant or read creation time after it vanished.
		if _, err := tree[i].CreateTime(); err != nil {
			return fmt.Errorf("inspect process %d identity: %w", tree[i].Pid, err)
		}
		children, err := tree[i].Children()
		if err != nil {
			return fmt.Errorf("inspect children of process %d: %w", tree[i].Pid, err)
		}
		tree = append(tree, children...)
	}
	if cmd.Process.Pid > 0 {
		if output, err := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).CombinedOutput(); err != nil {
			// Do not report success if descendants may still own inherited log handles.
			_ = cmd.Process.Kill()
			return fmt.Errorf("taskkill process tree %d: %w: %s", cmd.Process.Pid, err, output)
		}
	}
	_ = cmd.Process.Kill()
	select {
	case <-waitDone:
		deadline := time.Now().Add(timeout)
		for {
			alive := false
			for _, member := range tree {
				running, err := member.IsRunning()
				if err != nil {
					return fmt.Errorf("check process %d termination: %w", member.Pid, err)
				}
				if running {
					alive = true
				}
			}
			if !alive {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for process tree %d", cmd.Process.Pid)
			}
			time.Sleep(10 * time.Millisecond)
		}
	case <-time.After(timeout):
		return fmt.Errorf("timed out waiting for process %d after tree termination", cmd.Process.Pid)
	}
}
