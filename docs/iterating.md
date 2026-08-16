# Improving the conversion

The corpus and the scorecard exist so that "the conversion got better" is a
claim with a number behind it. This is how to run one round of that.

```
make score          # measure
                    # fix one thing, at the level it belongs
                    # add a corpus entry or two
                    # deepen the lesson the fix touches
make test           # everything still green
git commit          # with the score delta in the message
```

That is the whole loop. The rest of this page is what each step means.

## Reading the scorecard

`make score` converts every program under `testdata/corpus/`, builds what comes
out, runs it beside real `perl`, and prints a table:

```
  tier    entries    translated         typed      emitted     compiled    equivalent      honest
  tier1        37   30/37 (81%)   17/37 (46%)  37/37 (100%)  36/37 (97%)   11/37 (30%)          -
```

Each column counts entries that got that far. Knowing what a column proves
matters less than knowing what it does not, because a defect no column can
express is invisible however many rounds stare at the table.

| Column | An entry passes when | What that does not prove |
|---|---|---|
| `translated` | every construct produced either code or a diagnostic; a single refusal fails the file | nothing about whether the code is right, only that none of it went missing unannounced |
| `typed` | no variable fell back to the dynamic value type | not that the types are the best ones, only that inference never gave up |
| `emitted` | valid Go came out | validity is `go/parser`'s standard, not the compiler's |
| `compiled` | the Go toolchain built it | a program can build and still be wrong everywhere |
| `equivalent` | the built program matched `perl` byte for byte on stdout, on exit status, and on every file it wrote; stderr wording too, unless the entry sanctions stderr, where only its presence must agree | equivalence on this entry's input only: an approximation that diverges on inputs the entry never feeds it passes clean |
| `honest` | tier 4 only: the entry's own standard held, and every `report-must-contain` and `diagnostic-must-contain` line in its expectation was satisfied by the report's own words | the report said the required true thing; whether it says it well is a reading judgment, not a column |

Tier 4 is judged by `honest` and not by `equivalent`, because those entries
exist to prove the tool fails well. A skipped check never counts as a pass.
An entry whose output legitimately differs run to run carries a `verify.pl`
invariant oracle and is judged by it instead of a byte diff; one with
neither an `expected_stdout` nor an oracle cannot be checked at all, and the
scorecard says so in a corpus note rather than skipping in silence.

Under the table:

- **Quality** counts TODOs left in the output, how often type inference gave up,
  how many constructs were refused or approximated, how many statements
  vanished, and how many built programs stopped on a runtime panic instead of
  running to the end.

  *Statements that vanished* is the safety net under the lowering: a statement
  that produced no code and no diagnostic of its own was silently dropped, the
  one wrong this tool promises never to commit, and the net marks it (P2G3598)
  instead of letting it disappear. Each marked statement also fails
  `translated`, but the count is kept apart because every one is a converter
  defect with a known site: some lowering path returned nothing without saying
  so, and the fix is to teach that path to either translate the statement or
  refuse it for its own stated reason.

  *Programs that panicked* is the number that says whether a partial conversion
  is usable: a program that dies on its fourth line teaches nothing about the
  forty lines below it, however well they converted. Drive both of these
  towards zero even when it costs nothing on the other columns.
- **Where entries fall over first** groups every failing entry under the
  earliest stage it failed, with the reason. This is the list to pick from.
- **Corpus notes** appear when an entry's own files contradict its row in
  `testdata/corpus/MANIFEST.json`. The files win; the note is there to be fixed.
- **Since the last run** is the delta against `testdata/scorecard.json`.

## The results file

`testdata/scorecard.json` is the committed record of the last full run. It holds
results and nothing else: no timings, no machine details. A run that reproduces
the stored results leaves the file alone, so a diff on it always means the
conversion moved, and running `make score` to look at the numbers does not dirty
your tree.

Two consequences worth knowing:

- A narrowed run does not write the file. `make score ARGS="-tier tier2"` says
  nothing about the rest of the corpus and must not become the baseline the next
  full run is compared against. Pass `-out somewhere.json` if you want a partial
  run recorded.
- Commit the file with the change that moved it, and put the delta in the commit
  message.

Useful narrowings while working on one thing:

```
make score ARGS="-tier tier2"                 # one tier
make score ARGS="-only regex -v"              # entries matching a name, in full detail
make score ARGS="-short"                      # skip the equivalence stage, the slow one
```

## Picking what to work on

Roughly in this order:

1. Statements that vanished, and programs that panicked. Both are converter
   defects with a known site, both are listed by entry under Quality, and
   both undermine every other number until they are zero.
2. Entries that fail `equivalent`. The Go builds and runs and gets a different
   answer, which is the worst outcome and usually the most specific bug.
3. Entries that fail `compiled`. The reason names the file and line.
4. A high dynamic-fallback rate, which is type inference giving up.
5. A refusal that shows up across many entries.
6. A construct that converts but reads nothing like Go.

Prefer the smallest fix that is still at the right level. Improving type
inference beats special-casing an expression; adding an analysis pass beats
patching the emitter. A fix in the right place usually moves several entries at
once, and the delta will tell you whether it did.

