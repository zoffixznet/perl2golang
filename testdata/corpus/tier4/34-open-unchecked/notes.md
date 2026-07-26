# 34-open-unchecked: open() failure not checked

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
`open(my $fh, '<', ...)` on a nonexistent path (line 9) FAILS by returning
false and setting `$!`; nothing throws. The script continues: `<$fh>` in list
context returns the empty list (line 12), in scalar context undef (line 15),
and the process exits 0 — reporting success to its caller. Warnings that
would normally appear on stderr are suppressed (line 7), as real legacy
scripts routinely do.

## Why the naive conversion is subtly wrong
Go's `os.Open` returns an error the generated code must put SOMEWHERE. Two
tempting-but-wrong translations: (1) `if err != nil { log.Fatal(err) }` —
the converted program now dies with exit 1 where the Perl kept going and
exited 0; that changes pipeline/cron behaviour, sometimes for the "better",
but it is NOT the program the user asked to convert. (2) Ignoring the error
and using the nil `*os.File` — reads panic, where Perl quietly returned
nothing.

## What the converter should do
- Category: **convert-verify**: model Perl filehandles as a wrapper that can
  hold a failed state. `open` sets the wrapper's error and returns false;
  reads on a failed/unopened handle return empty/undef without panicking;
  `$!` maps to the saved error's message (errno text). Behaviour matches
  Perl exactly — exit 0 and all.
- MANDATORY report entry per unchecked open: the tool must point out that
  the failure is unhandled, because preserving this behaviour is faithful
  but almost never what the user ultimately wants. The diagnostic must
  distinguish "converted faithfully" from "endorsed".
- Forbidden: inserting a fatal error check the original did not have, or
  letting a nil handle panic.

## Ideal diagnostic (word for word)
> input.pl:9: warning P2G-W410: the result of open() is not checked (Perl
> continues with an unopened handle; reads return nothing; the script exits
> 0). Converted faithfully via the perlrt filehandle wrapper. NOTE: this
> silently swallows I/O failure — after verifying the conversion, consider
> adding explicit error handling in the Go code.

## What a human should do instead
In the Perl source, `open(...) or die "...: $!"` FIRST, then convert — the
conversion of checked opens is clean idiomatic Go. Unchecked-open scripts
should be fixed before, not during, translation.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0 — the exit code IS part of the fixture):
`open ok? no (No such file or directory)`, `lines read: 0`,
`first line: undef`, `script keeps running and exits 0`. The `$!` text
`No such file or directory` is the standard errno string for ENOENT; the
converted program must produce the same text for the same failure.
