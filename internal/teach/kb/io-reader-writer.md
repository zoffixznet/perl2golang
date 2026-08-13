---
id: io-reader-writer
title: io.Reader and io.Writer - the universal plumbing
tags: [idiom, interfaces, io, composition]
perl_triggers: [filehandle, open, readline, print-to-filehandle, io-handle, open-to-scalar-ref, stdin, stdout]
severity: info
prerequisites: [implicit-interfaces]
---

Perl unified I/O around the filehandle, and `open my $fh, '<', \$string` let scalars impersonate files. Go unified it around two one-method interfaces - `io.Reader` (`Read(p []byte) (n int, err error)`) and `io.Writer` (`Write(p []byte) (n int, err error)`) - and essentially *everything* speaks them: files, network connections, HTTP bodies, compression, hashing, buffers, string readers. Internalise this and half the standard library becomes guessable; write functions against `*os.File` instead, and you will rewrite them the first time a test or an HTTP handler needs to call them.

## The Perl you know

```perl
open my $fh, '<', \$string or die;      # scalar as filehandle
while (my $line = <$fh>) { $lines++ }

open my $out, '>', \my $captured;       # capture prints for testing
print {$out} "log line\n";
```

## The Go you write

One counting function, three unrelated sources - compiled and run as shown:

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

func countLines(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	for {
		n, err := r.Read(buf)
		count += bytes.Count(buf[:n], []byte{'\n'})
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return count, err
		}
	}
}

func main() {
	// One function, any source: a string, a buffer, a file, a socket.
	n, err := countLines(strings.NewReader("a\nb\nc\n"))
	fmt.Println(n, err)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "log line %d\n", 1) // Fprintf writes to ANY io.Writer
	fmt.Fprintf(&buf, "log line %d\n", 2)
	n, err = countLines(&buf)
	fmt.Println(n, err)

	// io.Copy composes any Reader with any Writer:
	if _, err := io.Copy(os.Stdout, strings.NewReader("copied straight through\n")); err != nil {
		fmt.Println(err)
	}
}
```

```
3 <nil>
2 <nil>
copied straight through
```

The manual `Read` loop above is deliberately educational - note its two quirks: `Read` gives you *up to* `len(buf)` bytes with no line awareness, and `io.EOF` arrives as an error value you check by identity (a sanctioned sentinel - `sentinel-and-custom-errors`). In daily code you reach for the composers instead: `io.Copy`, `io.ReadAll`, `bufio.Scanner` for line-by-line (`bufio-scanner-limit`), `fmt.Fprintf` for formatted writes to anything.

## The mismatch

The translations: `open my $fh, '<', \$string` → `strings.NewReader(s)`; capture-into-scalar → `&bytes.Buffer{}` as the writer, then `buf.String()`; `print {$fh} ...` → `fmt.Fprintf(w, ...)`; `STDOUT`/`STDERR` → `os.Stdout`/`os.Stderr`, which are simply values satisfying `io.Writer` - there is no special print-to-default-filehandle machinery, `fmt.Printf` is literally `Fprintf(os.Stdout, ...)`. The design lesson runs deeper than translation: because the interfaces are tiny, they *stack* - `gzip.NewReader` wraps any reader and is one, `io.TeeReader` splits a stream, `io.MultiWriter` fans out, `http.Request.Body` is a reader - so Perl patterns involving temp files or slurp-transform-write become pipelines of wrapped readers with no intermediate storage. When writing your own code: take `io.Reader`/`io.Writer` parameters at function boundaries and let `main` decide it was a file (`accept-interfaces-return-structs`); implement `Write` on your own type when you want to *be* a destination (a test log sink is four lines). One warning for the transition: there is no line-oriented `<$fh>` operator at this layer - reaching for `Read` to get "a line" is the mistake `bufio` exists to fix.

## Positions and exact lengths

`seek`, `tell` and fixed-length `read` live one interface over: `io.Seeker`, which `*os.File` and even `strings.Reader` satisfy. The whence argument is a named constant - `io.SeekStart`, `io.SeekCurrent`, `io.SeekEnd` are what `0`, `1`, `2` meant - and there is no `Tell` at all: a seek of zero bytes from the current position moves nothing and reports where it stayed. The other habit to swap out is `read($fh, $buf, $n)`: the Go call that means "exactly n bytes" is `io.ReadFull`, because a plain `Read` may return fewer bytes than the buffer holds with *no error*, which works on a small file and quietly breaks on a pipe.

```go
package main

import (
	"fmt"
	"io"
	"strings"
)

