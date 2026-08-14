package ai

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runtimeDownloadMB is the size of the local runtime's own archive. It carries
// its own graphics libraries, which is why it is large and why a machine with
// only a vendor driver needs nothing else installed.
const runtimeDownloadMB = 1422

// runtimeInstallDir returns where a rootless install should put the runtime:
// under the user's home directory, so it needs no administrator rights, adds no
// system account and installs no drivers. The path is resolved here so that the
// command shown to the user is the literal command that runs.
func runtimeInstallDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "$HOME/.local/ollama"
	}
	return filepath.Join(home, ".local", "ollama")
}

// SetupStep is one command the tool proposes to run, with what it will
// download and whether it needs administrator rights.
type SetupStep struct {
	Description string
	Command     []string
	DownloadMB  int
	NeedsRoot   bool
}

// String renders the command exactly as it will be run.
func (s SetupStep) String() string { return strings.Join(s.Command, " ") }

// SetupPlan is what installing the runtime and fetching a model would involve.
//
// It is deliberately inert: building a plan reads nothing, downloads nothing
// and changes nothing. The caller prints it, obtains consent, and only then
// calls [SetupPlan.Execute].
type SetupPlan struct {
	// Model is the tag the plan would install, empty when there is nothing
	// suitable to install.
	Model string
	// Licence is the model's licence, empty when it is not known.
	Licence string
	Steps   []SetupStep
	// TotalDownloadMB is everything the plan would fetch.
	TotalDownloadMB int
	// DiskNeedMB is the free space the plan needs, which is more than the
	// download because of the unpacking and a margin.
	DiskNeedMB int
	// Warnings are the reasons a user might decline, in their own terms.
	Warnings []string
	// Notes are the things the user has to do that this tool will not do for
	// them, such as starting the runtime.
	Notes []string
}

// Runnable reports whether the plan has anything to run.
func (p SetupPlan) Runnable() bool { return len(p.Steps) > 0 }

// Describe renders the plan for the terminal: what will be downloaded, what
// will be run, and anything the user should know before agreeing.
func (p SetupPlan) Describe() []string {
	var lines []string
	if p.Model != "" {
		licence := p.Licence
		if licence == "" {
			licence = "unknown"
		}
		lines = append(lines, fmt.Sprintf("Model:     %s", p.Model), fmt.Sprintf("Licence:   %s", licence))
	}
	if p.TotalDownloadMB > 0 {
		lines = append(lines, fmt.Sprintf("Download:  %s, once", formatMB(p.TotalDownloadMB)))
	}
	for _, w := range p.Warnings {
		lines = append(lines, "Warning:   "+w)
	}
	for _, n := range p.Notes {
		lines = append(lines, "Note:      "+n)
	}
	if !p.Runnable() {
		lines = append(lines, "There is nothing to run.")
		return lines
	}
	lines = append(lines, "", "These commands will run, and nothing else:")
	for _, s := range p.Steps {
		lines = append(lines, "  $ "+s.String())
	}
	return lines
}

