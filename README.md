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

Everything runs locally. No account, no API key, and your Perl is never
executed. By default nothing opens a socket either; the one feature that talks
to anything talks to a model on your own machine, and only when you ask for it
with `--ai`.

## Install

Requires a Go toolchain, 1.24 or newer. The Go it generates needs 1.23.

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

The lessons cover the language itself (types and zero values, `nil`, slices,
maps, errors, interfaces, pointers, goroutines), the parts of the standard
library a script actually lands on (`fmt`, `strings`, `strconv`, `sort`,
`bufio`, `regexp`, `os/exec`, `encoding/json`, `time`, `path/filepath`,
`flag`), and the tooling around them (`go test` and the table-driven habit,
benchmarks, coverage, the race detector, `go vet`). Every Go sample in them is
compiled and run by this repository's test suite, and the output each lesson
shows is the output its code actually produces.

## The optional local model

Everything above works with no model, no account, no network and no
configuration, and that is the mode the tool is built around. `--ai` is an
extra: it hands the finished Go to a model running on your own machine and asks
it to name the things the converter had to invent names for.

```
perl2go ai status                 what is configured, and what this machine can run
perl2go report.pl --ai            convert, and let the model name things
```

Without `--ai`, perl2go opens no socket at all. There is no telemetry, no
update check and no hosted service anywhere in it.

What the model is asked for is narrow on purpose:

- better names for short locals, worked out from how they are used
- names for struct types and their fields
- doc comments on declarations that have none

That is all, by default. The model returns names, never code, and this tool does
the rewriting. Every name has to be a valid Go identifier in the surrounding style,
must not collide with anything already in the file, and must leave the file
parsing, compiling and passing `go vet` alongside the rest of its package. A
name that fails any of those is dropped and the converter's own name is kept.
`--ai` can improve the result or leave it alone; it cannot damage it.

It needs a local runtime speaking the Ollama API, which perl2go talks to and
does not manage. Models are a machine-wide resource shared with everything else
you run, so perl2go uses a model you already have, never downloads one to
convert a file, and never removes, moves or copies one. `OLLAMA_HOST` and
`OLLAMA_MODELS` are honoured as the runtime's own tools honour them.

`perl2go ai setup` inspects the machine, reports what it found, lists the
freely licensed models that fit it with their real download sizes, and prints
the exact commands it would run. It stops there unless you add `--yes`.

If the runtime is not running, is missing the model, is out of memory or is too
slow, the conversion is the deterministic one, the exit status is normal, and a
line on standard error says which of those happened.

A conversion costs one request per generated program. Loading a model that is
not resident dominates that, so the first conversion after a reboot is slower
than the rest by a wide margin.

## The interactive session

`perl2go repl` gives you a prompt. Type Perl, see the Go it becomes, and see
the Go concepts behind it. It is the fastest way to answer "what does this look
like in Go", and the answer is the same Go a file conversion would produce.

```
$ perl2go repl
perl2go 0.1.0  type Perl, see the Go. :help for commands, :quit to leave.

perl> my @nums = (3, 1, 4, 1, 5);
  nums := []int{3, 1, 4, 1, 5}
  concepts: slices-not-arrays, var-vs-short-declaration  (:explain to expand)

perl> my %seen;
  seen := map[string]any{}
  concepts: nil-slices-vs-nil-maps  (:explain to expand)

perl> $seen{$_}++ for @nums;
  seen := map[string]int{}
  for _, item := range nums {
  	seen[strconv.Itoa(item)]++
  }
  (that replaced 1 line shown earlier; :go full shows the session as it stands)
  concepts: explicit-conversions-no-coercion, range-is-not-foreach  (:explain to expand)

perl> sub trim {
 ...>     my $s = shift;
 ...>     $s =~ s/^\s+|\s+$//g;
  (sub body opened at line 1; a blank line twice discards it)
 ...>     return $s;
 ...> }
  // pattern2 matches the pattern ^\s+|\s+$.
  var pattern2 = regexp.MustCompile("^\\s+|\\s+$")
  // trim performs one step of the program's work.
  func trim(s any) any {
  	s = pattern2.ReplaceAllString(toText(s), "")
  	return s
  }
  (support code added: toText; :go full includes it)
  concepts: replace-and-expansion, submatch-and-named-groups  (+2 more; :explain to expand)
```

Three things in that transcript are the point of the feature:

- The session holds a **program**, not a list of snippets. `my %seen` became a
  `map[string]any` and then a `map[string]int` once the next line showed what
  goes in it. When re-inference changes something already printed, the session
  says so instead of quietly contradicting itself.
- A snippet that spans lines needs **no continuation marker**. The prompt keeps
  reading until the snippet parses, so `sub trim {` simply carries on. Two
  blank lines in a row throw away whatever is half-typed.
- The **concepts** line names what the snippet touched, once per session.
  `:explain` expands the last snippet's concepts, and `:explain <id>` prints the
  whole lesson, the same lesson a conversion writes into `docs/concepts/`.

Meta commands, all listed by `:help`:

| | |
|---|---|
| `:go [full]` | reprint the last Go, or the whole session ready to paste |
| `:explain [WHAT]` | expand a concept, a `P2G` code, or the last snippet |
| `:concepts` | every concept this session touched, with its title |
| `:why` | the converter's reasoning for the last snippet |
| `:diag` | the last snippet's diagnostics in full, with source and carets |
| `:vars` | the variables in scope and the Go type inferred for each |
| `:perl` | the Perl the session holds so far |
| `:mode clean\|annotated` | plain Go, or Go with the reasoning in comments |
| `:notes on\|off` | show or hide the concept line |
| `:save FILE` / `:load FILE` | write the session out, or type a file in |
| `:reset` / `:clear` | forget the session, or clear the screen |
| `:quit` | leave; `:q` and Ctrl-D do the same |

