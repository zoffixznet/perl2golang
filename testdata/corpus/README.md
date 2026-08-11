# Perl conversion corpus

219 self-contained Perl programs with recorded expectations. They are the
reference material the converter is measured against: every entry pairs a
real Perl program with the exact output real perl produces for it, so a
change to the converter can be scored instead of argued about.

Nothing here needs the network, a database, a particular user account, a
particular hostname, a particular timezone, or any file outside the entry's
own directory.

An entry is also run from a copy of its directory rather than from the
directory itself, under a name the entry cannot predict. An entry that prints
its own path, or anything derived from it, records an expectation that only
holds where it was written. Print a property of the path instead of the path:
whether it is absolute, whether two ways of computing it agree, what it looks
like once it has been made relative again.

## Tiers

| Directory | Entries | Purpose |
|---|---|---|
| `tier1/` | 45 | Language fundamentals: scalars, numbers, arrays, hashes, control flow, sorting, strings, output, context, exit status. Small programs, one topic each. |
| `tier2/` | 80 | Script-shaped programs: subroutines, references, nested data, closures, regex, file and stdin I/O, command-line handling, error handling, core modules. |
| `tier3/` | 33 | Full programs of the kind that decide whether the converter is any good: object systems, operator overloading, parsers, schedulers, template engines, process control. Several span more than one file. |
| `tier4/` | 36 | Adversarial constructs. These exist to prove the converter **fails honestly** — see below. |
| `domain/` | 25 | Sysadmin, text-wrangling, bioinformatics and release-engineering scripts, each reading fixtures under its own `files/` directory. |

Tiers 1-3 and `domain/` are conversion targets: the converter should produce
Go that reproduces the recorded output. Tier 4 is different — an entry there
passes when the tool says the right true thing about the file, which may be a
refusal.

## Layout of an entry

Each entry is one directory. Not every file appears in every entry.

```
NN-short-slug/
  input.pl          the program
  cmd               one line of arguments for the program (empty file = no arguments)
  stdin             stdin to feed the program (absent = no stdin)
  files/            input fixtures the program reads by relative path
  expected_stdout   byte-exact stdout
  expected_exit     exit status, a bare integer plus a newline
  allow_stderr      present only when writing to stderr is intentional
  verify.pl         invariant oracle for output that differs run to run
  notes.md          what the entry exercises and what it costs to convert
  expectation.md    tier4 only: what the tool must report about this file
```

A few entries also carry `.pm` modules or a module subdirectory next to
`input.pl`; those are part of the program, not fixtures.

## Running an entry

The contract is exact, and the harness follows it:

1. The working directory is the entry directory, so `files/...` resolves.
2. `cmd` is split into words the way a shell would (quotes and backslashes
   honoured), and those words become the program's arguments. `cmd` does
   **not** contain the script name. It is never handed to a shell.
3. The program is run as `perl input.pl ARGS...`.
4. `stdin` is fed on standard input when the file exists; otherwise stdin is
   empty.
5. stdout must equal `expected_stdout` byte for byte, and the exit status must
   equal the integer in `expected_exit`.
6. stderr must be empty unless the entry has an `allow_stderr` marker.

An entry must leave its own directory exactly as it found it. Entries that
create files clean up after themselves or write under a temporary directory.

## How the expectations were produced

`expected_stdout` and `expected_exit` are captured from a run of real perl —
5.42.2 on x86_64 Linux — never written by hand. Each program is run twice and
the two runs compared byte for byte, which is what makes the recorded output a
fact rather than a guess.

Everything that could differ between two runs or two machines has been driven
out of the programs themselves: timestamps come from a pinned epoch or a
command-line argument, hash keys reach the output only through `sort`, the
time locale is pinned where month and day names are printed, environment
variables that would change behaviour are cleared by the program that reads
them, and no program depends on where it is checked out.

One entry is exempt: `tier4/25-hash-order` prints hash keys in Perl's
per-process random order on purpose, so it has an `expected_exit` but no
`expected_stdout`. It carries a `verify.pl` instead and is checked against
invariants rather than a byte diff.

## verify.pl: the invariant oracle

An entry whose output is legitimately different every run cannot record an
`expected_stdout`, and without one nothing about its behaviour could be
checked at all. Such an entry supplies a `verify.pl` beside its program, and
the harness judges each run by it instead of by a byte diff. Like `cmd` and
`stdin` the file's presence is the switch; nothing matches on the entry's
name.

The contract:

- `verify.pl` receives one run's stdout on its standard input.
- Checking a generated program, it also receives two arguments: the path to
  the conversion report as JSON, and the path to the generated `main.go`. So
  it can insist the report admits something, or reject a shape of code.
- Checking perl's own output, it receives no arguments and must judge the
  output alone.
- It runs with the working directory the program ran in, so files the
  program wrote are there to inspect.
- Exit 0 is a pass. Anything else is a failure, and the first line the
  oracle printed to stderr becomes the recorded reason.
