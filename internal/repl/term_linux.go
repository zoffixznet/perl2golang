//go:build linux

package repl

import "syscall"

// The termios ioctl numbers differ between Linux and the BSDs, and they are
// the only thing that does.
const (
	ioctlGetTermios = syscall.TCGETS
	ioctlSetTermios = syscall.TCSETS
)
