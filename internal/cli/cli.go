// Package cli is the perl2go command line.
//
// [Run] is the whole program: package main hands it the arguments and the
// three streams and turns what it returns into the process exit status.
// Keeping the behaviour here rather than in main means the terminal output can
// be tested in process, without building a binary or spawning anything.
//
// Two rules shape everything below. Standard output carries the artifact, so
// it stays pipeable and never receives a note or a progress line; standard
// error carries the commentary. And the tool never runs the input: converting
// makes no network connection and spawns no interpreter.
package cli

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"perl2golang/internal/convert"
	"perl2golang/internal/diag"
	"perl2golang/internal/report"
)

// Exit statuses. These four are the complete set, they are documented in the
// root help text, and nothing else ever comes back from [Run].
const (
	// ExitOK means the conversion finished and the output was written.
	// Warnings and refusals are reported rather than treated as failures,
	// because almost every real conversion produces some and a tool whose
	// normal outcome is a nonzero status teaches people to ignore statuses.
	// --strict is there for callers who want the opposite.
	ExitOK = 0
	// ExitStrict means --strict was given and the run produced warnings or
	// refusals. The output was still written, because a CI gate that also
	// destroys the artifact makes the failure harder to diagnose.
	ExitStrict = 1
	// ExitFailed means the conversion did not finish and nothing was written.
	ExitFailed = 2
	// ExitUsage means the command line could not be understood.
	ExitUsage = 3
)

// Run executes one perl2go invocation and returns the exit status.
//
// args are the arguments after the program name. stdin is only read when an
// input path is "-". Nothing is written to stdout except the artifact the
// user asked for: the terminal summary moves to stderr whenever stdout is
// carrying converted code or JSON.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	e := &env{
		stdin: stdin, stdout: stdout, stderr: stderr, argv: args,
		// A wrapper that asked for JSON gets JSON even when its command line
		// was wrong, so it never has to parse two formats.
		jsonMode: hasFlag(args, "json"),
	}
	// A panic anywhere below is a defect in perl2go. The user gets a message
	// naming what to report rather than a stack trace, and the status says the
	// run failed, which is what it did.
	defer func() {
		if r := recover(); r != nil {
			code = e.internalError(r)
		}
	}()
	return dispatch(e, args)
}

// env carries the streams and the original arguments through the commands, so
// that no code below reaches for os.Stdout and every test can watch what was
// written.
type env struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	// argv is the argument list as given, echoed in the --json envelope so a
	// stored object records the run that produced it.
	argv []string
	// jsonMode records that --json appeared anywhere on the command line, so
	// that even a usage error is reported as one JSON object.
	jsonMode bool
}

// commands is the closed set of first words that name a subcommand. Anything
// else in first position is a file for convert, which is what makes
// `perl2go script.pl` shorthand for `perl2go convert script.pl`.
var commands = []string{"convert", "repl", "explain", "ai", "version", "help"}

// dispatch routes one invocation to its command.
func dispatch(e *env, args []string) int {
	name, rest, explicit := splitCommand(args)

	// -h anywhere means help, and which help depends on whether the user
	// named a command. `perl2go --help` is the tour; `perl2go convert --help`
	// is the flag list.
	if hasFlag(rest, "h", "help") {
		if !explicit {
			return e.print(rootHelp())
		}
		return e.commandHelp(name)
	}
	if !explicit && hasFlag(rest, "version") {
		return e.print(versionLine() + "\n")
	}
	// --repl is the flag spelling of the repl command, for people who reach
	// for a flag before a subcommand.
	if !explicit && hasFlag(rest, "repl") {
		return runRepl(e, rest)
	}

	switch name {
	case "convert":
		return runConvert(e, rest)
	case "repl":
		return runRepl(e, rest)
	case "explain":
		return runExplain(e, rest)
	case "ai":
		return runAI(e, rest)
	case "version":
		return runVersion(e, rest)
	case "help":
		if len(rest) == 0 {
			return e.print(rootHelp())
		}
		if !slices.Contains(commands, rest[0]) {
			return e.usagef("there is no command called %q. The commands are %s",
				rest[0], strings.Join(commands, ", "))
		}
		return e.commandHelp(rest[0])
	}
	// splitCommand only ever returns a name from commands.
	panic("cli: unrouted command " + name)
}

