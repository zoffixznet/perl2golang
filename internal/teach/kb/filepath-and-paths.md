---
id: filepath-and-paths
title: Paths are string functions, and file tests return errors
tags: [idiom, files, paths, stdlib]
perl_triggers: [file-basename, basename, dirname, fileparse, file-spec, catfile, catdir, splitpath, splitdir, canonpath, rel2abs, abs2rel, cwd, getcwd, abs-path, file-path, mkpath, make-path, file-find, find, glob, file-test-operators, opendir, readdir, unlink, rename, mkdir, file-temp]
severity: info
prerequisites: [errors-are-values, strings-are-bytes]
---

`File::Basename`, `File::Spec`, `File::Path`, and `File::Find` collapse into two standard-library packages: `path/filepath` for anything that touches the filesystem, and `os` for the operations themselves. The API is smaller than the CPAN set it replaces and there is nothing to install. The one decision to get right on day one is `path` versus `path/filepath`: `path` is for slash-separated strings that are not filesystem paths (URLs, archive members, `io/fs` names), `filepath` is for the operating system. Importing the wrong one compiles and mostly works, which is exactly why it is worth learning once.

The other habit to drop is the file test operator. `-e`, `-d`, `-f`, and `-s` were single characters returning a boolean; in Go you call `os.Stat`, which returns information *and* an error, and you distinguish "not there" from "there but unreadable" instead of collapsing both into false.

## The Perl you know

```perl
use File::Basename qw(basename dirname fileparse);
use File::Spec;
use File::Path qw(make_path);
use File::Find;

my $name = basename $path;
my $dir  = dirname $path;
my ($base, $parent, $suffix) = fileparse($path, qr/\.[^.]*/);
my $full = File::Spec->catfile($dir, 'out', 'report.txt');

make_path("$dir/out");
my @logs = glob "$dir/*.log";

if (-d $dir && -f $path && -s _) { ... }

find(sub { push @found, $File::Find::name if -f && /\.log$/ }, $dir);
```

## The Go you write

```go
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	p := "/var/log/nginx/access.log.1"

	// basename, dirname, and the suffix, which is not what you may expect.
	fmt.Println(filepath.Base(p), filepath.Dir(p), filepath.Ext(p))
	fmt.Println(filepath.Base("/var/log/"), filepath.Dir("access.log"), filepath.Base(""))

	// fileparse with a suffix list: Ext plus TrimSuffix.
	name := filepath.Base(p)
	fmt.Println(strings.TrimSuffix(name, filepath.Ext(name)))

	// catfile is Join, and Join cleans as it goes: empty parts vanish,
	// .. is resolved textually.
	fmt.Println(filepath.Join("/var", "log", "", "nginx", "..", "app.log"))
	fmt.Println(filepath.Clean("/var/log/../log/./nginx/"))

	rel, err := filepath.Rel("/var/log", "/var/log/nginx/access.log")
	fmt.Println(rel, err)
	// Rel walks up when it has to, and refuses when there is no answer at
	// all, which is what happens when only one of the two is absolute.
	fmt.Println(filepath.Rel("/srv/data/inner", "/srv/data/notes.txt"))
	fmt.Println(filepath.Rel("/var/log", "var/log"))
	fmt.Println(filepath.IsAbs("etc/hosts"), filepath.IsAbs("/etc/hosts"))
	fmt.Println(filepath.SplitList("/usr/bin:/bin")) // the PATH splitter

	// Dir does not drop a trailing separator first, and dirname does.
	fmt.Println(filepath.Dir("logs/"), filepath.Dir(filepath.Clean("logs/")))
	// Split is the two-value answer; there is no third part outside Windows.
	fmt.Println(filepath.Split("var/log/app.log"))
	fmt.Println(strings.Split("var/log/app.log", "/"))

	// A scratch tree to walk. os.RemoveAll in a defer is the Go equivalent
	// of File::Temp's cleanup on scope exit.
	dir, err := os.MkdirTemp("", "demo")
	if err != nil {
		fmt.Println("mkdtemp:", err)
		return
	}
	defer os.RemoveAll(dir)

	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		fmt.Println("mkdir:", err)
		return
	}
	os.WriteFile(filepath.Join(dir, "a", "one.txt"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "b", "two.log"), []byte("hi"), 0o644)

	// File::Find, with the callback returning errors instead of setting globals.
	var found []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // an unreadable directory is reported, not ignored
		}
		if d.IsDir() && d.Name() == "b" {
			return fs.SkipDir // prune this subtree and keep going
		}
		if !d.IsDir() {
			found = append(found, filepath.Base(path))
		}
		return nil
	})
	fmt.Println(found, err)

	matches, _ := filepath.Glob(filepath.Join(dir, "a", "*.txt"))
	fmt.Println(len(matches), filepath.Base(matches[0]))

	// The file tests, spelled out.
	fi, err := os.Stat(filepath.Join(dir, "a"))
	fmt.Println(fi.IsDir(), fi.Mode().Perm(), err)
	_, err = os.Stat(filepath.Join(dir, "nope"))
	fmt.Println(errors.Is(err, fs.ErrNotExist))

	// Abs is textual and answers for a path that is not there; EvalSymlinks
	// consults the disk and is what makes a missing path detectable. That
	// pair is abs_path, and the two halves fail differently.
	missing := filepath.Join(dir, "nope", "none.txt")
	abs, absErr := filepath.Abs(missing)
	_, evalErr := filepath.EvalSymlinks(missing)
	fmt.Println(abs == missing, absErr, evalErr != nil)
}
```

