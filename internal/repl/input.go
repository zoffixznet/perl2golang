package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// lineSource is where one line of input comes from.
//
// There are two: a raw-mode editor when a terminal is on both ends, and plain
// line-buffered reading everywhere else. The rest of the REPL cannot tell
// which it is talking to, which is what makes a piped session and a typed
// session take exactly the same code path.
type lineSource interface {
	// readLine writes the prompt and returns one line without its newline.
	// It returns io.EOF at the end of input and errInterrupt for Ctrl-C.
	readLine(prompt string) (string, error)
	// fallback explains, once, why line editing is not available. Empty when
	// it is.
	fallback() string
	close()
}

// newLineSource picks the reader for this session.
func newLineSource(o Options, h *history) lineSource {
	if o.Interactive {
		in, inOK := o.Stdin.(*os.File)
		out, outOK := o.Stdout.(*os.File)
		if inOK && outOK {
			if ed, err := newEditor(in, out, h, o.Color); err == nil {
				return ed
			} else if !errors.Is(err, errNoRawMode) {
				return &plain{
					in:   bufio.NewReader(o.Stdin),
					out:  o.Stdout,
					note: fmt.Sprintf("line editing is off: %v", err),
				}
			}
		}
		return &plain{
			in:  bufio.NewReader(o.Stdin),
			out: o.Stdout,
			note: "line editing is off: this terminal does not support raw mode, so arrow keys " +
				"and history are unavailable. Everything else works.",
		}
	}
	// Input is a file or a pipe. Echoing what was read keeps the transcript
	// readable: `perl2golang repl < session.pl` reads exactly like somebody typed
	// it, which is what makes a session a demo and a test fixture at once.
	return &plain{in: bufio.NewReader(o.Stdin), out: o.Stdout, echo: true}
}

// plain reads whole lines and does no editing.
type plain struct {
	in   *bufio.Reader
	out  io.Writer
	echo bool
	note string
}

func (p *plain) fallback() string { return p.note }
func (p *plain) close()           {}

func (p *plain) readLine(prompt string) (string, error) {
	fmt.Fprint(p.out, prompt)
	line, err := p.in.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err != nil {
		if line == "" {
			return "", io.EOF
		}
		// A last line with no newline is still a line.
		if p.echo {
			fmt.Fprintln(p.out, line)
		}
		return line, nil
	}
	if p.echo {
		fmt.Fprintln(p.out, line)
	}
	return line, nil
}
