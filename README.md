# perl2go

Convert Perl 5 scripts to Go, and learn Go from your own code while you do it.

`perl2go` reads a Perl script and writes a Go project. It produces two copies of
the same program: a plain one that reads like Go somebody wrote by hand, and an
annotated one, buried in comments explaining what each Go construct is, why it
is written that way, and which line of your Perl it came from. Alongside them it
writes a set of documents: a walkthrough tying regions of your script to regions
of the output, a lesson for every Go concept your code actually touched, and an
honest account of anything it could not translate.

The conversion is the excuse. The point is that you finish reading knowing Go.

Everything runs locally. No network, no account, no API key, and your Perl is
never executed.

## Install

Requires a Go toolchain, 1.23 or newer.

```
make build      # builds ./bin/perl2go
make install    # installs perl2go into ~/.local/bin
```

`make help` lists every target.

## Use

Convert a file. The output goes to `<name>-go/` unless you say otherwise:

```
perl2go convert report.pl
perl2go report.pl -o /tmp/report-go
```

Convert a snippet and print the result instead of writing files:

```
perl2go -e 'my %count; $count{$_}++ for @ARGV; print "$_ => $count{$_}\n" for sort keys %count'
```

Read Perl from standard input:

```
cat report.pl | perl2go -
```

Look a concept up directly, without converting anything:

```
perl2go explain slice-aliasing-and-copy
perl2go explain --list
```

## What you get

Converting `report.pl` produces `report-go/`:

```
report-go/
  go.mod
  main.go                 the program, as ordinary Go
  helpers.go              small support functions, only the ones used
  annotated/
    main.go               the same program, explained line by line
  README.md               how to build and run both of them
  docs/
    start-here.md         what was produced and what to read first
    walkthrough.md        your Perl beside the Go it became, region by region
    conversion-report.md  what converted, what was approximated, what was not
    not-translated.md     every gap, with what to do about it by hand
    exercises.md          checkable tasks against your own generated code
    go-for-perl-developers.md   the general orientation
    concepts/             one lesson per Go concept your code touched
```

Both programs compile and behave the same way. The annotations are comments and
nothing else, which is a test in this repo rather than a promise.

Run either one:

```
cd report-go
go run .              # the plain program
go run ./annotated    # the annotated one
```

The generated project has no dependencies. `go.mod` names the module and the
language version and nothing else.

## Honesty about what it could not do

Every conversion produces a report. Anything approximated or refused appears
three ways: a `TODO` in the generated code, an entry in the report with a stable
diagnostic code, and a line in the terminal summary. `perl2go explain P2G4004`
prints the full entry for any code.

A refusal names the construct, says why Go cannot express it the same way, and
tells you what to write instead. That is the intended output for the parts of
Perl that have no Go counterpart, not a failure to be worked around.

Pass `--strict` to make any refusal a nonzero exit status, which is what you
want in a script.

## Known limitations

These are real and current, not oversights:

- **AI-assisted improvement is planned and not implemented.** A later release
  will be able to run the output through a locally hosted model to improve the
  Go and enrich the tutorials, opt-in and offline. The tool is fully functional
  without it, and nothing in it makes a network connection today.
- **There is no REPL yet.** Use `-e` to see what a snippet becomes.
- **Coverage is aimed at ordinary script-shaped Perl:** scalars, arrays, hashes,
  subroutines, references, control flow, regular expressions, string and list
  builtins, file reading and writing, and the common CPAN modules whose work the
  Go standard library does. Object-oriented Perl (`bless`, `@ISA`, method
  dispatch), `eval`, `local`, `tie`, `format`, typeglobs and source filters are
  reported rather than converted.
- **Go's regular expressions are RE2**, which has no backreferences and no
  lookaround. A pattern using either is refused by name rather than translated
  into something that matches different text.
- **The generated Go is best effort.** It is meant to be read, run and edited,
  not trusted blindly. Where the two languages disagree in a way that survives
  translation, the report says so; where the tool got something wrong, the Go
  compiler and your own reading are the backstop.
- **Perl is not required to run this tool**, and converting a file never runs
  it. Perl is only used by this repository's own test suite, against scripts in
  `testdata/corpus/`.

## Working on perl2go

```
make test       # the full suite
make test-short # skips the toolchain-heavy tests
make score      # runs the corpus and prints the conversion scorecard
make lint       # go vet plus a gofmt check
make deps       # checks for the system tools the other targets need
```

`make score` is the measure of conversion quality: it converts every script in
`testdata/corpus/`, compiles the result, runs it, and compares its output byte
for byte against what real `perl` produces. It writes the numbers to a file and
prints the change since the previous run.

The corpus is tiered by difficulty. Tiers 1 and 2 are ordinary Perl and are what
this release targets. Tiers 3 and 4 and the `domain/` set are harder material
kept as a measure of where the tool stands, not as a bar it currently clears.
