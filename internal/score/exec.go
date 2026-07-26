package score

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// outputLimit caps how much of a program's stdout and stderr is kept. A corpus
// program prints kilobytes; anything past this is a runaway, and holding it in
// memory for 155 entries at once would be the only way this tool could fall
// over.
const outputLimit = 8 << 20

// Output is what came out of running one program.
type Output struct {
	Stdout []byte
	Stderr []byte
	// Exit is the exit status. It is -1 when the program never ran or was
	// killed by a signal.
	Exit int
	// TimedOut is true when the program was killed for overrunning its time
	// limit, which is a failure with its own reason rather than a wrong
	// answer.
	TimedOut bool
	// Err is set when the program could not be started at all.
	Err error
	// Elapsed is how long the program ran.
	Elapsed time.Duration
	// Truncated is true when the program printed more than outputLimit.
	Truncated bool
}

// runSpec describes one program to run. There is no shell anywhere in here: the
// argument list is passed to the process directly, so nothing in a corpus
// program's arguments can be interpreted as shell syntax.
type runSpec struct {
	// Dir is the working directory, which is what makes a program's relative
	// paths resolve.
	Dir  string
	Name string
	Args []string
	// Stdin is fed to the program; an empty slice means stdin is at end of
	// file immediately, never a terminal.
	Stdin []byte
	// Env replaces the inherited environment when non-nil.
	Env     []string
	Timeout time.Duration
}

// runProgram executes one program under a deadline and collects everything it said.
//
// A program that overruns its deadline is killed rather than waited on, so one
// runaway corpus entry cannot hang the scorecard.
func runProgram(ctx context.Context, spec runSpec) Output {
	if spec.Timeout <= 0 {
		spec.Timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()

	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = outputLimit, outputLimit

	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Stdin = bytes.NewReader(spec.Stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = spec.Env
	// Kill the whole process group's worth of stragglers by not waiting on
	// inherited pipes: the deadline must end the run even if a child holds
	// stdout open.
	cmd.WaitDelay = time.Second

	start := time.Now()
	err := cmd.Run()
	out := Output{
		Stdout:    stdout.Bytes(),
		Stderr:    stderr.Bytes(),
		Exit:      -1,
		Elapsed:   time.Since(start),
		Truncated: stdout.overflowed || stderr.overflowed,
	}
	if cmd.ProcessState != nil {
		out.Exit = cmd.ProcessState.ExitCode()
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.TimedOut = true
		out.Exit = -1
		return out
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			out.Err = err
			out.Exit = -1
		}
	}
	return out
}

// limitedBuffer is a bytes.Buffer that stops growing at a limit and remembers
// that it did.
type limitedBuffer struct {
	buf        bytes.Buffer
	limit      int
	overflowed bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.buf.Len()
	if room <= 0 {
		b.overflowed = true
		return len(p), nil
	}
	if len(p) > room {
		b.buf.Write(p[:room])
		b.overflowed = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }

// haveTool reports whether a program is on PATH.
func haveTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// copyTree copies a directory recursively. It is how each program gets its own
// private copy of an entry's fixtures: two programs that both write output.txt
// must never see each other's file.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case !d.Type().IsRegular():
			// Sockets and devices have no place in a corpus entry and
			// nothing sensible to copy.
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeFiles writes a set of files under dir, keyed by slash-separated path.
func writeFiles(dir string, files map[string][]byte) error {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		rel := filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("refusing to write %q outside the build directory", name)
		}
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, files[name], 0o644); err != nil {
			return err
		}
	}
	return nil
}

// snapshot maps every regular file under dir to a hash of its contents, keyed by
// slash-separated relative path. Comparing two snapshots is how the scorecard
// checks that two programs wrote the same files, not just the same stdout.
func snapshot(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			// A file the program deleted between the walk and the read
			// is a difference worth recording, not a crash.
			out[filepath.ToSlash(rel)] = "unreadable: " + err.Error()
			return nil
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	return out, err
}

// changesAgainst returns the paths under a snapshot that differ from the
// baseline the entry started with: the files the program created, changed or
// removed.
func changesAgainst(baseline, after map[string]string) map[string]string {
	changed := map[string]string{}
	for path, sum := range after {
		if baseline[path] != sum {
			changed[path] = sum
		}
	}
	for path := range baseline {
		if _, ok := after[path]; !ok {
			changed[path] = "(removed)"
		}
	}
	return changed
}

// describeFileDiff explains how two sets of file changes differ, or returns an
// empty string when they agree.
func describeFileDiff(want, got map[string]string) string {
	var only, missing, differing []string
	for path, sum := range got {
		other, ok := want[path]
		switch {
		case !ok:
			only = append(only, path)
		case other != sum:
			differing = append(differing, path)
		}
	}
	for path := range want {
		if _, ok := got[path]; !ok {
			missing = append(missing, path)
		}
	}
	sort.Strings(only)
	sort.Strings(missing)
	sort.Strings(differing)

	var parts []string
	if len(differing) > 0 {
		parts = append(parts, "different contents in "+strings.Join(differing, ", "))
	}
	if len(only) > 0 {
		parts = append(parts, "wrote "+strings.Join(only, ", ")+" and the Perl did not")
	}
	if len(missing) > 0 {
		parts = append(parts, "did not write "+strings.Join(missing, ", "))
	}
	return strings.Join(parts, "; ")
}
