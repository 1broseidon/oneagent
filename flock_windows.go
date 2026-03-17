//go:build windows

package oneagent

// Windows has no flock. Locking is best-effort and skipped on this platform.
func flockShared(_ uintptr) error  { return nil }
func flockExcl(_ uintptr) error    { return nil }
func flockUnlock(_ uintptr) error  { return nil }
