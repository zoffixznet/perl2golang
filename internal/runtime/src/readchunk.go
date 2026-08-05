package src

import (
	"io"
	"os"
)

// readChunk reads exactly n bytes from r when it can, and returns what
// arrived. A short answer means the input ended early, and an answer is
// never longer than asked for.
//
// io.ReadFull is the call that makes "exactly n" true: a plain Read may
// return fewer bytes than the buffer holds with no error at all, which works
// on a small file and quietly breaks on a pipe or a socket. The error is
// folded into the short answer here; code that must tell a closed pipe from
// a short file should call io.ReadFull itself and look at the error.
func readChunk(r io.Reader, n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	got, _ := io.ReadFull(r, buf)
	return string(buf[:got])
}

// tellPos is where the next read or write on f lands, counted in bytes from
// the start, and -1 when f cannot say.
//
// There is no Tell in the os package: asking is a seek of zero bytes from
// the current position, which moves nothing and reports where it stayed.
func tellPos(f *os.File) int {
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return -1
	}
	return int(pos)
}
