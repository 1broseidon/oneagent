//go:build unix

package oneagent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
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
		pid := cmd.Process.Pid
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		go func() {
			timer := time.NewTimer(5 * time.Second)
			defer timer.Stop()
			<-timer.C
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}()
		return nil
	}
}
