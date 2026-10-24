//go:build browser && unix

package browser_test

import (
	"os/exec"
	"syscall"
)

// ownProcessGroup starts cmd in a new process group and makes cancellation kill the
// whole group, so helper processes the browser started do not outlive the call.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
