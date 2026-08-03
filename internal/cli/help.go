package cli

import "strings"

// The help text is written out here rather than assembled from the flag
// package's defaults. Grouped flags with real examples are worth more than a
// list in registration order, and the groups answer, in order, what goes in,
// what comes out, how faithful the result is, and how loud the run is.

// rootHelp is `perl2golang --help`.
func rootHelp() string {
	return dedent(`
	perl2golang converts Perl 5 programs into Go, and explains the Go it produces.

	usage:
	  perl2golang [flags] <file.pl>...     convert files (convert is the default)
	  perl2golang <command> [flags] [args]

	commands:
	  convert    convert Perl to Go; the default, so the word can be left out
	  repl       type Perl, see the Go it becomes, one snippet at a time
	  explain    print a teaching concept, or look up a diagnostic code
	  ai         inspect and set up the optional local model
	  version    print the version and build information
	  help       help for perl2golang, or for any command

	common flags:
	  -o, --out DIR      write the generated project here (default: <basename>-go)
	  -e, --expr CODE    convert a snippet given on the command line
	      --stdout       write the artifacts to standard output instead of to files
	      --json         write one machine-readable object to standard output
	      --strict       exit nonzero when anything needs review
	      --force        overwrite an output directory that already has files in it
	  -v, --verbose      show every diagnostic in full, with its source
	      --color WHEN   auto, always, never (default: auto; NO_COLOR is honoured)
	      --ai           let a local model improve the names in the generated Go
	  -h, --help         help for perl2golang, or for any command
	      --version      print the version and exit

	examples:
	  perl2golang report.pl                         convert one file into report-go/
	  perl2golang lib/*.pl -o build/                several files, a directory each
	  perl2golang -e 'print join ",", 1..5'         convert a snippet, print the Go
	  cat old.pl | perl2golang -                    read Perl from standard input
	  perl2golang repl                              explore Perl to Go interactively
	  perl2golang explain slice-aliasing-and-copy   read one teaching concept
	  perl2golang explain P2G4004                   what a diagnostic code means
	  perl2golang report.pl --ai                    let a local model name things
	  perl2golang ai status                         what the optional model needs

	exit status:
	  0  the conversion finished; warnings and refusals are reported, not failures
	  1  --strict was given and the run produced warnings or refusals
	  2  the conversion failed and nothing was written
	  3  usage error

	Conversion never runs your Perl. No subprocess is spawned over your input, and
	without --ai no network connection is made at all, so converting a file you
	have not read is safe.

	Full flag list: perl2golang convert --help
	`)
}

// convertHelp is `perl2golang convert --help`.
func convertHelp() string {
	return dedent(`
	convert Perl 5 source into a Go project, with the reasoning written out.

	usage:
	  perl2golang convert [flags] <file.pl>...
	  perl2golang convert [flags] -e 'CODE'
	  perl2golang convert [flags] -              read Perl from standard input

	For each input perl2golang writes a directory holding the program, the same
	program annotated with what every construct means and where it came from, the
	teaching documents for the constructs this file actually used, and a
	conversion report that says plainly what did not convert cleanly.

	input:
	  -e, --expr CODE      convert this snippet instead of a file; implies --stdout
	                       and skips the teaching documents
	  -                    an input path of a single dash reads standard input, and
	                       implies --stdout the same way

	output:
	  -o, --out DIR        write the generated project here
	                       (default: <basename>-go; with several files DIR is the
	                       parent and each file gets DIR/<basename>-go)
	      --force          overwrite an output directory that already has files in it
	      --stdout         write every artifact to standard output, framed so a
	                       person can read it and a script can split it
	      --stdout=bare    write only the converted Go, with no framing at all;
	                       this is what -e and standard input do by default
	      --stdout=framed  force the framed stream even for -e and standard input
	      --json           write one JSON object holding every artifact and the
	                       full report; implies --color=never. The files are
	                       still written as well, unless the input is a snippet
	                       or standard input.

	conversion:
	      --strict         treat anything that needs review as a failure. The output
	                       is still written, and the exit status becomes 1.

	optional local model (off by default):
	      --ai             let a local model choose better names in the generated
	                       Go. Without this flag perl2golang opens no socket at all.
	      --ai-model TAG   which model to use (default: a code model the runtime
	                       already has; nothing is ever downloaded to convert)
	      --ai-endpoint U  the runtime's base URL (default: $OLLAMA_HOST, or
	                       http://localhost:11434)
	      --ai-timeout D   how long one request may take (default: 2m)
	      --ai-jobs LIST   which jobs to run. Default rename,shapes,comments: the
	                       three that only ever produce names. Also accepts idioms,
	                       walkthrough, the group names code and docs, all,
	                       and none. The idioms and walkthrough jobs are
	                       experimental and say so when you turn them on.

	                       Anything the model produces has to parse, compile and
	                       pass go vet alongside the rest of its package before it
	                       is used. When it does not, the deterministic output is
	                       written and the report says what was turned down.

	output control:
	  -v, --verbose        show every diagnostic in full, with its source line and
	                       carets, instead of the three-line summary
	      --color WHEN     auto, always, never (default: auto). Colour is only ever
	                       written to a terminal, never into a pipe.

	examples:
	  perl2golang convert report.pl
	      writes report-go/ with the program, the annotated program, and docs/

	  perl2golang convert report.pl -o /tmp/out --force
	      the same bundle, somewhere else, over whatever was there

	  perl2golang convert lib/*.pl -o build/
	      convert a set of files, each into its own directory under build/

	  perl2golang convert report.pl --strict
	      for CI: fail the build when anything needs a human

	  perl2golang convert -e 'print "$_\n" for sort keys %ENV' > snip.go
	      print the Go for a snippet, with the notes on standard error

	  perl2golang convert report.pl --json | jq -r '.conversions[0].report.stats'
	      read the numbers from a script

	  git show HEAD:tools/old.pl | perl2golang convert -
	      convert something that is not on disk

	  perl2golang convert report.pl --ai
	      the same conversion, with a local model naming what the converter had to
	      invent names for

	environment:
	  NO_COLOR             set to anything to turn colour off
	  TERM=dumb            same effect
	  OLLAMA_HOST          the local runtime --ai talks to
	  OLLAMA_MODELS        the shared model store, read but never overridden

	Conversion never runs your Perl: no subprocess is spawned over your input, and
	without --ai no network connection is made.

	exit status: see perl2golang --help
	`)
}

