# perl2golang

Convert Perl 5 scripts to Go, and learn Go from your own code while you do it.

`perl2golang` reads a Perl script and writes a Go project. It produces two
copies of the same program: a plain one that reads like Go somebody wrote by
hand, and an annotated one, buried in comments explaining what each Go construct
is, why it is written that way, and which line of your Perl it came from.
Alongside them it writes a set of documents: a walkthrough tying regions of your
script to regions of the output, a lesson for every Go concept your code
actually touched, and an honest account of anything it could not translate.

The conversion is not meant to be a 100% complete, as there are many concepts and behaviors that don't translate very well and would require human judgement to translate into applicable Go variants. The point is to produce something that is usable while also educating the user about Golang to facilitate their learning of the language.

Everything runs on your machine and nothing you convert goes anywhere. There
is no external service, no account, no API key, no telemetry, and no network
connection of any kind; your Perl is read, never executed, and never sent
anywhere.

## Install

Download the archive for your system from the releases page, unpack it, and put
the `perl2golang` binary on your `PATH`. It is one static binary with nothing
beside it: no Go toolchain, no Perl, no libraries to install first. Each
release also carries a `SHA256SUMS` file to check the download against.

To build it yourself instead, with Go 1.24 or newer:

```
make build      # builds ./bin/perl2golang
make install    # installs perl2golang into ~/.local/bin (set BINDIR for elsewhere)
```

`make help` lists every target. Either way, building the Go the tool writes
needs a toolchain of 1.23 or newer.

## Platforms

Binaries are published for Linux, macOS and Windows, on 64-bit Intel and on
ARM. Every one of them is built from the same source in the same release. The
project is developed and tested on Linux, which is where the test suite and the
corpus run, so that is the platform the tool is exercised hardest on.

Two things do not work the same everywhere:

- On Windows the session reads whole lines instead, because it cannot put the
  terminal into raw mode. See "Known limitations".
- A converted program that runs a command written as one string hands it to
  `sh`, which is what the original did. On Windows that needs a POSIX shell to
  be present.

## Use

Convert a file. The output goes to `<name>-go/` unless you say otherwise:

```
perl2golang convert report.pl
perl2golang report.pl -o /tmp/report-go
```

Convert a snippet and print the result instead of writing files:

```
perl2golang -e 'my %count; $count{$_}++ for @ARGV; print "$_ => $count{$_}\n" for sort keys %count'
```

Read Perl from standard input:

```
cat report.pl | perl2golang -
```

Look a concept up directly, without converting anything:

```
perl2golang explain slice-aliasing-and-copy
perl2golang explain --list
```

The lessons cover the language itself (types and zero values, `nil`, slices,
maps, errors, interfaces, pointers, goroutines), the parts of the standard
library a script actually lands on (`fmt`, `strings`, `strconv`, `sort`,
`bufio`, `regexp`, `os/exec`, `encoding/json`, `time`, `path/filepath`,
`flag`), and the tooling around them (`go test` and the table-driven habit,
benchmarks, coverage, the race detector, `go vet`). Every Go sample in them is
compiled and run by this repository's test suite, and the output each lesson
shows is the output its code actually produces.

## What the output looks like

Three short scripts and the Go this tool writes for them. Each Go program
below is the tool's real output, pasted unedited, and each pair was run side
by side and printed identical bytes.

A word count, the classic hash-and-sort shape:

```perl
my %count;
while (my $line = <STDIN>) {
    chomp $line;
    $count{lc $_}++ for split /\W+/, $line;
}
delete $count{''};

for my $word (sort { $count{$b} <=> $count{$a} || $a cmp $b } keys %count) {
    printf "%5d %s\n", $count{$word}, $word;
}
```

```go
package main

import (
	"bufio"
	"cmp"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
)

// pattern2 matches the pattern \W+.
var pattern2 = regexp.MustCompile("\\W+")

// main is the program's entry point.
func main() {
	count := map[string]int{}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		for _, item := range splitPattern(pattern2, line, 0) {
			count[strings.ToLower(item)]++
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(255)
	}
	delete(count, "")
	sorted := slices.Clone(slices.Collect(maps.Keys(count)))
	slices.SortStableFunc(sorted, func(a string, b string) int {
		return cmp.Or(cmp.Compare(count[b], count[a]), strings.Compare(a, b))
	})
	for _, word := range sorted {
		fmt.Printf("%5d %s\n", count[word], word)
	}
}
```

