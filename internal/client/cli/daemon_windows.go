//go:build windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

// getSysProcAttr returns platform-specific process attributes for daemonization
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// isProcessRunningOS checks if a process is running using OS-specific method
func isProcessRunningOS(process *os.Process) bool {
	// On Windows, we try to open the process to check if it exists
	// FindProcess doesn't actually check if process exists on Windows
	// We can try to send signal, but Windows doesn't support signal 0
	// Instead, we'll try to kill with signal 0 which returns an error if process doesn't exist
	err := process.Signal(os.Signal(syscall.Signal(0)))
	if err != nil {
		// Try alternative: check if we can get process info
		// If the process doesn't exist, Signal will fail
		return false
	}
	return true
}

// killProcessOS kills a process using OS-specific method
func killProcessOS(process *os.Process) error {
	// On Windows, use Kill() directly
	return process.Kill()
}

// setupDaemonCmd configures the command for daemon mode
func setupDaemonCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = getSysProcAttr()
}
