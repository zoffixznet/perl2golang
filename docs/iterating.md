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

Each column counts entries that got that far.

| Column | An entry passes when |
|---|---|
| `translated` | every construct in it had a translation, so a single refusal fails the file |
| `typed` | no variable fell back to the dynamic value type |
| `emitted` | valid Go came out |
| `compiled` | the Go toolchain built it |
| `equivalent` | the built program printed what `perl` printed, byte for byte, and exited the same way |
| `honest` | tier 4 only: the tool said something true about a construct it cannot translate |

Tier 4 is judged by `honest` and not by `equivalent`, because those entries
exist to prove the tool fails well. A skipped check never counts as a pass.

Under the table:

- **Quality** counts TODOs left in the output, how often type inference gave up,
  and how many constructs were refused or approximated.
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

1. Entries that fail `equivalent`. The Go builds and runs and gets a different
   answer, which is the worst outcome and usually the most specific bug.
2. Entries that fail `compiled`. The reason names the file and line.
3. A high dynamic-fallback rate, which is type inference giving up.
4. A refusal that shows up across many entries.
5. A construct that converts but reads nothing like Go.

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

## The other half of the round

Conversion quality and teaching quality move together, and a round that improves
only one of them is half a round. When a construct starts converting, the lesson
that explains it should get better in the same change: a new concept in
`internal/teach/kb/`, a sharper explanation in one that already exists, or a
note in the lowering that cites it.

Every Go sample in a lesson is compiled by the test suite, and a sample followed
by an unlabelled output block is run and compared against that block byte for
byte. Write the output by running the sample, never by hand.
