//go:build openbsd

package ai

import "syscall"

// availableBytes reports the space an unprivileged user can still write on the
// filesystem holding dir. OpenBSD keeps the same numbers as the other BSDs
// under their C names, which is the only thing that differs.
func availableBytes(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.F_bavail) * uint64(st.F_bsize), nil
}
