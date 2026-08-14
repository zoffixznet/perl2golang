//go:build !windows && !openbsd && !(linux || darwin || dragonfly || freebsd)

package ai

// availableBytes has no system call to make on a platform whose standard
// library exposes none, so it reports zero with no error. Zero is read as
// unknown everywhere it lands: the size checks skip themselves rather than
// refuse a download, and the summary prints "unknown" rather than a figure it
// did not measure.
func availableBytes(dir string) (uint64, error) { return 0, nil }
