//go:build !windows

package oneagent

import "syscall"

func flockShared(fd uintptr) error { return syscall.Flock(int(fd), syscall.LOCK_SH) }
func flockExcl(fd uintptr) error   { return syscall.Flock(int(fd), syscall.LOCK_EX) }
func flockUnlock(fd uintptr) error { return syscall.Flock(int(fd), syscall.LOCK_UN) }
