package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"perl2golang/internal/ai"
	"perl2golang/internal/convert"
)

// Everything to do with the optional local model lives here, and nothing in
// this file runs unless the user asked for it. That is the whole of the
// promise in one sentence: with no --ai flag, aiFlags.enabled is false, no
// client is constructed, no improver is passed to the converter, and the
// pipeline never has anything to call.

// aiFlags is the parsed AI part of a convert command line.
type aiFlags struct {
	enabled  bool
	model    string
	endpoint string
	timeout  time.Duration
	jobs     string
}

// register adds the AI flags to a flag set.
func (f *aiFlags) register(fs *flag.FlagSet) {
	fs.BoolVar(&f.enabled, "ai", false, "")
	fs.StringVar(&f.model, "ai-model", "", "")
	fs.StringVar(&f.endpoint, "ai-endpoint", "", "")
	fs.DurationVar(&f.timeout, "ai-timeout", 0, "")
	fs.StringVar(&f.jobs, "ai-jobs", "", "")
}

// used reports whether any AI flag appeared, so a command line that configures
// AI mode without turning it on can be called out rather than silently ignored.
func (f *aiFlags) used() bool {
	return f.enabled || f.model != "" || f.endpoint != "" || f.timeout != 0 || f.jobs != ""
}

// aiSession is a live AI mode for one invocation: the client, the improver the
// converter is given, and whatever has to be said about it afterwards.
type aiSession struct {
	client   *ai.Client
	improver convert.Improver
}

// startAI prepares AI mode, or explains why it is not available and returns
// nil so the run continues deterministically.
//
// Every failure here is a degradation, never an error: a user who asked for a
// better conversion and cannot have one still wants the conversion.
func startAI(e *env, f *aiFlags) (*aiSession, int) {
	if !f.enabled {
		if f.used() {
			fmt.Fprintln(e.stderr, "perl2go: the --ai-* flags only take effect with --ai, which was not given")
		}
		return nil, ExitOK
	}

	jobs, err := ai.ParseJobs(f.jobs)
	if err != nil {
		return nil, e.usagef("%v", err)
	}
	if len(jobs) == 0 {
		fmt.Fprintln(e.stderr, "perl2go: --ai-jobs=none leaves nothing for the model to do, so the conversion is the deterministic one")
		return nil, ExitOK
	}
	for _, job := range jobs.Experimental() {
		fmt.Fprintf(e.stderr, "perl2go: the %s job asks the model to write prose. A model this size writes "+
			"thinner explanations than the built-in lessons and can name a function that does not exist, "+
			"so read what it produces before trusting it.\n", job)
	}

	client := ai.New(ai.Options{
		Endpoint: f.endpoint,
		Model:    f.model,
		Timeout:  f.timeout,
		Jobs:     jobs,
		Progress: e.stderr,
	})

	ctx := context.Background()
	installed, err := client.Available(ctx)
	if err != nil {
		fmt.Fprintf(e.stderr, "perl2go: %s\n", degradeMessage(err, client.Endpoint()))
		return nil, ExitOK
	}
	if len(installed) == 0 {
		fmt.Fprintf(e.stderr, "perl2go: the runtime at %s has no models, so the conversion is the deterministic one.\n"+
			"Run `perl2go ai setup` to see what this machine can run.\n", client.Endpoint())
		return nil, ExitOK
	}

	model := f.model
	if model == "" {
		model = ai.PreferredModel(installed)
		client = ai.New(ai.Options{
			Endpoint: f.endpoint,
			Model:    model,
			Timeout:  f.timeout,
			Jobs:     jobs,
			Progress: e.stderr,
		})
	} else if !slices.Contains(installed, model) && !slices.Contains(installed, model+":latest") {
		fmt.Fprintf(e.stderr, "perl2go: the runtime at %s does not have %s, so the conversion is the deterministic one.\n"+
			"It has: %s\n", client.Endpoint(), model, strings.Join(installed, ", "))
		return nil, ExitOK
	}

	fmt.Fprintf(e.stderr, "perl2go: ai mode on, using %s at %s for %s\n", model, client.Endpoint(), jobs)
	return &aiSession{client: client, improver: ai.NewImprover(client)}, ExitOK
}

