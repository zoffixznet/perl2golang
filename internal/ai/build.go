package ai

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// VerifyBuild type-checks candidate files against a real module without
// touching the working tree.
//
// replace maps an existing file in the module to a file holding the candidate
// text. The toolchain reads the candidates through an overlay, so the module on
// disk is left exactly as it was whatever the outcome. Module downloading is
// switched off, so an import the model invented fails offline in milliseconds
// instead of reaching out for a package that does not exist.
//
// It returns nil when the toolchain is not installed: the caller still has the
// parse and structure checks, which need no toolchain at all.
func VerifyBuild(ctx context.Context, moduleDir string, replace map[string]string) error {
	if len(replace) == 0 {
		return nil
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return nil
	}

	dir, err := os.MkdirTemp("", "perl2golang-overlay-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	abs := make(map[string]string, len(replace))
	for from, to := range replace {
		fromAbs, err := filepath.Abs(from)
		if err != nil {
			return err
		}
		toAbs, err := filepath.Abs(to)
		if err != nil {
			return err
		}
		abs[fromAbs] = toAbs
	}
	overlay, err := json.Marshal(struct {
		Replace map[string]string
	}{Replace: abs})
	if err != nil {
		return err
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, goBin, "build", "-overlay", overlayPath, "./...")
	cmd.Dir = moduleDir
	cmd.Env = append(os.Environ(),
		"GOPROXY=off", // an invented import fails here, offline and instantly
		"GOFLAGS=",    // no inherited flags that could change the outcome
		"GOWORK=off",  // the module under test, not whatever workspace is around
		"CGO_ENABLED=0",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		// The toolchain could not be run at all. That is not the model's
		// fault, and the cheaper checks still stand.
		return nil
	}
	return &RejectedError{Gate: "compile", Reason: firstCompileError(string(out))}
}

// gateTimeout bounds one verification build, so a wedged toolchain cannot hang
// a conversion that would otherwise have finished without the model.
const gateTimeout = 2 * time.Minute

// VerifyPackage compiles and vets a whole package in a scratch module.
//
// This is the gate a model-touched file has to pass. Parsing proves the file is
// Go; only a compile proves that the names in it resolve, and only vet catches
// the class of mistake that compiles and is still wrong, a printf verb that
// does not match its argument being the one that matters most here.
//
// Keys of files are paths relative to the module root. A "go.mod" among them is
// used as is. The build is hermetic: the module proxy is off, so nothing
// reaches the network and an import that does not exist fails in milliseconds.
// With no toolchain on PATH the check is skipped and nil is returned, because
// the caller still has the parse and structure gates and a machine without Go
// should still get whatever the model could offer.
func VerifyPackage(ctx context.Context, files map[string][]byte) error {
	if len(files) == 0 {
		return nil
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return nil
	}

	dir, err := os.MkdirTemp("", "perl2golang-gate-")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(dir)

	haveGoMod := false
	for name, src := range files {
		rel := filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return &RejectedError{Gate: "compile", Reason: "a file in the candidate package escapes the module root"}
		}
		if rel == "go.mod" {
			haveGoMod = true
		}
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil
		}
		if err := os.WriteFile(full, src, 0o600); err != nil {
			return nil
		}
	}
	if !haveGoMod {
		mod := "module perl2golang-gate\n\ngo " + goDirective() + "\n"
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o600); err != nil {
			return nil
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, gateTimeout)
	defer cancel()

	for _, step := range []struct {
		name string
		args []string
	}{
		{"compile", []string{"build", "./..."}},
		{"vet", []string{"vet", "./..."}},
	} {
		cmd := exec.CommandContext(runCtx, goBin, step.args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GOPROXY=off",
			"GOFLAGS=-mod=mod",
			"GOTOOLCHAIN=local",
			"GOWORK=off",
			"CGO_ENABLED=0",
		)
		out, err := cmd.CombinedOutput()
		if err == nil {
			continue
		}
		if runCtx.Err() != nil {
			return &RejectedError{Gate: step.name, Reason: "the toolchain did not finish within " + gateTimeout.String()}
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			// The toolchain could not be run at all. That is not the model's
			// fault, and the cheaper gates still stand.
			return nil
		}
		return &RejectedError{Gate: step.name, Reason: firstCompileError(string(out))}
	}
	return nil
}

// goDirective returns the language version for the scratch module's go.mod, so
// it never asks for a toolchain newer than the one running.
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
	return "1.22"
}

// firstCompileError picks the first useful line out of a build failure.
func firstCompileError(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 200 {
			line = line[:197] + "..."
		}
		return line
	}
	return "the result did not compile"
}
