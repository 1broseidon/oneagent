//go:build unix

package oneagent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func applyCancelableCommandContext(cmd *exec.Cmd, ctx context.Context) {
	if ctx == nil || ctx.Done() == nil {
		return
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
}