// PlanSetup works out what it would take to run model on the hardware it is
// given.
//
// An empty model means "the best one this hardware can run", taken from
// [Recommend]. The plan installs the runtime under the user's home directory
// when it is missing, because that needs no administrator rights, creates no
// system account and installs no drivers.
func PlanSetup(hw Hardware, model string, runtimeInstalled bool) SetupPlan {
	plan := SetupPlan{Model: model}

	spec, known := Lookup(model)
	switch {
	case model == "":
		fits := Recommend(hw)
		if len(fits) == 0 {
			plan.Warnings = append(plan.Warnings, "no model in the catalogue fits the available memory and free disk")
			return plan
		}
		spec, known = fits[0], true
		plan.Model = spec.Name
	case ExcludedReason(model) != "":
		plan.Warnings = append(plan.Warnings, ExcludedReason(model))
		plan.Warnings = append(plan.Warnings, "this tool does not download models whose licence is not free for every user")
		return plan
	case !known:
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("%s is not in this tool's catalogue, so its size and its licence are unknown until it is downloaded", model))
	}

	if known {
		plan.Licence = spec.Licence
		if !spec.LicenceInModel {
			plan.Notes = append(plan.Notes, "this model carries no licence text, so the runtime will report its licence as unknown")
		}
	}

	installDir := runtimeInstallDir()
	if !runtimeInstalled {
		if missing := missingTools(); len(missing) > 0 {
			plan.Steps = append(plan.Steps, SetupStep{
				Description: "install the tools the runtime's archive needs",
				Command:     append([]string{"sudo", "apt-get", "install", "-y"}, missing...),
				NeedsRoot:   true,
			})
			plan.Warnings = append(plan.Warnings,
				"installing "+strings.Join(missing, " and ")+" needs administrator rights")
		}
		plan.Steps = append(plan.Steps,
			SetupStep{
				Description: "create the runtime directory",
				Command:     []string{"sh", "-c", `mkdir -p "` + installDir + `"`},
			},
			SetupStep{
				Description: "download and unpack the runtime",
				Command: []string{"sh", "-c",
					`curl -fsSL https://ollama.com/download/ollama-linux-amd64.tar.zst | tar --zstd -x -C "` + installDir + `"`},
				DownloadMB: runtimeDownloadMB,
			},
		)
		plan.Notes = append(plan.Notes,
			"add "+installDir+"/bin to PATH, then start the runtime with 'ollama serve' in another terminal; the model download needs it running")
	}

	pull := SetupStep{
		Description: "download the model",
		Command:     []string{"ollama", "pull", plan.Model},
	}
	if known {
		pull.DownloadMB = spec.DownloadMB
	}
	plan.Steps = append(plan.Steps, pull)

	for _, s := range plan.Steps {
		plan.TotalDownloadMB += s.DownloadMB
	}
	plan.DiskNeedMB = plan.TotalDownloadMB*5/4 + 2048

	if hw.DiskFreeMB > 0 && hw.DiskFreeMB < int64(plan.DiskNeedMB) {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf(
			"this needs about %s free on %s, and %s is available",
			formatMB(plan.DiskNeedMB), hw.ModelStorePath, formatMB(int(hw.DiskFreeMB))))
	}

	if known {
		switch vram := hw.BestVRAMMB(); {
		case vram > 0 && !spec.FitsGPU(vram):
			plan.Warnings = append(plan.Warnings, fmt.Sprintf(
				"%s wants about %s of graphics memory and %s is available, so it will run partly on the CPU and be slow",
				spec.Name, formatMB(spec.MinVRAMMB), formatMB(vram)))
		case vram == 0 && !spec.FitsCPU(hw.RAMTotalMB):
			plan.Warnings = append(plan.Warnings, fmt.Sprintf(
				"%s wants about %s of memory to run on the CPU and %s is available",
				spec.Name, formatMB(spec.MinRAMMB), formatMB(hw.RAMTotalMB)))
		case vram == 0:
			plan.Notes = append(plan.Notes, "there is no graphics card to use, so the model runs on the CPU and each file takes noticeably longer")
		}
	}

	return plan
}

// missingTools reports which of the archive's prerequisites are absent. curl
// and tar are usually present; the compressor is not guaranteed.
func missingTools() []string {
	var missing []string
	for _, tool := range []string{"curl", "zstd", "tar"} {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}
	return missing
}

// Execute runs the plan's commands in order, streaming their output to w.
//
// Call this only after the user has seen [SetupPlan.Describe] and agreed to it:
// this package never asks for consent itself, and never runs anything on its
// own. Each command is echoed to w before it runs, so what the user was shown
// and what actually ran cannot drift apart.
func (p SetupPlan) Execute(ctx context.Context, w io.Writer) error {
	if !p.Runnable() {
		return ErrNothingToDo
	}
	if w == nil {
		w = io.Discard
	}
	for i, step := range p.Steps {
		fmt.Fprintf(w, "$ %s\n", step)
		cmd := exec.CommandContext(ctx, step.Command[0], step.Command[1:]...)
		cmd.Stdout = w
		cmd.Stderr = w
		if err := cmd.Run(); err != nil {
			return &SetupError{Step: i, Description: step.Description, Command: step.Command, Err: err}
		}
	}
	return nil
}