func main() {
	r := strings.NewReader("HDR20240601BODY....TRL")

	// An exact-length read is io.ReadFull, not Read.
	tag := make([]byte, 3)
	if _, err := io.ReadFull(r, tag); err != nil {
		fmt.Println("short record:", err)
		return
	}
	fmt.Printf("tag %s\n", tag)

	// tell: a seek of zero bytes moves nothing and reports the position.
	pos, _ := r.Seek(0, io.SeekCurrent)
	fmt.Println("at byte", pos)

	// seek: jump to the trailer, three bytes before the end.
	if _, err := r.Seek(-3, io.SeekEnd); err != nil {
		fmt.Println("seek:", err)
		return
	}
	trailer, _ := io.ReadAll(r)
	fmt.Printf("trailer %s\n", trailer)
}
```

```
tag HDR
at byte 3
trailer TRL
```

One caution when mixing layers: a position belongs to the underlying file, and a `bufio.Scanner` or `bufio.Reader` reads ahead of what you have consumed. Seek the `*os.File` first, then wrap it - seeking under an active buffered reader leaves the buffer describing bytes from the old position.

The same read-ahead is why a program that reads one handle in several shapes needs **one buffered reader per handle, made once and used everywhere**. Perl let you write `my $header = <$fh>` and then `while (<$fh>)` and then a slurp of the rest, and every read continued exactly where the last stopped, because the position was the handle's own. Wrap the same `*os.File` in a fresh `bufio.Reader` or `Scanner` at each of those sites and each wrapper reads ahead into its private buffer: the header read buffers 4KB, the loop's scanner starts from wherever the file position really is, and lines silently vanish into the first buffer. The shape that works is to make the `bufio.Reader` next to the `Open` and pass *it* around - it is an `io.Reader` too, so everything downstream accepts it - and to read single lines with `r.ReadString('\n')`, which keeps the newline the way `<$fh>` did and returns what it has plus an error at the end of input.

## A handle is a value, so put it wherever values go

Perl's filehandles started out as names in a symbol table, which is why `*STDOUT`, `\*STDOUT` and `*STDOUT{IO}` all exist and why passing one around used to mean passing the glob. Go has no symbol table to point into: `os.Stdout` is a `*os.File`, a value, and every one of those spellings collapses to that same value on the way across. Once that lands, a whole category of Perl awkwardness disappears, because a value goes anywhere a value goes: into a map, into a struct field, into a slice, into a function parameter.

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// sink is a record with an open file in it, beside the ordinary fields. A
// handle is a value like any other, so it goes in a field like any other.
type sink struct {
	path    string
	written int
	file    *os.File
}

func main() {
	dir, err := os.MkdirTemp("", "streams")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll(dir)

	// A collection of open files is an ordinary map. The file and the error
	// arrive together, so the error is dealt with in a temporary and only a
	// file ever reaches the map.
	out := map[string]*os.File{}
	for _, name := range []string{"access", "error"} {
		f, err := os.Create(filepath.Join(dir, name+".log"))
		if err != nil {
			fmt.Println(err)
			return
		}
		out[name] = f
	}
	fmt.Fprintln(out["access"], "GET /index.html 200")
	fmt.Fprintln(out["error"], "permission denied")
	for _, f := range out {
		if err := f.Close(); err != nil {
			fmt.Println("close:", err)
		}
	}

	// The same value in a struct field, filled after the record exists.
	s := &sink{path: filepath.Join(dir, "audit.log")}
	if s.file, err = os.Create(s.path); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Fprintln(s.file, "audit trail")
	s.written++
	s.file.Close()

	// os.Stdout is one of these values too, not a name in a table.
	for _, w := range []*os.File{os.Stdout} {
		fmt.Fprintln(w, "and standard output is just another one")
	}
	for _, name := range []string{"access", "error", "audit"} {
		info, err := os.Stat(filepath.Join(dir, name+".log"))
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("%s: %d bytes\n", name, info.Size())
	}
}
```

```
and standard output is just another one
access: 20 bytes
error: 18 bytes
audit: 12 bytes
```

The one shape worth copying out of that is the three-step open into a container. `open($out{$name}, '>', $path) or die` is one statement in Perl because the handle is the only thing being produced; in Go the call produces two things and a map slot holds one, so it becomes: open into a temporary, check the error there, store the file. Skipping the middle step is not possible in a way the compiler will accept, which is the point. Declare the map as `map[string]*os.File` rather than `map[string]any` while you are there: the assertion you save at every use is worth more than the flexibility you give up, and if the collection really does hold several kinds of handle, `map[string]io.Writer` says that better than `any` does.

Further reading: https://pkg.go.dev/io#Reader
