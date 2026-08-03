# 43 - File::Spec's class methods and the Cwd pair

## What this exercises
Path arithmetic done the portable way. `File::Spec` names a class only so that
each operating system can subclass it, so every call here is a class method
with no object behind it, and the Go answer is one package of plain functions.

Nothing absolute is printed. What is printed is either a relative path or a
property of an absolute one, so the transcript is the same wherever the tree
lives and whatever the directory is called.

## Perl constructs
- `File::Spec->catfile` and `catdir`, including `catfile(@parts)` where the
  components arrive as one array rather than as separate arguments
- `File::Spec->splitpath` returning **three** values (volume, directories,
  file) and `splitdir` returning the components
- `File::Spec->canonpath`, which tidies `//` and `/./` but deliberately does
  **not** resolve `..`
- `File::Spec->file_name_is_absolute`, `curdir`, `updir`
- `File::Spec->rel2abs` with and without a base, and `abs2rel` including the
  case where the answer has to walk up with `..`
- `Cwd::getcwd`
- `basename($path, @suffixes)`, whose suffixes are taken **literally**, beside
  `fileparse($path, @patterns)`, whose suffixes are **patterns** and which
  strips every one of them that matches in turn
- `dirname('logs/')`, which is `.` and not `logs`, because dirname drops
  trailing separators before it looks for the parent

## Go concepts a converter must teach
- `catfile` and `catdir` are both `filepath.Join`, which cleans as it builds:
  an empty component disappears and `..` is resolved textually. `File::Spec`
  does neither, so the two part company on input that is already untidy.
- `filepath.Split` returns the two parts a caller normally wants and says
  nothing about a volume, which `filepath.VolumeName` answers separately and
  which is empty on everything but Windows.
- `filepath.Clean` is textual and does not follow symbolic links, so a path
  that walks up out of a symlinked directory lands somewhere else than
  following the links would. `filepath.EvalSymlinks` is the version that
  touches the disk.
- `filepath.Abs` is `rel2abs` with no base: it joins onto the working
  directory and cleans, and returns an error because reading the working
  directory can fail. `filepath.Rel` is `abs2rel`, and it errors rather than
  guessing when one path is absolute and the other is not.
- `os.Getwd` returns an error alongside the directory, because the directory a
  process is sitting in can be removed while it runs.
- `filepath.Base` has no notion of a suffix to strip and `filepath.Ext` takes
  everything after the last dot, so `archive.tar.gz` needs a rule written out
  rather than a call.
