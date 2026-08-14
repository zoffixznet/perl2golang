//go:build linux || darwin || dragonfly || freebsd

package ai

import "syscall"

// availableBytes reports the space an unprivileged user can still write on the
// filesystem holding dir.
//
// Bavail, not Bfree: the difference between them is the reserve only the
// superuser may spend, which a model download cannot use. The fields are
// spelled the same on every system here but typed differently, hence the
// conversions.
func availableBytes(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
