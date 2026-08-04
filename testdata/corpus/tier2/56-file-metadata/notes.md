# 56 - asking a file about itself, and changing the answers

## What this exercises
The metadata half of filesystem work: the status list, permission bits,
timestamps, symbolic links, and the calls that move and remove. Everything is
under a temporary directory, and the only numbers printed are ones the script
set itself, so the transcript does not move.

## Perl constructs
- `stat` in list context, read by position, and `( stat $file )[2]` and
  `( stat $file )[8, 9]`, a list slice read for one value and for two
- `chmod` with an octal mode, and reading the mode back out
- `-f`, `-s`, `-w`, `-e` and `-l`, the last of which must not follow the link
- `utime` with fixed times, and `timegm` to produce them
- `symlink` and `readlink`
- `rename` and `unlink`, both of which answer with a count or a truth value

## Go concepts a converter must teach
- `os.Stat` returns an `fs.FileInfo` and an error, and every field is a method
  on it: `Size`, `Mode`, `ModTime`, `IsDir`. Nothing hands back thirteen
  numbers, so nothing has to remember which index is which, and the fields
  that are not portable are simply not there.
- `os.Stat` follows a symbolic link and answers about the target; `os.Lstat`
  does not. That is the whole difference between `-f` and `-l`, and using the
  wrong one of the two is the usual bug.
- Permissions are an argument written in octal, which Go spells `0o644`, and
  `fi.Mode().Perm()` is how they come back.
- `os.Chtimes` takes `time.Time` values rather than whole seconds, which is
  the same information with the unit attached.
- `time.Date` is the inverse of taking a moment apart, and it *normalises* a
  field out of range rather than refusing it: 31 February becomes 3 March
  where `timegm` would have raised.
- `os.Rename`, `os.Symlink` and `os.Remove` all report an error rather than a
  truth value or a count, which is more information in a shape that cannot be
  ignored.
