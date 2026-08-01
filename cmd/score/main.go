// Command score measures the conversion against the corpus.
//
// It converts every Perl program under testdata/corpus, compiles what comes
// out, runs it beside real perl, and prints how far each entry got. The result
// is written to a file so two runs can be compared, which is what makes
// "the conversion got better" a claim with a number behind it.
//
// Run it with `make score`.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"perl2go/internal/score"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "score: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		tier    = flag.String("tier", "", "score one tier only (tier1, tier2, tier3, tier4, domain)")
		only    = flag.String("only", "", "score only entries whose name contains this substring")
		asJSON  = flag.Bool("json", false, "print the scorecard as JSON instead of a table")
		out     = flag.String("out", "", "where to write the results file (default testdata/scorecard.json)")
		short   = flag.Bool("short", false, "skip the equivalence stage, which is the slow one")
		verbose = flag.Bool("v", false, "print a line per entry and the full failure list")
		jobs    = flag.Int("jobs", runtime.NumCPU(), "how many entries to work on at once")
		timeout = flag.Duration("timeout", score.DefaultTimeout, "how long one program may run before it is killed")
		corpus  = flag.String("corpus", "", "corpus manifest to read, listing entry paths relative to the repository root (default testdata/corpus/MANIFEST.json)")
		noWrite = flag.Bool("no-write", false, "do not write the results file")
	)
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q; see -h", flag.Arg(0))
	}

	root, err := score.FindRoot("")
	if err != nil {
		return err
	}
	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(root, "testdata", "scorecard.json")
	}

	previous, err := score.LoadScorecard(outPath)
	if err != nil {
		return err
	}

	// Ctrl-C stops the run rather than leaving processes behind: every
	// program the scorecard starts is tied to this context.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := score.Options{
		Root:     root,
		Manifest: *corpus,
		Tier:     *tier,
		Only:     *only,
		Short:    *short,
		Jobs:     *jobs,
		Timeout:  *timeout,
	}
	if !*asJSON {
		opts.Progress = progress(os.Stderr)
	}

	sc, err := score.Run(ctx, opts)
	if err != nil {
		return err
	}
	delta := sc.Compare(previous)

	if *asJSON {
		if err := score.RenderJSON(os.Stdout, sc, delta); err != nil {
			return err
		}
	} else {
		score.Render(os.Stdout, sc, delta, score.RenderOptions{Verbose: *verbose})
	}

	if *noWrite {
		return nil
	}
	// A run of one tier says nothing about the rest of the corpus, so it must
	// not become the record the next full run is compared against. Writing it
	// somewhere else is still allowed, because that file was asked for by name.
	if sc.Filter.Partial() && *out == "" {
		if !*asJSON {
			fmt.Printf("this run covered %s, so %s was left alone\n",
				sc.Filter.Describe(), display(root, outPath))
		}
		return nil
	}
	written, err := sc.Save(outPath)
	if err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}
	if !*asJSON {
		if written {
			fmt.Printf("results written to %s\n", display(root, outPath))
		} else {
			fmt.Printf("%s already holds these results and was left alone\n", display(root, outPath))
		}
	}
	return nil
}

// display names a path the shortest way that is still unambiguous: relative to
// the repository when it lives inside it, and in full when it does not.
func display(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}

// progress prints a one-line counter while the run works through the corpus. It
// goes to stderr so redirecting the table to a file keeps the table clean, and
// it is skipped entirely when stderr is not a terminal.
func progress(f *os.File) func(done, total int, r score.EntryResult) {
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return nil
	}
	last := time.Now()
	return func(done, total int, r score.EntryResult) {
		if done != total && time.Since(last) < 200*time.Millisecond {
			return
		}
		last = time.Now()
		fmt.Fprintf(f, "\r\033[K  %d/%d  %s", done, total, r.ID())
		if done == total {
			fmt.Fprint(f, "\r\033[K")
		}
	}
}

func usage() {
	fmt.Fprint(flag.CommandLine.Output(), `score - measure the conversion against the corpus

Usage: score [flags]

Converts every corpus program, compiles the result, runs it beside real perl,
and prints how far each entry got. The numbers are written to a results file so
the next run can be compared against this one.

Flags:
`)
	flag.PrintDefaults()
}
