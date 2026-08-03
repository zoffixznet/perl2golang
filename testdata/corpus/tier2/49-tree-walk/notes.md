# 49 - building, walking and removing a tree

## What this exercises
The three filesystem modules a script reaches for together: File::Temp for a
scratch directory, File::Path to build a tree inside it, and File::Find to
walk it. Every path is under a temporary directory whose name differs on every
run, so nothing prints a path: what prints is what was made, what the walk
found, and what is left afterwards.

## Perl constructs
- `tempdir(CLEANUP => 1)` and `tempfile('work-XXXX', DIR => ..., SUFFIX =>
  ...)`, which returns both a handle and a name
- `print {$fh} $text`, the braced handle form
- `make_path(@dirs)` returning the directories it created, and returning
  nothing on a second pass over the same tree
- `File::Spec->catfile($root, split m{/}, $rel)`, where a list of components
  arrives in the middle of the argument list
- `find(sub {...}, $root)` with `-d $_`, `-f $_`, `-s $_` and
  `$File::Find::name`
- `$File::Find::prune = 1` to skip a subtree
- `remove_tree` returning how many entries it removed

## Go concepts a converter must teach
- `os.MkdirTemp` and `os.CreateTemp` take a pattern with a `*` where the
  random part goes, and neither cleans up: `defer os.RemoveAll(dir)` is where
  a Go program says when the tree goes.
- `os.MkdirAll` and `os.RemoveAll` report an error and nothing else, so a
  script that printed a count has to work the count out itself.
- `filepath.Join` takes its components as separate arguments and a slice only
  with `...`, so a list arriving in the middle has to be gathered first.
- `filepath.WalkDir` passes the path and an `fs.DirEntry` to the callback
  instead of setting globals and changing directory, visits in lexical order,
  and reads pruning as a returned `fs.SkipDir`.
- Returning any other error from the callback stops the walk and hands that
  error back to the caller, which is how a walk reports a problem rather than
  carrying on in silence.
