# 22 - file tests and directory walking

## What this exercises
`opendir`/`readdir` with sorted output, the `-e`/`-f`/`-d`/`-s`/`-r` file test
operators, a recursive descent, and the `_` filehandle cache.

## Perl constructs
- `opendir my $dh, $root or die`, `readdir $dh`, `closedir $dh or die`
- `sort grep { $_ ne '.' && $_ ne '..' } readdir $dh` - readdir returns `.` and
  `..` and in **unspecified order**, so both the filter and the sort are
  mandatory for deterministic output
- `grep { !/^\.\.?$/ }` as the regex form of the same filter
- file tests `-d`, `-f`, `-s`, `-e`, `-r` used as booleans and as values
  (`-s` returns the size)
- nested ternary chains selecting a type/note string
- recursive `walk` building `%by_ext`, an autovivified hash of arrays
- `my ($ext) = $kid =~ /\.(\w+)$/;` with a `defined` fallback
- **the `_` filehandle**: `if (-e $target && -f _ && -s _)` reuses the stat
  buffer from the previous test instead of re-stat-ing
- string path building with `"$dir/$kid"`
- `die "$0: ... \n" unless -d $root;` precondition check

## Go concepts a converter must teach
- `os.ReadDir` already excludes `.` and `..` and returns entries sorted by
  filename, so the Perl filter+sort collapses to nothing - but the converter
  must *know* that to avoid emitting dead code, and must keep the sort when the
  Perl code sorts on something other than name.
- File tests map to `os.Stat`: `-e` is "err == nil", `-f` is
  `info.Mode().IsRegular()`, `-d` is `IsDir()`, `-s` is `Size() > 0` or the
  size itself, `-r` needs a permission check or a trial open.
- **`-s` is overloaded**: boolean in a condition, size in a numeric position.
  Context again.
- The `_` cache is a manual optimisation; in Go the converter should hoist a
  single `os.Stat` into a variable, which is both faster and clearer.
- Recursive walking is `filepath.WalkDir` - recognising the hand-rolled
  recursion and replacing it is a nice win, but WalkDir's traversal order
  differs from a hand-rolled `sort`ed descent, so output ordering must be
  checked.
- Perl silently returns `undef` from `-s` on a missing file; Go returns an
  error that must be handled at every call.
- Path joining should become `filepath.Join`, not string concatenation.
