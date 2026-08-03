# 44 - File::Temp and File::Path, the neighbours still refused

## What this exercises
Building a scratch tree and taking it down again: the other half of what a
script reaches for when it touches the filesystem, and the half the converter
does not yet handle. `File::Spec` and the file tests around it convert; the
`tempdir`, `tempfile`, `make_path` and `remove_tree` calls do not, so this
entry exists to be a recorded target rather than a pass.

Every path is under a temporary directory whose name differs on every run, so
nothing prints a path. What prints is how many things were made, whether they
are there, and what is left when the tree comes down.

## Perl constructs
- `tempdir(CLEANUP => 1)`, which removes the tree when the process ends
- `tempfile(DIR => ..., SUFFIX => ...)` returning **both** a handle and a name
- `make_path`, which returns the list of directories it actually created, so
  a second call over the same tree returns nothing
- `remove_tree`, which returns a count of everything it removed, files
  included
- `opendir`/`readdir` filtered against `.` and `..`

## Go concepts a converter must teach
- `os.MkdirTemp` and `os.CreateTemp` are the two constructors, and neither
  cleans up by itself: `defer os.RemoveAll(dir)` is the counterpart of
  `CLEANUP => 1`, and inside a test `t.TempDir()` does it for you.
- `os.CreateTemp` returns an `*os.File`, and the name is `f.Name()` rather
  than a second return value. Its pattern puts the random part where a `*`
  appears, which is how a suffix is asked for.
- `os.MkdirAll` is `make_path` and is idempotent, but it reports only an
  error, not what it created. A count of new directories has to be worked out
  before the call, which is a good example of a Perl return value that carries
  more than Go's does.
- `os.RemoveAll` is `remove_tree` and likewise reports only an error.
- Permissions are an argument, written in octal because that is what they
  are, rather than a global umask side effect.
- `os.ReadDir` never includes `.` or `..`, so the filter that removes them has
  nothing to do and should not be carried over.