`splitPattern` is one of the small helpers the tool writes into `helpers.go`
when the program needs it, because Perl's `split` drops trailing empty fields
and Go's `regexp.Split` does not.

A log scan: arguments, `open or die` with the right exit status, a capture:

```perl
my $file = shift @ARGV or die "usage: $0 LOGFILE\n";
open my $fh, '<', $file or die "cannot open $file: $!\n";

my %seen;
while (<$fh>) {
    next unless /ERROR\s+\[(\w+)\]/;
    $seen{$1}++;
}
close $fh;

print "$_: $seen{$_}\n" for sort keys %seen;
```

```go
package main

import (
	"bufio"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
)

// errorValPattern2 matches the pattern ERROR\s+\[(\w+)\].
var errorValPattern2 = regexp.MustCompile("ERROR\\s+\\[(\\w+)\\]")

// args holds the command line arguments, without the program name.
var args = os.Args[1:]

// main is the program's entry point.
func main() {
	var first string
	if len(args) > 0 {
		first, args = args[0], args[1:]
	}
	file := first
	if !truthy(file) {
		fmt.Fprint(os.Stderr, "usage: "+os.Args[0]+" LOGFILE\n")
		os.Exit(255)
	}
	fh, err := os.Open(file)
	if err != nil {
		fmt.Fprint(os.Stderr, "cannot open "+file+": "+errnoText(err)+"\n")
		os.Exit(255)
	}
	seen := map[string]int{}
	scanner := bufio.NewScanner(fh)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		_ = line
		m := errorValPattern2.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		seen[m[1]]++
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(255)
	}
	fh.Close()
	for _, item := range slices.Sorted(maps.Keys(seen)) {
		fmt.Printf("%s: %d\n", item, seen[item])
	}
}
```

A `bless`-based class, which becomes a struct with methods:

```perl
package Tally;

sub new {
    my ($class, %args) = @_;
    my $self = { name => $args{name}, total => 0 };
    return bless $self, $class;
}

sub add {
    my ($self, $amount) = @_;
    $self->{total} += $amount;
    return $self;
}

sub report {
    my $self = shift;
    return sprintf "%s: %d", $self->{name}, $self->{total};
}

package main;

my $t = Tally->new(name => 'widgets');
$t->add($_) for 1 .. 4;
print $t->report, "\n";
```

```go
package main

import "fmt"

// Tally is one Tally and everything it knows about itself.
type Tally struct {
	Name  string
	Total int
}

// NewTally builds a Tally.
func NewTally(name string) *Tally {
	self := &Tally{Name: name, Total: 0}
	return self
}

func (t *Tally) Add(amount int) *Tally {
	t.Total += amount
	return t
}

func (t *Tally) Report() string {
	return fmt.Sprintf("%s: %d", t.Name, t.Total)
}

// main is the program's entry point.
func main() {
	t := NewTally("widgets")
	for i := 1; i <= 4; i++ {
		t.Add(i)
	}
	fmt.Print(t.Report(), "\n")
}
```

The output is not always this clean. A script leaning on nested references,
context tricks or code generation comes out with `any`-typed values, helper
calls and TODO markers, and the report says so entry by entry. These three
are honest examples of the ordinary case, not the worst one.

## The interactive session

`perl2golang repl` gives you a prompt. Type Perl, see the Go it becomes, and see
the Go concepts behind it. It is the fastest way to answer "what does this look
like in Go", and the answer is the same Go a file conversion would produce.

```
$ perl2golang repl
perl2golang 1.0.0  type Perl, see the Go. :help for commands, :quit to leave.

perl> my @nums = (3, 1, 4, 1, 5);
  nums := []int{3, 1, 4, 1, 5}
  concepts: var-vs-short-declaration, slices-not-arrays  (:explain to expand)

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
  func trim(s any) any {
  	s = pattern2.ReplaceAllString(toText(s), "")
  	return s
  }
  (support code added: toText; :go full includes it)
  concepts: replace-and-expansion, submatch-and-named-groups  (+2 more; :explain to expand, :diag for the full note)
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
`$XDG_STATE_HOME/perl2golang/history`; `--no-history` turns that off.

A session also works from a pipe, prompts and all, which makes a transcript
something you can save, read, diff and replay:

```
perl2golang repl < session.pl > transcript.txt
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
three ways: a `TODO` in the generated code, an entry in the report with a
stable diagnostic code, and a line in the terminal summary.
`perl2golang explain P2G4004` prints the full entry for any code.

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