- Complaints must not quote values that differ run to run, such as the key
  order that made the entry need an oracle at all: the recorded reason has
  to read the same on every run that fails the same way, or the saved
  scorecard changes on its own. Put deterministic checks (the report, the
  generated code) before the ones about the run's own output, name the
  broken invariant, and leave the values to a human rerunning the program.

The harness checks the oracle against perl's own output first; an oracle
that rejects what perl prints marks the entry broken (a corpus note, like
recorded-output drift), never the conversion. Each generated program is then
run several times and every run must satisfy the oracle, because an
invariant that only holds by luck can hold for one run. The oracle replaces
the stdout and stderr comparison but not the exit-status check, which is
still made against what perl's run exited with.

## Tier 4: honest failure

Tier 4 covers constructs a converter cannot translate faithfully, or can only
translate with a change in meaning: string `eval`, symbolic references,
typeglob assignment, `AUTOLOAD`, `tie`, `format`/`write`, prototypes that
change parsing, and the arithmetic and coercion rules where a token-for-token
translation compiles and then quietly gives wrong answers.

Every tier 4 entry has an `expectation.md` stating what the tool must do, in
one of six categories:

- `refuse-file` — decline the whole file, with a diagnostic explaining why.
- `refuse-statement` — convert the rest; replace the construct with a stub
  that panics if reached, and diagnose each site.
- `todo` — emit compilable Go plus a marked TODO at the site and a report
  entry; behaviour knowingly diverges until a human acts.
- `shim` — emit or call a runtime helper that reproduces the Perl semantics
  exactly, and note the shim in the report.
- `approximate` — convert with a documented, reported difference in meaning.
- `convert-verify` — full conversion expected; passes only if the built
  program reproduces `expected_stdout` and `expected_exit`.

`expectation.md` also lists the tripwires: the specific output lines that
separate a correct conversion from a plausible-looking wrong one.

Lines of the form `- report-must-contain:` are machine-checked, and every
one of them must hold for the entry to pass, whatever its category decided:
a report can refuse one construct while silently mistranslating the one the
entry is about, so "the tool reported something" is not the standard. Each
such line is one requirement; the backticked phrases on the line are
alternatives, so a line naming `truth` and `boolean` is satisfied by either
word, matched case-insensitively anywhere in a report entry's text. Prose on
the line outside backticks is for the human reader and never matched, which
is also why a file or line reference on a requirement line must not be
backticked.

## MANIFEST.json

`MANIFEST.json` is the machine-readable index of the corpus and the file the
Go test harness reads. It is a JSON array sorted by tier then entry name, one
object per entry:

| Field | Meaning |
|---|---|
| `tier` | `tier1`, `tier2`, `tier3`, `tier4` or `domain` |
| `name` | entry directory name |
| `path` | entry path relative to the repository root |
| `args` | `cmd` already split into an argument list |
| `has_stdin` | entry has a `stdin` file |
| `has_files` | entry has a `files/` directory |
| `allow_stderr` | stderr output is intentional |
| `expected_exit` | expected exit status |
| `deterministic` | stdout is reproducible run to run |
| `kind` | `convert` for tiers 1-3 and `domain`, `honest-failure` for tier 4 |

Regenerate it whenever an entry is added, removed or renamed, and keep it
sorted; the harness treats it as the list of entries that exist.

It is an index, not an authority. Everything it claims about an entry is
checked against the entry's own files before the entry runs, and the files
win: the arguments come from `cmd`, the exit status from `expected_exit`, and
a row that has fallen behind is reported as a corpus note rather than acted
on. So a stale manifest costs you a note, not a wrong result, but the note is
there to be fixed.

## Adding an entry

```sh
scripts/corpus-add.sh tier2 my-new-case
```

That creates the directory with a stub `input.pl` and an empty `cmd`, and
records `expected_stdout` and `expected_exit` by running perl. Then:

1. Write the real program in `input.pl`. Keep `use strict; use warnings;` and
   keep it to core modules.
2. Put any input files under `files/` and read them by relative path. Put
   arguments in `cmd` and any standard input in `stdin`.
3. Make it deterministic. Sort hash keys before printing them, take "now"
   from an argument or a pinned constant, never read `/etc`, `/proc` or the
   user's home directory, and never print a path that depends on where the
   repository lives.
4. Re-record the expectations with
   `make corpus-record TIER=<tier> NAME=<name>`. It runs the program from
   inside its own directory, with its own `cmd` and `stdin`, twice, and
   refuses to record an entry whose two runs disagree.
5. If it writes to stderr on purpose, create an empty `allow_stderr` file.
6. Write `notes.md`: what the entry exercises, and what makes it hard to
   convert.
7. Update the entry's row in `MANIFEST.json` and add a row for it to the
   tier's `INDEX.md`.
8. Run `make score ARGS="-tier <tier> -only <name>"` and read the corpus
   notes: that is where a manifest row that no longer matches shows up.

An entry that cannot be made deterministic does not belong in tiers 1-3 or
`domain`. If the non-determinism is the point, it belongs in `tier4/` with an
`expectation.md` saying so and a `verify.pl` checking the invariants that
replace the byte diff, because an entry with neither an `expected_stdout`
nor an oracle cannot be checked at all and the scorecard says so in a note.