// finish reports what the model did and lets the runtime have its memory back.
func (s *aiSession) finish(w io.Writer, verbose bool) {
	if s == nil {
		return
	}
	summary := s.client.Summary()
	fmt.Fprintln(w, "  "+summary.Headline())
	if verbose {
		for _, warning := range summary.Warnings {
			fmt.Fprintf(w, "  ai: %s\n", warning)
		}
		for _, change := range summary.Changes {
			fmt.Fprintf(w, "  ai: %s %s\n", change.Job, change.Detail)
		}
		if summary.Calls > 0 {
			fmt.Fprintf(w, "  ai: %d call(s), %d tokens out, %.0f tok/s\n",
				summary.Calls, summary.OutputTokens, summary.TokensPerSecond())
		}
	}
	// Asking the runtime to drop the model returns the memory to the machine.
	// It is best effort by design: a failure here changes nothing about the
	// conversion that was just written.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.Unload(ctx)
}

// degradeMessage turns a client failure into one sentence a user can act on.
func degradeMessage(err error, endpoint string) string {
	var unavailable *ai.UnavailableError
	if errors.As(err, &unavailable) {
		return fmt.Sprintf("no inference runtime answered at %s, so the conversion is the deterministic one.\n"+
			"Start one, or run `perl2go ai setup` to see what this machine can run.", endpoint)
	}
	var timeout *ai.TimeoutError
	if errors.As(err, &timeout) {
		return fmt.Sprintf("the runtime at %s did not answer in time, so the conversion is the deterministic one.", endpoint)
	}
	return fmt.Sprintf("AI mode is not available (%v), so the conversion is the deterministic one.", err)
}

// runAI is the `perl2go ai` command: what is configured, and what this machine
// could run.
func runAI(e *env, args []string) int {
	if len(args) == 0 {
		return e.print(aiHelp())
	}
	switch args[0] {
	case "status":
		return aiStatus(e, args[1:])
	case "setup":
		return aiSetup(e, args[1:])
	case "help", "-h", "--help":
		return e.print(aiHelp())
	}
	return e.usagef("there is no ai command called %q. The ai commands are status and setup", args[0])
}

