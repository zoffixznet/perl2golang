package src

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

// This file holds the helpers for running another program.
//
// The shape differs from the original in one way that matters more than any
// other: exec.Command takes an argument list and starts the program directly,
// where a command written as one string is handed to a shell, which splits it
// into words, expands globs in it, and runs anything a semicolon introduces.
// The list form is the safe one and the one to reach for; the string form is
// kept only for a command line that was already written as text.

// runProgram runs a program with this program's own standard input, output
// and error, and returns the wait status the original recorded.
func runProgram(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return waitStatus(cmd.Run())
}

// runShell runs a command line through the shell, which is what a command
// written as one string does.
//
// Everything the shell does to that string still happens: word splitting,
// glob expansion, redirection, and running whatever follows a semicolon. That
// is a large door to leave open, and passing the words to runProgram closes
// it, at the cost of writing them out.
func runShell(line string) int {
	return runProgram("sh", "-c", line)
}

// captureProgram runs a program and returns everything it wrote to standard
// output, together with the wait status. Its standard error is left joined to
// this program's, which is where it went before.
func captureProgram(name string, args ...string) (string, int) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), waitStatus(err)
}

// captureShell is captureProgram for a command line written as one string.
func captureShell(line string) (string, int) {
	return captureProgram("sh", "-c", line)
}

// captureLines is captureShell for a caller that wanted the output as a list.
// The newline stays on the end of each line, which is what a read gives, and
// a final line with no newline is still a line.
func captureLines(line string) ([]string, int) {
	out, status := captureShell(line)
	var lines []string
	for len(out) > 0 {
		i := strings.IndexByte(out, '\n')
		if i < 0 {
			lines = append(lines, out)
			break
		}
		lines = append(lines, out[:i+1])
		out = out[i+1:]
	}
	return lines, status
}

// waitStatus encodes what a child did the way the original recorded it: the
// exit code in the high byte, so that shifting right by eight gives the code
// and the low seven bits name a signal.
//
// A child killed by a signal is the one case this cannot reproduce
// faithfully: the standard library reports the code as -1 without saying
// which signal, so that is what comes back.
func waitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode() << 8
	}
	return -1
}

// pipeHandle is a program running alongside this one, with one end of a pipe
// held open to it.
//
// It reads and writes like a file, so the code around it does not have to know
// which it has, and closing it waits for the program to finish, which is what
// closing a pipe did before.
type pipeHandle struct {
	cmd    *exec.Cmd
	out    io.ReadCloser
	in     io.WriteCloser
	status int
	done   bool
}

// openRead starts a program and hands back a handle its standard output can
// be read from.
func openRead(name string, args ...string) (*pipeHandle, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &pipeHandle{cmd: cmd, out: out}, nil
}

// openWrite starts a program and hands back a handle that writes to its
// standard input.
func openWrite(name string, args ...string) (*pipeHandle, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &pipeHandle{cmd: cmd, in: in}, nil
}

// Read reads what the program has written so far.
func (p *pipeHandle) Read(b []byte) (int, error) {
	if p.out == nil {
		return 0, errors.New("this pipe was opened for writing")
	}
	return p.out.Read(b)
}

// Write hands more input to the program.
func (p *pipeHandle) Write(b []byte) (int, error) {
	if p.in == nil {
		return 0, errors.New("this pipe was opened for reading")
	}
	return p.in.Write(b)
}

// Close finishes the conversation and waits for the program to exit.
//
// The wait is the part a file close does not do and a pipe close always did:
// without it the program would still be running, its output might not have
// arrived, and its exit status would never be known.
func (p *pipeHandle) Close() error {
	if p.done {
		return nil
	}
	p.done = true
	if p.in != nil {
		p.in.Close()
	}
	err := p.cmd.Wait()
	p.status = waitStatus(err)
	if p.out != nil {
		p.out.Close()
	}
	return err
}

// Status is how the program finished, in the form the wait status had.
func (p *pipeHandle) Status() int { return p.status }
