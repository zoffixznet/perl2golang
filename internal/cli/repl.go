package cli

import (
	"errors"
	"flag"
	"io"
	"os"

	"perl2golang/internal/diag"
	"perl2golang/internal/repl"
)

// runRepl starts the interactive session.
//
// The session writes its whole transcript to stdout, prompts and echoed input
// included, so that `perl2golang repl < session.pl > transcript.txt` produces
// something a person reads top to bottom and a test diffs. Only a failure that
// prevents the session starting goes to stderr.
func runRepl(e *env, args []string) int {
	var (
		mode      string
		noNotes   bool
		noHistory bool
		history   string
		load      string
		color     string
	)
	fs := flag.NewFlagSet("repl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&mode, "mode", "clean", "")
	fs.BoolVar(&noNotes, "no-notes", false, "")
	fs.BoolVar(&noHistory, "no-history", false, "")
	fs.StringVar(&history, "history", "", "")
	fs.StringVar(&load, "load", "", "")
	fs.StringVar(&color, "color", "auto", "")
	// --repl is how the command is reached from the flag side, so it is
	// accepted and ignored here.
	fs.Bool("repl", false, "")

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return e.print(replHelp())
		}
		return e.usagef("%v", err)
	}
	if len(positional) > 0 {
		return e.usagef("repl takes no arguments, got %q. To convert a file, "+
			"run `perl2golang convert %s`", positional[0], positional[0])
	}
	if mode != "clean" && mode != "annotated" {
		return e.usagef("--mode takes clean or annotated, not %q", mode)
	}

	return repl.Run(repl.Options{
		Stdin:  e.stdin,
		Stdout: e.stdout,
		Stderr: e.stderr,
		// Colour follows stdout, which is where the transcript goes, and
		// NO_COLOR overrules everything. A redirected session never receives
		// an escape sequence.
		Color: diag.ColorEnabled(e.stdout, color),
		// Line editing needs a terminal on both ends: something to read keys
		// from and something to redraw.
		Interactive: isCharDevice(e.stdin) && isCharDevice(e.stdout),
		Mode:        mode,
		Notes:       !noNotes,
		NoHistory:   noHistory,
		HistoryFile: history,
		Load:        load,
	})
}

// isCharDevice reports whether a stream is a terminal. It is the same test the
// diagnostic renderer uses to decide about colour, applied to whichever end of
// the session is being asked about.
func isCharDevice(s any) bool {
	f, ok := s.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
