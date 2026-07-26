package cli

import "strings"

// The help text is written out here rather than assembled from the flag
// package's defaults. Grouped flags with real examples are worth more than a
// list in registration order, and the groups answer, in order, what goes in,
// what comes out, how faithful the result is, and how loud the run is.

// rootHelp is `perl2go --help`.
func rootHelp() string {
	return dedent(`
	perl2go converts Perl 5 programs into Go, and explains the Go it produces.

	usage:
	  perl2go [flags] <file.pl>...     convert files (convert is the default command)
	  perl2go <command> [flags] [args]

	commands:
	  convert    convert Perl to Go; the default, so the word can be left out
	  explain    print a teaching concept, or look up a diagnostic code
	  version    print the version and build information
	  help       help for perl2go, or for any command

	common flags:
	  -o, --out DIR      write the generated project here (default: <basename>-go)
	  -e, --expr CODE    convert a snippet given on the command line
	      --stdout       write the artifacts to standard output instead of to files
	      --json         write one machine-readable object to standard output
	      --strict       exit nonzero when anything needs review
	      --force        overwrite an output directory that already has files in it
	  -v, --verbose      show every diagnostic in full, with its source
	      --color WHEN   auto, always, never (default: auto; NO_COLOR is honoured)
	  -h, --help         help for perl2go, or for any command
	      --version      print the version and exit

	examples:
	  perl2go report.pl                         convert one file into report-go/
	  perl2go lib/*.pl -o build/                convert several files, one directory each
	  perl2go -e 'print join ",", 1..5'         convert a snippet and print the Go
	  cat old.pl | perl2go -                    read Perl from standard input
	  perl2go explain slice-aliasing-and-copy   read one teaching concept
	  perl2go explain P2G4004                   look up what a diagnostic code means

	exit status:
	  0  the conversion finished; warnings and refusals are reported, not failures
	  1  --strict was given and the run produced warnings or refusals
	  2  the conversion failed and nothing was written
	  3  usage error

	Conversion never runs your Perl. No subprocess is spawned over your input and
	no network connection is made, so converting a file you have not read is safe.

	AI-assisted conversion and the interactive REPL are designed but not built yet.
	There is no --ai flag and no repl command in this release.

	Full flag list: perl2go convert --help
	`)
}

// convertHelp is `perl2go convert --help`.
func convertHelp() string {
	return dedent(`
	convert Perl 5 source into a Go project, with the reasoning written out.

	usage:
	  perl2go convert [flags] <file.pl>...
	  perl2go convert [flags] -e 'CODE'
	  perl2go convert [flags] -              read Perl from standard input

	For each input perl2go writes a directory holding the program, the same program
	annotated with what every construct means and where it came from, the teaching
	documents for the constructs this file actually used, and a conversion report
	that says plainly what did not convert cleanly.

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

	output control:
	  -v, --verbose        show every diagnostic in full, with its source line and
	                       carets, instead of the three-line summary
	      --color WHEN     auto, always, never (default: auto). Colour is only ever
	                       written to a terminal, never into a pipe.

	examples:
	  perl2go convert report.pl
	      writes report-go/ with the program, the annotated program, and docs/

	  perl2go convert report.pl -o /tmp/out --force
	      the same bundle, somewhere else, over whatever was there

	  perl2go convert lib/*.pl -o build/
	      convert a set of files, each into its own directory under build/

	  perl2go convert report.pl --strict
	      for CI: fail the build when anything needs a human

	  perl2go convert -e 'print "$_\n" for sort keys %ENV' > snip.go
	      print the Go for a snippet, with the notes on standard error

	  perl2go convert report.pl --json | jq -r '.conversions[0].report.stats'
	      read the numbers from a script

	  git show HEAD:tools/old.pl | perl2go convert -
	      convert something that is not on disk

	environment:
	  NO_COLOR             set to anything to turn colour off
	  TERM=dumb            same effect

	Conversion never runs your Perl: no subprocess is spawned over your input and no
	network connection is made.

	exit status: see perl2go --help
	`)
}

// explainHelp is `perl2go explain --help`.
func explainHelp() string {
	return dedent(`
	print one teaching concept, or look up what a diagnostic code means.

	usage:
	  perl2go explain <concept-id>
	  perl2go explain <P2Gxxxx>
	  perl2go explain --list

	flags:
	      --list     list every concept with its title, and stop

	examples:
	  perl2go explain slice-aliasing-and-copy
	  perl2go explain "iteration order"     a search, when the id is not remembered
	  perl2go explain P2G4004               what a diagnostic means and what to do
	  perl2go explain --list

	Concepts are the same documents a conversion writes into docs/concepts/, so
	anything named in a report can be read here without converting anything.
	`)
}

// versionHelp is `perl2go version --help`.
func versionHelp() string {
	return dedent(`
	print the version and build information.

	usage:
	  perl2go version
	  perl2go --version

	The line names the perl2go version, the commit it was built from when the
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
