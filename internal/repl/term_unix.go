//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package repl

import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

// makeRaw puts a terminal into the mode the line editor needs and returns the
// function that puts it back.
//
// This is a direct termios ioctl rather than golang.org/x/term because the
// whole of what is needed is below: two ioctls and five flags. Adding a module
// requirement to a tool that has none, and whose generated output has none
// either, is a poor trade for thirty lines.
//
// Output processing is deliberately left on, so a newline still turns into a
// carriage return and a line feed and the rest of the program can print
// normally while a session is open.
func makeRaw(fd uintptr) (func(), error) {
	var saved syscall.Termios
	if err := ioctl(fd, ioctlGetTermios, &saved); err != nil {
		return nil, errNoRawMode
	}
	raw := saved
	// Read one byte at a time, unbuffered, with no echo, no line editing by
	// the kernel, and no signal generation: Ctrl-C has to arrive as a byte so
	// that it can discard a pending snippet instead of killing the session.
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctl(fd, ioctlSetTermios, &raw); err != nil {
		return nil, errNoRawMode
	}
	restore := saved
	return func() { _ = ioctl(fd, ioctlSetTermios, &restore) }, nil
}

// watchForSignals puts the terminal back before a signal that would otherwise
// end the process takes it away.
//
// Raw mode is a property of the terminal, not of this program, so a session
// killed from another window must not leave the shell behind it without echo.
// The default behaviour is restored and the signal re-raised, so the process
// still dies the way the sender asked it to.
func watchForSignals(restore func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		s, ok := <-sigs
		if !ok {
			return
		}
		restore()
		signal.Stop(sigs)
		sig, ok := s.(syscall.Signal)
		if !ok {
			os.Exit(1)
		}
		_ = syscall.Kill(syscall.Getpid(), sig)
	}()
}

func ioctl(fd, request uintptr, t *syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

// winsize mirrors struct winsize from the kernel headers. Only the column
// count is used, to decide when a long line has to scroll sideways.
type winsize struct {
	rows, cols, xpixel, ypixel uint16
}

// terminalWidth returns the terminal's column count, or 80 when it cannot be
// asked. Eighty is the right guess: it is the width that has never been wrong
// enough to matter.
func terminalWidth(fd uintptr) int {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd,
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno != 0 || ws.cols == 0 {
		return 80
	}
	return int(ws.cols)
}
