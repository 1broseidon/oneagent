//go:build windows

package oneagent

import (
	"context"
	"os/exec"
)

func applyCancelableCommandContext(_ *exec.Cmd, _ context.Context) {}