// splitCommand decides which command an argument list names.
//
// Only the first argument counts, so `perl2go convert version.pl` converts a
// file called version.pl rather than looking for a command in the middle of
// the line. A Perl file whose name collides with a command word is reachable
// as `perl2go convert explain` or as `./explain`.
func splitCommand(args []string) (name string, rest []string, explicit bool) {
	if len(args) > 0 && slices.Contains(commands, args[0]) {
		return args[0], args[1:], true
	}
	return "convert", args, false
}

// hasFlag reports whether args contain any of the named flags, in either the
// one-dash or the two-dash spelling. Scanning stops at "--", after which
// everything is a positional argument.
func hasFlag(args []string, names ...string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if !strings.HasPrefix(a, "-") {
			continue
		}
		bare := strings.TrimLeft(a, "-")
		bare, _, _ = strings.Cut(bare, "=")
		if slices.Contains(names, bare) {
			return true
		}
	}
	return false
}

// commandHelp prints one subcommand's help to stdout.
func (e *env) commandHelp(name string) int {
	switch name {
	case "repl":
		return e.print(replHelp())
	case "explain":
		return e.print(explainHelp())
	case "ai":
		return e.print(aiHelp())
	case "version":
		return e.print(versionHelp())
	case "help":
		return e.print(rootHelp())
	default:
		return e.print(convertHelp())
	}
}

// print writes finished text to stdout. Help and version output are the
// artifact of the commands that produce them, so they belong on stdout.
func (e *env) print(s string) int {
	fmt.Fprint(e.stdout, s)
	return ExitOK
}

// usage reports a command line that could not be understood, and points at
// the help that explains it.
func (e *env) usage(entry report.Entry) int {
	if e.jsonMode {
		return usageJSON(e, entry.Code+": "+entry.Message)
	}
	e.diagnose(entry, "command line", nil, diag.ColorEnabled(e.stderr, ""))
	return e.helpHint()
}

// usagef reports a command line problem that has no registry code, such as a
// flag the standard flag package rejected.
func (e *env) usagef(format string, args ...any) int {
	message := fmt.Sprintf(format, args...)
	if e.jsonMode {
		return usageJSON(e, message)
	}
	fmt.Fprintf(e.stderr, "perl2go: %s\n", message)
	return e.helpHint()
}

// helpHint closes a usage error by naming the help that would have prevented
// it.
func (e *env) helpHint() int {
	fmt.Fprintln(e.stderr, "run `perl2go --help` for the flags, or `perl2go convert --help` for the full list")
	return ExitUsage
}

// diagnose renders one diagnostic in full block form on stderr. srcLines may
// be nil, in which case the excerpt is left out.
func (e *env) diagnose(entry report.Entry, file string, srcLines []string, color bool) {
	_ = diag.Render(e.stderr, entry, file, srcLines, diag.Options{Color: color})
}

// internalError turns a recovered panic into something a user can act on. It
// names the tool, says the failure is a bug in it, and repeats the command
// line so a report can be reproduced. It deliberately prints no stack trace:
// the person who ran this did not write it.
func (e *env) internalError(r any) int {
	fmt.Fprintf(e.stderr, "perl2go: internal error: %v\n", r)
	fmt.Fprintf(e.stderr, "This is a bug in perl2go %s, not a problem with the input.\n", convert.Version)
	fmt.Fprintf(e.stderr, "Please report it with the input file and this command line:\n")
	fmt.Fprintf(e.stderr, "  perl2go %s\n", strings.Join(e.argv, " "))
	return ExitFailed
}

// versionLine is the one-line version banner, with enough build detail to
// identify a binary in a bug report.
func versionLine() string {
	parts := []string{runtime.Version(), runtime.GOOS + "/" + runtime.GOARCH}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				parts = append([]string{s.Value[:7]}, parts...)
			}
		}
	}
	return fmt.Sprintf("perl2go %s (%s)", convert.Version, strings.Join(parts, ", "))
}
