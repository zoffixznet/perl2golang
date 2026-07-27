//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package repl

import "syscall"

// macOS and the BSDs spell the same two ioctls differently.
const (
	ioctlGetTermios = syscall.TIOCGETA
	ioctlSetTermios = syscall.TIOCSETA
)
