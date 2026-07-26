# 13-fs-walker

Directory auditor over a checked-in fixture tree: File::Find with sorted
traversal and pruning, File::Spec/File::Basename path surgery, rollups.

## Constructs exercised
- `File::Find::find` with an options hash: `preprocess => sub { sort @_ }`
  for deterministic order, `wanted` closure mutating outer lexicals
- pruning via `$File::Find::prune = 1` on dot-directories, regex
  `/^\.(?!\.?$)/` (negative lookahead so `.` and `..` survive)
- the `-f _` cached-stat filehandle reuse after `-d $_`
- `$File::Find::name` vs `$_` (full path vs basename inside wanted)
- `File::Spec`: `catdir`, `catfile`, `abs2rel`, `splitdir`, `splitpath`,
  `canonpath`, `updir`, `file_name_is_absolute`
- `File::Basename::fileparse` with a suffix regex `qr/\.[^.]+$/`
- `-s` file size on fixed fixtures; depth-indented tree via `'  ' x $depth`
- multi-key sorts (bytes desc then ext asc; size desc then name asc)
- `(sort ...)[0..2]` slice for top-N

## Gotchas encoded in expected output (converter must reproduce)
- `canonpath` does NOT collapse `..` -- output keeps `src/util/../lib`
- File::Find calls `wanted` for the root dir itself (5 dirs counted)
- pruned directory's file (cache.bin, 19 bytes) is absent from every total

## Conversion challenges
- File::Find's callback-with-globals model (`$File::Find::name`, `prune`,
  the magic `_` stat cache, chdir behavior) vs Go's filepath.WalkDir with
  return-value-based pruning (fs.SkipDir)
- closures over accumulator variables -> methods on a collector struct
- negative lookahead prune regex (RE2 has no lookahead -- must rewrite)
- reproducing File::Spec quirks exactly (no `..` collapse) when Go's
  filepath.Clean *does* collapse them -- a subtle output-fidelity trap

## Go teaching opportunities
- filepath.WalkDir, filepath.Rel/Join/Ext, io/fs abstractions, why sorted
  traversal is default in Go's WalkDir but not Perl's find
