//go:build !windows

package ui

import "syscall"

func killProcessGroup(pid int, term bool) {
	if pid <= 0 {
		return
	}
	if term {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	} else {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}

func killProcessSingle(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
