# 09-log-tail-state

**Domain:** log analysis. An incremental log follower ("tail since last
run"): loads a key=value state file (byte offset + MD5 signature of the
file's first 64 bytes), detects rotation (signature changed) and
copytruncate-style truncation (offset > size), scans only the new region,
alerts on ERROR/FATAL lines, and prints the updated state block. Exit 1
because matches were found. This corpus copy prints the new state to
stdout rather than rewriting it, so the fixture tree never mutates.

## Constructs exercised
- Byte-level file I/O: `binmode`, `read` into a buffer, `-s` file size,
  `seek` to a stored offset, `Digest::MD5::md5_hex` over raw bytes.
- The partial-last-line guard (`last unless $line =~ /\n\z/`) -- offset
  advances only over complete lines, tracked via `length $line` in bytes.
- A state file parsed into a hash with defaults merged underneath
  (`%state = (defaults); ... $state{$1}=$2`).
- A four-way decision ladder assigning `($start, $why)` as a two-element
  list -- rotation / truncation / no growth / resume.
- User-suppliable pattern compiled with `qr/$pattern/` falling back to a
  default `qr/\b(?:ERROR|FATAL)\b/`.
- Fixed-order state emission (`for qw(file offset sig siglen runs)`) --
  the comment explains the deploy pipeline diffs this block.

## Conversion challenges
- `length $line` is a byte count here (no encoding layer); Go's
  `len(line)` agrees, but a converter inserting `bufio.Scanner` loses the
  trailing newline *and* the raw byte count -- it must use `ReadString`
  or track offsets manually. This is the central trap of the entry.
- `read $lf, my $head, $n` declares the buffer inline in the call -- an
  in-arguments `my` declaration converters often mishandle.
- `qr//` from a runtime string: Go must `regexp.Compile` user input and
  decide what to do with Perl-specific syntax (here the default pattern
  is RE2-clean, but the design point stands).
- Numeric context on state values read as strings (`$state{offset} >
  $size`, `$state{runs} + 1`): classic string/number duality across a
  persistence boundary.
- `md5_hex($head // '')` -- defined-or on a possibly-failed `read`.
- The state record is a struct candidate (`file/offset/sig/siglen/runs`)
  with a serialisation order contract.
