package ai

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	dir, err := os.MkdirTemp("", "perl2go-overlay-")
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
