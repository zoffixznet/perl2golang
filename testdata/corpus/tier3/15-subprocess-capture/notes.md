# 15-subprocess-capture

Process management using `$^X` (the running perl) as the only external
command, so results are identical on any machine with perl installed.

## Constructs exercised
- backticks in scalar context (byte/newline counting via `tr/\n//`) and in
  list context (`chomp @lines`), with shell single-quoting protecting `$_`
  from BOTH Perl interpolation and the shell
- `open $fh, '-|', LIST` read pipe (list form, no shell) and
  `open $fh, '|-', LIST` write pipe; child communicates back through a
  tempdir file whose path is passed as an argv
- `close $fh` on a pipe setting `$?`; `$? >> 8` exit-code decoding,
  `$? & 127` signal, `$? & 128` core-dump bit
- `system()` list form: return value vs `$?`, nonzero exit (42), a child
  `die` producing exit 255 (with `$! = 0` so errno can't leak in)
- shell redirection inside backticks (`2>&1 1>/dev/null`) to capture stderr
  ONLY
- `$?` persistence: saved copy survives the next child overwriting `$?`
- `$^X` as a portable path to the interpreter (never printed -- machine
  independence is part of the entry's design)

## Conversion challenges
- backticks/system/open-pipe triple: all become os/exec, but the semantics
  differ (backticks imply /bin/sh, list forms do not) -- converter must
  know which calls need a shell and which must avoid one
- `$?` is global mutable state written by four different constructs; Go
  returns per-call error values (`*exec.ExitError`), so the "saved vs
  current" dance restructures completely
- `>> 8` decoding vs Go's ExitCode(); signal/core bits via
  syscall.WaitStatus
- write-pipe + argv-file communication pattern -> exec.Cmd with StdinPipe
- capturing stderr only requires wiring Stdout to io.Discard, not `2>&1`

## Go teaching opportunities
- exec.Command, CombinedOutput vs Output, ExitError inspection, StdinPipe;
  why Go has no implicit shell and what that fixes