## Corpus files are never special-cased

The converter must not recognise a corpus file, a corpus path, or anything else
that would make the score better without making the conversion better. Nothing
in the scoring harness looks at an entry's name either: every decision comes
from the manifest and from the entry's own files.

Weakening an entry to make it pass is the same offence. If a case turns out to
be genuinely unconvertible, move it to `tier4/` with an `expectation.md` saying
what the tool must report about it, and say so in the commit message. That tier
is judged by whether the tool tells the truth, which is a real standard and a
fair place for it.

## Adding entries

Each round should leave the corpus wider than it found it: an entry covering
the ground the fix opened up, and one covering the neighbouring case that still
fails, so the next round has a target with a recorded expectation behind it.

```
make corpus-add TIER=tier2 NAME=my-new-case
```

The script scaffolds the directory, registers the entry in the manifest, and
prints the remaining steps when it finishes. Once the program is written:

```
make corpus-record TIER=tier2 NAME=my-new-case
```

records what it prints, by running it under real `perl` the same way the
scorecard does: from inside its own directory, with its own `cmd` and `stdin`,
twice, refusing to record an entry whose two runs disagree.
`testdata/corpus/README.md` describes the layout and the rules an entry has to
follow, the most important being that it must be deterministic and must not
depend on where the repository lives.

## What the numbers do not measure

The table is a floor, not a verdict, and some of what matters most has no
column on purpose. Know the list, so a good-looking table is read as "the
measured things held" and nothing more:

- **The teaching bundle has no metric.** By this project's own thesis the
  bundle is the product, and its quality is prose quality: whether a Perl
  expert would actually learn Go from it. The suite holds the floor
  mechanically (every sample compiles, samples with output blocks run and
  match them, every cited lesson exists), but above that floor the only
  honest measurement is reading a full bundle as the target reader and
  fixing what fails the reading. Nothing on the scorecard moves when an
  explanation is muddy.
- **Idiomatic Go is measured only at the floor.** Everything emitted is
  gofmt-shaped by construction, `compiled` holds, and `go vet` has been
  checked against generated output and currently finds nothing the build
  does not. Whether the Go reads like Go, the difference between a struct
  and a `map[string]any`, is a review judgment.
- **Approximations are only tested where the corpus feeds them.** 1200-odd
  reported approximations are honest notes, and `equivalent` proves each
  entry's own input unaffected. Which of them bite on other inputs is
  unmeasured.
- **stdout/stderr interleaving is not compared.** The two streams are
  captured separately, so a program that writes the right bytes to each in
  the wrong relative order still matches.
- **Resources are not audited.** Nothing checks that handles are closed or
  files are cleaned up unless the difference reaches output, exit status, or
  the files-written comparison.

## Corpus health: measure against Perl you did not write

Every corpus entry was written for this corpus, so the corpus can only ask
questions its authors thought of. A corpus that has stopped discriminating
looks exactly like rising scores. The antidote is to sample real Perl from
outside the project, the machine's own installed modules and scripts are
right there, run it through the converter, and compare the failure profile
with what the corpus predicts. When the profiles disagree, the corpus is
missing a shape, and the fix is a new distilled entry, never a copy of
someone else's file.

One such pass, over 59 installed modules and scripts, found what the corpus
could not: a parser loop that never terminated on a legal trailing comma
above an `__END__` marker, hex literals in float company killing whole
conversions, a sub named `init` colliding with Go's runtime-called function,
Latin-1 source reaching the emitter, and a compile rate of 58% against the
corpus's 98%. The corpus now carries entries distilled from those shapes,
and the gap between the two numbers is the honest measure of how much the
corpus flatters the tool: the corpus is script-shaped by design, while
module-heavy code leans on OO and export machinery that mostly does not
convert yet.

## The other half of the round

Conversion quality and teaching quality move together, and a round that improves
only one of them is half a round. When a construct starts converting, the lesson
that explains it should get better in the same change: a new concept in
`internal/teach/kb/`, a sharper explanation in one that already exists, or a
note in the lowering that cites it.

Every Go sample in a lesson is compiled by the test suite, and a sample followed
by an unlabelled output block is run and compared against that block byte for
byte. Write the output by running the sample, never by hand.

Two things that compiling cannot tell you, and that the suite checks separately.
A sample may only use the standard library as of the `go` directive in `go.mod`,
because that is the version the project asks readers to install; the compiler
will not enforce it, since the directive gates language features rather than the
library, so a toolchain newer than the floor builds a call to a method that did
not exist yet and the sample breaks only for the reader who believed the floor.
`TestSamplesStayWithinTheProjectGoVersion` runs the `stdversion` analyser to
catch that on any toolchain. And a documented output block may not depend on
behaviour the language does not promise: the capacity `append` picks when it
reallocates, map iteration order, goroutine interleaving. Those differ between
releases, so print the property the lesson is actually about - that the capacity
grew, that the keys are all there - rather than the number a particular runtime
happened to produce.