// replHelp is `perl2golang repl --help`.
func replHelp() string {
	return dedent(`
	type Perl, see the Go it becomes.

	usage:
	  perl2golang repl [flags]
	  perl2golang --repl [flags]
	  perl2golang repl < session.pl        replay a file as though it were typed

	Each snippet is converted the moment it parses, so a snippet that spans lines
	needs no continuation marker: the prompt keeps reading until the snippet is
	complete. Declarations stay in scope for the rest of the session, so a program
	can be built a line at a time. The Go shown is the Go a file conversion would
	produce, and the concepts named under it are the same teaching concepts the
	generated documents are built from.

	Your Perl is never executed here either. It is read, converted and explained.

	flags:
	      --mode MODE      clean or annotated Go (default: clean); :mode switches it
	      --no-notes       do not print the concept line after each snippet
	      --no-history     do not read or write the history file
	      --history FILE   history file ($XDG_STATE_HOME/perl2golang/history)
	      --load FILE      type the lines of FILE in before handing over the prompt
	      --color WHEN     auto, always, never (default: auto; NO_COLOR is honoured)

	meta commands (:help inside the session prints this list):
	  :help            the list
	  :go [full]       reprint the last Go, or the whole session ready to paste
	  :explain [WHAT]  expand a concept, a P2G code, or the last snippet's concepts
	  :concepts        every concept this session has touched, with its title
	  :why             the converter's reasoning for the last snippet
	  :diag            the last snippet's diagnostics in full, with source
	  :vars            the variables in scope and the Go type inferred for each
	  :perl            the Perl the session holds so far
	  :mode            switch between clean and annotated output
	  :notes on|off    show or hide the concept line
	  :save FILE       write the session's Perl to a file
	  :load FILE       type the lines of a file into the session
	  :reset           forget the session program and start again
	  :clear           clear the screen, keeping the session
	  :quit            leave; :q and Ctrl-D do the same

	keys, when the session has a terminal on both ends:
	  left/right, Ctrl-A, Ctrl-E    move by character, to the start, to the end
	  Alt-B, Alt-F, Ctrl-arrow      move by word
	  Ctrl-K, Ctrl-U, Ctrl-W        kill to end of line, to start, one word back
	  up/down, Ctrl-R               history, and reverse search over it
	  Ctrl-C                        discard the snippet being typed, keep the session

	examples:
	  perl2golang repl
	  perl2golang repl --mode annotated
	  perl2golang repl --load warmup.pl

	exit status: 0 unless the session could not be started
	`)
}

// explainHelp is `perl2golang explain --help`.
func explainHelp() string {
	return dedent(`
	print one teaching concept, or look up what a diagnostic code means.

	usage:
	  perl2golang explain <concept-id>
	  perl2golang explain <P2Gxxxx>
	  perl2golang explain --list

	flags:
	      --list     list every concept with its title, and stop

	examples:
	  perl2golang explain slice-aliasing-and-copy
	  perl2golang explain "iteration order"     a search, when the id escapes you
	  perl2golang explain P2G4004               what a diagnostic means
	  perl2golang explain --list

	Concepts are the same documents a conversion writes into docs/concepts/, so
	anything named in a report can be read here without converting anything.
	`)
}

// versionHelp is `perl2golang version --help`.
func versionHelp() string {
	return dedent(`
	print the version and build information.

	usage:
	  perl2golang version
	  perl2golang --version

	The line names the perl2golang version, the commit it was built from when the
	binary carries that information, the Go toolchain that built it, and the
	platform it was built for.
	`)
}

// dedent turns an indented raw string literal into left-aligned help text. The
// literals above are indented to sit inside their functions, which keeps the
// source readable; this removes exactly that indentation.
func dedent(s string) string {
	s = strings.TrimPrefix(s, "\n")
	lines := strings.Split(s, "\n")
	var out strings.Builder
	for _, line := range lines {
		out.WriteString(strings.TrimPrefix(line, "\t"))
		out.WriteByte('\n')
	}
	// The literal ends with a newline and a tab before the backquote, which
	// dedent has just turned into one empty line too many.
	return strings.TrimSuffix(out.String(), "\n")
}