```
access.log.1 /var/log/nginx .1
log . .
access.log
/var/log/app.log
/var/log/nginx
nginx/access.log <nil>
../notes.txt <nil>
 Rel: can't make var/log relative to /var/log
false true
[/usr/bin /bin]
logs .
var/log/ app.log
[var log app.log]
[one.txt] <nil>
1 one.txt
true -rwxr-xr-x <nil>
true
true <nil> true
```

Note `filepath.Ext` on `access.log.1`: it returns `.1`, the text after the *last* dot, with no notion of a suffix list. Perl's `fileparse($path, qr/\.log(\.\d+)?/)` could express "the extension I mean"; Go makes you write that rule yourself, usually as a `strings.TrimSuffix` or a small regex.

## The mismatch

`File::Spec` deserves its own paragraph, because the arrow in `File::Spec->catfile` misleads: there is no object, and the class exists only so each operating system can subclass it. Everything it offers is a plain function in `path/filepath`. `catfile` and `catdir` are both `Join`, `canonpath` is `Clean`, `file_name_is_absolute` is `IsAbs`, `rel2abs` is `Abs`, `abs2rel` is `Rel`, `splitdir` is `strings.Split` on the separator, `curdir` and `updir` are the strings `"."` and `".."`, and `tmpdir` is `os.TempDir`. Two of those are not exact. `Join` cleans as it builds and `catfile` does not, so they part company on input that is already untidy: `catfile('a', '', 'b')` is `a//b` and `Join("a", "", "b")` is `a/b`. And `canonpath` deliberately leaves `..` alone where `Clean` resolves it, which is the safer default for the same reason `Clean` is dangerous: neither consults the filesystem, so `..` out of a symlinked directory means one thing textually and another on disk.

`splitpath` is the odd one out, because it returns three values and `filepath.Split` returns two. The missing one is the volume, which `filepath.VolumeName` answers separately and which is empty everywhere but Windows. If you find yourself wanting all three, that is a sign the code was written to be portable to a system it will never run on.

The translation table. `basename`/`dirname` are `filepath.Base`/`filepath.Dir`, with edge cases worth memorising: `Base` strips trailing separators (`/var/log/` gives `log`), and both return `.` rather than an empty string when there is nothing to return. `Dir` does *not* strip a trailing separator first, though, and `dirname` does, so `dirname('logs/')` is `.` while `filepath.Dir("logs/")` is `logs`; wrapping the argument in `Clean` puts them back in step. `basename` also takes a list of suffixes to strip and quotes them, so they are literals, while `fileparse` takes the same list as *patterns* and strips every one that matches in turn: `fileparse('access.log.1', '\.\d+', '\.log')` comes back as `access` with a suffix of `.log.1`, having taken two bites. `catfile`/`catdir` are both `filepath.Join`, which cleans the result, so joining with an empty component or a `..` does what you meant. `File::Spec->rel2abs` is `filepath.Abs`, which returns an error because it consults the working directory; `filepath.Rel` computes the other direction. `filepath.Clean` is purely textual and does not touch the disk, which means it resolves `..` *without* following symlinks: use `filepath.EvalSymlinks` when the difference matters, for example before deciding a user-supplied path is inside a directory you control.

`mkdir -p` is `os.MkdirAll` and is idempotent; `unlink` is `os.Remove`, `rmdir` too, and recursive removal is `os.RemoveAll`; `rename` is `os.Rename` and still fails across filesystems. Permissions are an argument, not a global umask side effect, and they are written in octal (`0o755`) because that is what they are. `File::Temp` becomes `os.MkdirTemp`/`os.CreateTemp`, which do not clean up by themselves: pair them with `defer os.RemoveAll(dir)`, or use `t.TempDir()` inside a test, which does (`table-driven-tests`).

`glob` is `filepath.Glob` and is deliberately less capable than the shell: it understands `*`, `?`, `[abc]`, and `\` escapes, and it has no brace expansion, no `~`, no `**`, and no sorting guarantees beyond lexical order. `File::Find` is `filepath.WalkDir`, and the shape is better: no `$File::Find::name` globals, the callback gets the full path and an `fs.DirEntry` (which avoids a `stat` per file, unlike the older `filepath.Walk`), returning `fs.SkipDir` prunes a subtree, and returning any other error stops the walk and surfaces at the call site. The file tests all become `os.Stat` plus a question about the result: `-e` is `err == nil`, `-d` is `fi.IsDir()`, `-f` is `fi.Mode().IsRegular()`, `-s` is `fi.Size() > 0`, and `-r`/`-w` have no direct equivalent because the honest answer is to open the file and handle the failure. There is no `_` cached-stat handle: hold on to the `fs.FileInfo` you already have.

Further reading: https://pkg.go.dev/path/filepath