## What to expect

Measured against the bundled corpus at this version: every conversion emits
Go, 99% of it compiles, 87% of programs convert every statement, and 79% of
the runnable ones match perl's output byte for byte. Those numbers flatter
the tool exactly as much as the corpus resembles your code, and the corpus
is script-shaped on purpose. On a sample of installed Perl modules and
scripts the tool had never seen, about half compiled, because module-heavy
code leans on export and OO machinery that is out of scope. The short
version: an ordinary script converts and runs; a module tree mostly does
not, and says why.

## Known limitations

These are real and current, not oversights:

- **The session's line editing needs a terminal that supports raw mode**, which
  covers Linux, macOS and the BSDs. On Windows, and anywhere else without it,
  the session falls back to reading whole lines: everything still works, but
  the arrow keys, history and `Ctrl-R` do not, and the session says so once
  when it starts. Given a pipe rather than a terminal it reads whole lines too,
  and says nothing, because a transcript should hold the session and not a note
  about the session. There is no readline dependency to install, because there
  is no dependency at all.
- **Coverage is aimed at ordinary script-shaped Perl:** scalars, arrays, hashes,
  subroutines, references, control flow, regular expressions, string and list
  builtins, file reading and writing, command-line options, classes, and the
  common CPAN modules whose work the Go standard library does. A package that
  blesses a hash reference becomes a struct with methods on a pointer receiver,
  `@ISA` becomes embedding, and a module in a `.pm` file beside the script is
  converted with it. The constructs it refuses on principle are listed under
  "Out of scope" below.
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
  did.
- **Perl is not required to run this tool**, and converting a file never runs
  it. Perl is only used by this repository's own test suite, against scripts in
  `testdata/corpus/`.

## Out of scope

Decided, not deferred. Issues asking for these will be met with a pointer to
this list:

- **Module and CPAN-scale conversion.** The unit of conversion is a script,
  plus any `.pm` files sitting beside it. Pointing the tool at an installed
  module tree produces well-labelled refusals, not a port.
- **`use overload`.** An overloaded operator is refused by name, with the
  method call to write in its place.
- **`tie`, `format`/`write`, typeglobs, source filters, `AUTOLOAD`, `DESTROY`,
  `fork`/`waitpid`, and writes through `@_` aliasing.** Each is refused with
  an explanation of what to write instead. The refusal is the finished
  feature, not a stopgap.
- **A backtracking regex engine.** Backreferences and lookaround are refused
  by name rather than approximated into something that matches different
  text, and the generated project stays dependency-free rather than take on
  an engine that provides them.
- **AI assistance of any kind.** A mode that put a locally hosted model in the
  loop was built, measured and then removed. Asked only to write the Go the
  converter had refused, under checks that stopped it touching anything else,
  it filled almost nothing: across 30 real Perl files carrying 346 unconverted
  sites, a 7B model landed one usable fill and a 9B model landed none, because
  most gaps in real code are calls into Perl modules that have no Go
  equivalent. Asked to restyle working code instead, it corrupted one touched
  program in four. That is a poor return for asking you to install an
  inference runtime and download several gigabytes, so conversion is
  deterministic and stays that way. Hosted API providers were never on the
  table either.
- **Reworking a script's Unix assumptions for Windows.** Windows gets a binary
  and the Go the tool writes is ordinary portable Go, but where a script leans
  on a POSIX shell, on file modes or on process semantics, the translation
  carries that assumption across rather than inventing a Windows equivalent
  for it.

## Working on perl2golang

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

The corpus is tiered by difficulty: tiers 1 and 2 are ordinary Perl, tier 3
is object systems, parsers and process control, `domain/` is realistic
sysadmin and data-wrangling scripts, and tier 4 holds adversarial constructs
judged on whether the tool tells the truth about them rather than on whether
they convert.

[docs/iterating.md](docs/iterating.md) is the guide to improving the
conversion: how to read the scorecard, how to pick what to work on, and the
rules a change has to respect.

## License

MIT. See [LICENSE](LICENSE).
