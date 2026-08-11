package src

import (
	"errors"
	"syscall"
	"unicode"
)

// errnoText renders an error the way the C library renders errno: the reason
// the operation failed, capitalised, and nothing else.
//
// Go's errors carry more than that on purpose. os.Open returns a *fs.PathError
// whose text is "open /etc/config: no such file or directory", naming the
// operation and the file as well as the reason, which is what you want in a
// log and is strictly better than the reason alone. This exists for messages
// that were written around the shorter form and read badly with the longer
// one substituted in.
//
// An error that is not an operating system error at all has no errno to
// render, so its own text is used unchanged.
func errnoText(err error) string {
	if err == nil {
		return ""
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	text := []rune(errno.Error())
	if len(text) == 0 {
		return ""
	}
	text[0] = unicode.ToUpper(text[0])
	return string(text)
}
