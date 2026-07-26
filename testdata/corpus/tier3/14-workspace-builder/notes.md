# 14-workspace-builder

Builds a project skeleton inside a File::Temp sandbox with make_path,
writes files, audits them with File::Find, and probes error paths.
Deterministic because only workspace-relative paths reach stdout.

## Constructs exercised
- `File::Temp::tempdir( CLEANUP => 1 )` and `tempfile` with a template
  (`scratch-XXXX`), `DIR`, `UNLINK`; the random name is *verified by regex*
  but never printed
- `File::Path::make_path` multi-dir creation, its create-count in scalar
  context, idempotent second call, and the `{ error => \$err }` protocol
  (arrayref-of-hashrefs error reporting instead of dying)
- writing files through `print {$fh} ...` (block filehandle syntax),
  checked `close`
- `%ENV` app-name override with fixed default (`SKEL_APP_NAME // 'shipit'`)
- `File::Spec->catfile( $work, $app, split m{/}, $rel )` -- splitting a
  logical path into components inline
- File::Find audit excluding the randomly named scratch file
- file test operators `-f -d -s -z -x` plus `-s _` stat reuse
- `$pkg = $1, last if /.../;` comma-operator + conditional `last`
- heredoc-free embedded file bodies with escaped `\n` / `\"`

## Conversion challenges
- tempdir/tempfile: Go's os.MkdirTemp/os.CreateTemp differ in template
  syntax (`XXXX` vs `*`) and there is no UNLINK/CLEANUP -- defer os.RemoveAll
- make_path's three behaviors (count, idempotence, error-collector) map to
  os.MkdirAll which returns nil on existing dirs and error otherwise --
  the "created N" count needs manual tracking
- `make_path` through an existing FILE: MkdirAll gives ENOTDIR; the
  converter must surface it as data, not a crash
- filtering nondeterministic names out of output (scratch-XXXX) while still
  asserting their shape
- `-z` (empty file) and `-x` (exec bit) tests -> os.Stat + mode bits

## Go teaching opportunities
- defer-based cleanup, os.WriteFile, io/fs walking, error values as data
