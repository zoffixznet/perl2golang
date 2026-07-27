//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd)

package repl

// On a platform without termios the session reads whole lines from the
// terminal driver instead. Everything works except the arrow keys and history
// recall, and the banner says so once.
func makeRaw(fd uintptr) (func(), error) { return nil, errNoRawMode }

func terminalWidth(fd uintptr) int { return 80 }

// watchForSignals has nothing to restore where there is no raw mode.
func watchForSignals(restore func()) {}
