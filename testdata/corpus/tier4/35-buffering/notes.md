# 35-buffering: STDOUT/STDERR interleaving under redirection

Group: **C - convertible, but the naive conversion is subtly wrong**

## Construct
Five prints alternating STDOUT and STDERR (lines 9-13). Perl (via stdio
semantics) BLOCK-buffers STDOUT when it is a file or pipe and leaves STDERR
unbuffered. Redirected to one destination (`> file 2>&1`), the observed
order is: both `err` lines first, then all three `out` lines - the stdout
buffer flushes once at exit. On a terminal the lines would alternate
(line-buffered stdout).

## Why the naive conversion is subtly wrong
Go's `fmt.Println`/`os.Stdout` writes are UNBUFFERED: the converted program
emits the five lines in source order even when redirected. That is arguably
"better", but it is observably different - log-scraping scripts, test
harnesses diffing combined output, and anything parsing interleaved
`command 2>&1` output will see a changed stream. The reverse trap also
exists: a converter that wraps stdout in `bufio.Writer` for performance and
forgets to flush on exit DROPS output entirely.

## What the converter should do
- Category: **convert-verify** with a declared I/O model:
  - Default recommendation: reproduce Perl's model - buffered stdout
    (bufio) flushed on normal exit and before any operation that Perl would
    flush around (reading from a terminal, `$| = 1` sites, exec), unbuffered
    stderr. Then the redirected interleaving matches `expected_stdout`.
  - If the converter chooses unbuffered stdout instead, it MUST say so in
    the report for any program that writes to both streams, because the
    merged-stream order changes.
  - Either way, `$|`/autoflush constructs (not in this file) must map onto
    the model.
- Non-negotiable: never lose output; flush-on-exit must cover exit paths
  including panics that reach the top level.

## Ideal diagnostic (word for word)
> input.pl:9: note P2G-W411: this program interleaves STDOUT and STDERR.
> Perl block-buffers STDOUT to files/pipes (stderr lines appear first under
> '2>&1' redirection); the converted Go uses the same buffered model and
> reproduces that order. If you prefer source-order output, enable the
> unbuffered-stdout option - output content is identical, interleaving is
> not.

## What a human should do instead
Stop relying on stream interleaving: tag lines by stream, or send both to
one explicitly synchronized writer. When comparing legacy behaviour, always
compare stdout and stderr separately as well as merged.

## Observed with perl 5.42.2 (x86_64-linux)
Captured with the two streams kept apart, which is what `expected_stdout`
holds and what the harness diffs:

- stdout: `out 1`, `out 2`, `out 3`
- stderr: `err 1`, `err 2` (hence the `allow_stderr` marker)
- exit 0

The trap itself only shows up when the streams are merged. Running
`perl input.pl > combined 2>&1` yields `err 1`, `err 2`, `out 1`, `out 2`,
`out 3` - stderr is unbuffered and gets out first, while the three stdout
lines sit in a block buffer until exit. A converter that reproduces Perl's
I/O model reproduces that merged order; one that writes stdout unbuffered
emits the five lines in source order instead. Both pass the stdout diff
above, so the merged order has to be checked by running the converted
program under the same redirection, not by comparing `expected_stdout`.
