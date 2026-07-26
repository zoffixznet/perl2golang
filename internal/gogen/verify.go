package gogen

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrNoToolchain is returned by Build when there is no go command on PATH.
// Callers test for it with errors.Is to tell "the code compiled" apart from
// "the code was never compiled", which are very different claims to make in a
// report.
var ErrNoToolchain = errors.New("gogen: no go toolchain on PATH, the emitted code was not compiled")

// buildTimeout bounds a verification build, so a wedged toolchain cannot hang
// a conversion.
const buildTimeout = 2 * time.Minute

// Parse reports whether src is syntactically valid Go.
//
// Emitting Go that does not parse is a bug in this tool rather than a problem
// with the input, so the error is deliberately loud: it quotes the offending
// line with its number, which is what turns "generated code is broken" into a
// one-minute fix.
func Parse(name string, src []byte) error {
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution); err != nil {
		return parseError(name, src, err)
	}
	return nil
}

func parseError(name string, src []byte, err error) error {
	var list scanner.ErrorList
	if errors.As(err, &list) && len(list) > 0 {
		first := list[0]
		return fmt.Errorf("emitted Go does not parse\n%s:%d:%d: %s\n%s",
			name, first.Pos.Line, first.Pos.Column, first.Msg,
			quoteLine(src, first.Pos.Line))
	}
	return fmt.Errorf("emitted Go does not parse\n%s: %w", name, err)
}

// quoteLine returns the numbered source line, ready to append to an error.
func quoteLine(src []byte, line int) string {
	lines := strings.Split(string(src), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return "\t" + strconv.Itoa(line) + " | " + strings.TrimRight(lines[line-1], "\r")
}

var toolchain struct {
	once sync.Once
	ok   bool
}

// HaveToolchain reports whether a go command is on PATH. The answer is looked
// up once and reused, because it cannot change usefully within a run.
func HaveToolchain() bool {
	toolchain.once.Do(func() {
		_, err := exec.LookPath("go")
		toolchain.ok = err == nil
	})
	return toolchain.ok
}

// Build compiles a set of files in a scratch module and reports what the
// compiler said. Keys of the map are paths relative to the module root, so
// "main.go" and "internal/run/run.go" both work; a "go.mod" in the map is used
// as is, and one is written when the map has none.
//
// The build is hermetic: the module proxy is off, so this never touches the
// network and a missing third-party dependency fails fast instead of being
// downloaded. When no toolchain is available the work is skipped and
// ErrNoToolchain is returned, so a caller can report "not checked" rather than
// claiming a clean build it never ran.
func Build(files map[string][]byte) error {
	if !HaveToolchain() {
		return ErrNoToolchain
	}
	if len(files) == 0 {
		return nil
	}

	dir, err := os.MkdirTemp("", "perl2go-verify-")
	if err != nil {
		return fmt.Errorf("gogen: creating build directory: %w", err)
	}
	defer os.RemoveAll(dir)

	haveGoMod := false
	for name, src := range files {
		rel := filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("gogen: refusing to write %q outside the build directory", name)
		}
		if rel == "go.mod" {
			haveGoMod = true
		}
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("gogen: creating %s: %w", filepath.Dir(rel), err)
		}
		if err := os.WriteFile(full, src, 0o644); err != nil {
			return fmt.Errorf("gogen: writing %s: %w", rel, err)
		}
	}
	if !haveGoMod {
		mod := "module perl2goverify\n\ngo " + goDirective() + "\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
			return fmt.Errorf("gogen: writing go.mod: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = dir
	// The environment is inherited so that GOCACHE, GOROOT and HOME come
	// along; the overrides that follow win, because os/exec keeps the last
	// value of a repeated variable.
	cmd.Env = append(os.Environ(),
		"GOPROXY=off",
		"GOFLAGS=-mod=mod",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("gogen: go build timed out after %s", buildTimeout)
		}
		return fmt.Errorf("gogen: go build failed: %w\n%s", err, strings.TrimRight(string(out), "\n"))
	}
	return nil
}

// goDirective returns the language version for a generated go.mod. It follows
// the toolchain that is running, so the scratch module never asks for a
// version the local compiler does not have.
func goDirective() string {
	v := strings.TrimPrefix(runtime.Version(), "go")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 2 {
		if _, err := strconv.Atoi(parts[0]); err == nil {
			if _, err := strconv.Atoi(parts[1]); err == nil {
				return parts[0] + "." + parts[1]
			}
		}
	}
	// Go 1.22 is the floor: below it a loop variable is shared between
	// iterations, which silently changes what a closure in a loop means.
	return "1.22"
}
