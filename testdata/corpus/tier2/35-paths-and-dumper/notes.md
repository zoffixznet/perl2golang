# 35 - File::Basename, Cwd, File::Spec and Data::Dumper

## What this exercises
Path manipulation and structure dumping - the two utility layers almost every
script pulls in. Absolute paths are converted back to relative before printing
so the output does not depend on where the tree lives.

## Perl constructs
- `basename`/`dirname` over a mix of relative, absolute, dotted and
  trailing-slash paths (note `dirname('relative.txt')` is `'.'` and
  `basename('trailing/slash/')` is `'slash'`)
- `fileparse($path, qr/\.[^.]*/)` returning **three** values (name, dirs,
  suffix) with a regex suffix pattern - `archive.tar.gz` splits as
  `archive.tar` + `.gz`, not `archive` + `.tar.gz`
- `File::Spec->catfile` / `catdir` / `splitpath` / `splitdir`
- `getcwd()` and `abs_path()` from `Cwd`, with `abs_path` returning `undef` for
  a path that does not exist. Neither is ever printed: what is printed is a
  property of the answer (is it absolute, does `getcwd()` agree with
  `abs_path('.')`), so the transcript is the same in any directory under any
  name.
- `File::Spec->abs2rel($abs, $cwd)` to make output location-independent
- `$Data::Dumper::Sortkeys = 1` (**mandatory** for reproducible output),
  `$Data::Dumper::Indent` (2 default, 1 compact, 0 one-line) and `Terse`
- `local $Data::Dumper::Indent = 0;` inside a block - dynamic scoping of a
  package global
- `Dumper($x)` naming its output `$VAR1`, and `Dumper($a, $b)` producing
  `$VAR1`/`$VAR2`
- `undef` inside a dumped structure
- **Dumper as a deep-comparison tool**: `Dumper($a) eq Dumper($b)`

## Go concepts a converter must teach
- `path/filepath` covers most of this: `filepath.Base`, `filepath.Dir`,
  `filepath.Join`, `filepath.Split`, `filepath.Abs`, `filepath.Rel`. The edge
  cases differ though - `filepath.Base("trailing/slash/")` is `"slash"`
  (matches), but `filepath.Dir("relative.txt")` is `"."` (also matches), while
  `filepath.Split` returns dir+file rather than Perl's three-value fileparse.
- `fileparse`'s suffix-pattern argument has no Go equivalent; it becomes
  `filepath.Ext` plus `strings.TrimSuffix`, and the `.tar.gz` case shows why a
  naive "strip everything after the first dot" is wrong.
- `abs_path` on a missing file returns `undef` in Perl; `filepath.Abs` in Go
  succeeds without touching the filesystem, and `filepath.EvalSymlinks` errors.
  Different failure semantics.
- `Data::Dumper` has no direct equivalent. Options: `%#v` (Go syntax, but map
  order is randomised until Go 1.12+ which sorts map keys in fmt - worth
  knowing), `encoding/json` with `MarshalIndent` (sorts keys, but loses type
  info and cannot represent cycles), or a third-party pretty-printer.
- **`Sortkeys` and `Indent` are dynamically-scoped globals.** `local` has no Go
  equivalent; the converter must thread the formatting options through as
  explicit arguments.
- Dumper-as-deep-compare becomes `reflect.DeepEqual`, which is both faster and
  more correct.