// aiStatus reports what is there, without changing anything.
func aiStatus(e *env, args []string) int {
	fs := flag.NewFlagSet("ai status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var endpoint string
	fs.StringVar(&endpoint, "ai-endpoint", "", "")
	if err := fs.Parse(args); err != nil {
		return e.usagef("%v", err)
	}

	client := ai.New(ai.Options{Endpoint: endpoint})
	fmt.Fprintf(e.stdout, "endpoint    %s\n", client.Endpoint())
	fmt.Fprintf(e.stdout, "model store %s\n", ai.DefaultModelStore())
	fmt.Fprintf(e.stdout, "jobs        %s (default under --ai)\n", ai.DefaultJobs())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	installed, err := client.Installed(ctx)
	if err != nil {
		fmt.Fprintf(e.stdout, "runtime     not answering: %v\n", err)
	} else {
		fmt.Fprintf(e.stdout, "runtime     answering, %d model(s) installed\n", len(installed))
		for _, m := range installed {
			fmt.Fprintf(e.stdout, "  %-28s %6d MB  %s %s\n", m.Name, m.SizeMB, m.ParameterSize, m.Quantization)
		}
	}

	hw, err := ai.Probe(ai.DefaultModelStore())
	if err == nil {
		fmt.Fprintln(e.stdout)
		for _, line := range hw.Describe() {
			fmt.Fprintln(e.stdout, line)
		}
		if rec := ai.Recommend(hw); len(rec) > 0 {
			fmt.Fprintln(e.stdout, "\nmodels this machine can run:")
			for _, m := range rec {
				fmt.Fprintf(e.stdout, "  %-28s %8s  %s\n", m.Name, m.DownloadSize(), m.Licence)
			}
		}
	}

	fmt.Fprintln(e.stdout, "\nNothing above was downloaded or changed. Models are shared with the")
	fmt.Fprintln(e.stdout, "rest of the machine, and perl2go never removes or moves one.")
	return ExitOK
}

// aiSetup prints the exact commands it would run and stops, unless the user
// says otherwise. Downloading gigabytes is never a side effect of a flag.
func aiSetup(e *env, args []string) int {
	fs := flag.NewFlagSet("ai setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		model    string
		yes      bool
		endpoint string
	)
	fs.StringVar(&model, "ai-model", "", "")
	fs.StringVar(&endpoint, "ai-endpoint", "", "")
	fs.BoolVar(&yes, "yes", false, "")
	if err := fs.Parse(args); err != nil {
		return e.usagef("%v", err)
	}

	hw, err := ai.Probe(ai.DefaultModelStore())
	if err != nil {
		fmt.Fprintf(e.stderr, "perl2go: could not inspect this machine: %v\n", err)
	}
	for _, line := range hw.Describe() {
		fmt.Fprintln(e.stdout, line)
	}

	client := ai.New(ai.Options{Endpoint: endpoint})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	installed, listErr := client.Installed(ctx)
	runtimeUp := listErr == nil

	if runtimeUp && len(installed) > 0 {
		fmt.Fprintln(e.stdout, "\nthe runtime already has:")
		for _, m := range installed {
			fmt.Fprintf(e.stdout, "  %-28s %6d MB\n", m.Name, m.SizeMB)
		}
		if model == "" {
			fmt.Fprintf(e.stdout, "\nNothing to do. `perl2go convert file.pl --ai` will use %s.\n",
				ai.PreferredModel(namesOf(installed)))
			return ExitOK
		}
	}

	if model == "" {
		rec := ai.Recommend(hw)
		if len(rec) == 0 {
			fmt.Fprintln(e.stderr, "perl2go: no model in the catalogue fits this machine, and nothing was downloaded")
			return ExitOK
		}
		model = rec[0].Name
		fmt.Fprintln(e.stdout, "\nmodels this machine can run, best first:")
		for _, m := range rec {
			fmt.Fprintf(e.stdout, "  %-28s %8s  %s\n    %s\n", m.Name, m.DownloadSize(), m.Licence, m.Notes)
		}
	}

	plan := ai.PlanSetup(hw, model, runtimeUp)
	if !plan.Runnable() {
		fmt.Fprintln(e.stdout, "\nNothing to install.")
		return ExitOK
	}
	fmt.Fprintln(e.stdout)
	for _, line := range plan.Describe() {
		fmt.Fprintln(e.stdout, line)
	}
	if !yes {
		fmt.Fprintln(e.stdout, "\nNothing has been downloaded. Run the commands above yourself, or")
		fmt.Fprintln(e.stdout, "re-run this with --yes to have perl2go run them.")
		return ExitOK
	}
	if err := plan.Execute(context.Background(), e.stderr); err != nil {
		fmt.Fprintf(e.stderr, "perl2go: %v\n", err)
		return ExitFailed
	}
	return ExitOK
}

func namesOf(models []ai.InstalledModel) []string {
	out := make([]string, len(models))
	for i, m := range models {
		out[i] = m.Name
	}
	return out
}

// aiHelp is `perl2go ai --help`.
func aiHelp() string {
	return dedent(`
	inspect and set up the optional local model.

	usage:
	  perl2go ai status              what is configured and what is installed
	  perl2go ai setup [--yes]       what this machine can run, and how to get it

	flags:
	      --ai-endpoint URL   the runtime to talk to (default: $OLLAMA_HOST, or
	                          http://localhost:11434)
	      --ai-model TAG      the model to set up
	      --yes               run the printed commands instead of only printing them

	AI mode is optional in the strongest sense: with no --ai flag perl2go opens no
	socket at all. When it is on, it talks to a runtime you already have, uses a
	model you already have where possible, and never removes, moves or copies one.
	Models are shared with the rest of the machine.

	environment:
	  OLLAMA_HOST      the runtime endpoint, honoured as the runtime's own tools do
	  OLLAMA_MODELS    the model store, reported by status and never overridden

	See perl2go convert --help for the flags that turn it on during a conversion.
	`)
}

// aiConvertHelp is the AI section of the convert help, kept here so the flags
// and their description sit in one file.
func aiConvertHelp() string {
	return dedent(`
	optional local model (off by default):
	      --ai             let a local model improve the names in the generated Go.
	                       Without this flag perl2go makes no network connection.
	      --ai-model TAG   which model to use (default: a code model the runtime
	                       already has)
	      --ai-endpoint U  the runtime's base URL (default: $OLLAMA_HOST, or
	                       http://localhost:11434)
	      --ai-timeout D   how long one request may take (default: 2m)
	      --ai-jobs LIST   which jobs to run. Default: rename,shapes,comments, the
	                       three that only ever produce names. Also accepts idioms,
	                       walkthrough, hints, the group names code and docs, all,
	                       and none.

	Anything the model produces has to parse, compile and pass go vet alongside the
	rest of its package before it is used. When it does not, the deterministic
	output is written and the report says what was turned down.
	`)
}