Nothing ends the session except `:quit`, Ctrl-D and a signal. Perl that does
not parse is reported with its position and leaves the session program
untouched; a construct with no Go equivalent shows the refusal and what to
write instead.

At a terminal you get line editing and history: arrow keys, `Ctrl-A`/`Ctrl-E`,
word movement with `Alt-B`/`Alt-F`, `Ctrl-K`/`Ctrl-U`/`Ctrl-W`, history with the
up and down arrows and `Ctrl-R` to search it, and `Ctrl-C` to throw away the
snippet you are typing without leaving. History is kept in
`$XDG_STATE_HOME/perl2go/history`; `--no-history` turns that off.

A session also works from a pipe, prompts and all, which makes a transcript
something you can save, read, diff and replay:

```
perl2go repl < session.pl > transcript.txt
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

A refusal never stops the program. Where a construct could not be converted the
code calls `notImplemented`, which hands back the zero value of the type that
position wanted and carries on, so a script with five refusals in it still
builds, still runs, and still does the parts that converted. The first time the
program reaches one it prints a line beginning `TODO` on standard error and
keeps going, and standard output is left alone so the two programs can still be
compared. Search the generated code for `notImplemented` to find every gap, or
for a diagnostic code to find one. Nothing the stand-in returns is an answer:
treat anything downstream of a gap the program actually reached as unproven
until you have written the real code in its place.

Pass `--strict` to make any refusal a nonzero exit status, which is what you
want in a script.

## Known limitations

These are real and current, not oversights:

- **By default the optional local model names things and nothing else.** It is
  not asked to write the tutorials, and that is a measured decision rather than
  caution: a 7B model writes thinner explanations than the ones in the knowledge
  base, and it will state something false about your own code with complete
  confidence. Naming is a job whose worst outcome is a mediocre name, and every
  name is checked before it is used. The two jobs that go further, an idiom
  review that has the model rewrite Go and a rewrite of the walkthrough
  document, are reachable through `--ai-jobs`, are labelled experimental, and
  say so when you turn them on. The concept lessons and the conversion report
  are never touched by a model at all.
- **The session's line editing needs a terminal that supports raw mode**, which
  covers Linux, macOS and the BSDs. Anywhere else the session falls back to
  reading whole lines: everything still works, but the arrow keys, history and
  `Ctrl-R` do not, and the session says so once when it starts. Given a pipe
  rather than a terminal it reads whole lines too, and says nothing, because a
  transcript should hold the session and not a note about the session. There is
  no readline dependency to install, because there is no dependency at all.
- **Coverage is aimed at ordinary script-shaped Perl:** scalars, arrays, hashes,
  subroutines, references, control flow, regular expressions, string and list
  builtins, file reading and writing, command-line options, classes, and the
  common CPAN modules whose work the Go standard library does. A package that
  blesses a hash reference becomes a struct with methods on a pointer receiver,
  `@ISA` becomes embedding, and a module in a `.pm` file beside the script is
  converted with it. `tie`, `format`, typeglobs, source filters, `AUTOLOAD`,
  `DESTROY` and operator overloading are reported rather than converted.
- **Embedding is not inheritance**, and the one place that shows is a base
  class calling a method its subclasses override: Go resolves that call against
  the base and the override is never reached. Every such call site is reported
  by name, and the lesson beside it gives the interface-and-composition shape
  that does work.
- **Go's regular expressions are RE2**, which has no backreferences and no
  lookaround. A pattern using either is refused by name rather than translated
  into something that matches different text. The refusal names the feature, the
  match site carries a `TODO`, and the report says what to write instead.
- **The generated Go is best effort.** It is meant to be read, run and edited,
  not trusted blindly. Where the two languages disagree in a way that survives
  translation, the report says so; where the tool got something wrong, the Go
  compiler and your own reading are the backstop. Expect a straightforward
  script to compile and need a little hand-finishing, and expect one built on
  nested data structures or code references to need more: those are where type
  inference gives up and falls back to `any`, and the report says how often it
  did. `make score` measures all of this against the bundled corpus, and the
  numbers it prints are the real ones.
- **Perl is not required to run this tool**, and converting a file never runs
  it. Perl is only used by this repository's own test suite, against scripts in
  `testdata/corpus/`.

## Working on perl2go

```
make test       # the full suite
make test-short # skips the toolchain-heavy tests
make score      # runs the corpus and prints the conversion scorecard
make lint       # go vet plus a gofmt check
make explain    # list the teaching concepts; TOPIC=<id> reads one
make repl       # start the interactive session
make repl-demo  # pipe a canned session through the repl and check the transcript
make demo       # convert a corpus script and run the result
make deps       # checks for the system tools the other targets need
```

`make score` is the measure of conversion quality: it converts every script in
`testdata/corpus/`, compiles the result, runs it, and compares its output byte
for byte against what real `perl` produces. It writes the numbers to a file and
prints the change since the previous run. `ARGS` narrows it, so
`make score ARGS="-tier tier2 -v"` scores one tier and shows every entry.

The corpus is tiered by difficulty. Tiers 1 and 2 are ordinary Perl and are what
this release targets. Tiers 3 and 4 and the `domain/` set are harder material
kept as a measure of where the tool stands, not as a bar it currently clears.

[docs/iterating.md](docs/iterating.md) is the guide to improving the
conversion: how to read the scorecard, how to pick what to work on, and the
rules a change has to respect.
