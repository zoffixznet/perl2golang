# 50 - what a script asks about a file, which does not convert yet

## What this exercises
The metadata half of filesystem work: mode, size, link count, symbolic links
and timestamps. Perl answers all of it with `stat` and the file test
operators; Go answers it with `os.Stat` and questions about what comes back.
`stat`, `chmod`, `symlink`, `readlink` and `utime` have no rule yet, so this
entry is a recorded target.

## Perl constructs
- `stat` in list context, returning thirteen fields read by position
- `chmod 0640, $file` and reading the mode back out of `$st[2] & 07777`
- `-r`, `-w`, `-x`, `-f`, `-l`, `-e`, `-s`
- `symlink` and `readlink`
- `utime` setting fixed access and modification times
- `rename` and `unlink`, which report success rather than raising

## Go concepts a converter must teach
- `os.Stat` returns an `fs.FileInfo` and an error, and the fields are methods
  on it: `fi.Size()`, `fi.Mode()`, `fi.ModTime()`, `fi.IsDir()`. There is no
  list of thirteen numbers and no need to remember which index is which.
- `fi.Mode().Perm()` is the permission bits, written in Go as `0o640`, and
  `fi.Mode()&fs.ModeSymlink` is how the type bits are asked about.
- `os.Lstat` is `lstat` and is what `-l` needs: `os.Stat` follows the link, so
  `-l` and `-f` disagree only when you use the right one of the two.
- `os.Chmod`, `os.Symlink`, `os.Readlink`, `os.Chtimes`, `os.Rename` and
  `os.Remove` are the operations, and every one of them returns an error
  rather than a truth value. `os.Remove` on a missing file returns an error
  where `unlink` returned zero.
- `-r` and `-w` have no counterpart on purpose: the honest answer is to open
  the file and handle the failure, because the permission bits do not account
  for ownership, capabilities or a read-only filesystem.
