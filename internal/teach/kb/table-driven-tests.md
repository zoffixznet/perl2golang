---
id: table-driven-tests
title: Tests are ordinary code in _test.go, and the table is the culture
tags: [idiom, testing, tooling]
perl_triggers: [test-more, test-simple, ok, is, isnt, like, is-deeply, done-testing, plan, prove, test-exception, subtest, t-directory]
severity: info
prerequisites: [toolchain-gofmt-godoc, multiple-return-values]
---

Testing in Go needs nothing installed: no `Test::More`, no `prove`, no `Build.PL` wiring, no plan. A file whose name ends in `_test.go` sits *next to* the code it tests, in the same package, and `go test ./...` finds and runs everything. The two adjustments from Perl are that there are no assertion functions in the standard library, so you write `if got != want { t.Errorf(...) }` yourself, and that the community has converged hard on one shape, the table-driven test, which you should copy on day one rather than discover on day thirty.

The absence of `is`, `ok`, and `like` is deliberate. A comparison you write yourself produces a failure message you wrote yourself, which beats decoding somebody's DSL at 2 a.m. The one third-party package that has become near-universal is `github.com/google/go-cmp/cmp`, for comparing structs and maps with a readable diff; reach for it when `==` is not enough, not before.

## The Perl you know

```perl
# t/hostport.t
use Test::More;

my ($host, $port) = split_hostport('db.internal:5432');
is $host, 'db.internal', 'host parsed';
is $port, 5432,          'port parsed';
ok !eval { split_hostport('db.internal'); 1 }, 'missing port dies';
like $@, qr/no port/, 'and says why';

done_testing;
```

Separate directory, separate runner (`prove -l t/`), and the test file has to arrange its own access to the code under test.

## The Go you write

Two files in the same directory. First `hostport.go`:

```go
package hostport

import (
	"fmt"
	"strconv"
	"strings"
)

// Split splits "host:port" into its parts. A missing port is an error; a
// missing host is not, because ":8080" means "every interface".
func Split(addr string) (host string, port int, err error) {
	h, p, found := strings.Cut(addr, ":")
	if !found {
		return "", 0, fmt.Errorf("split %q: no port", addr)
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return "", 0, fmt.Errorf("split %q: bad port: %w", addr, err)
	}
	return h, n, nil
}
```

Then `hostport_test.go`, in the same package, which is why it can test unexported functions too:

```go
package hostport

import "testing"

func TestSplit(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		{name: "host and port", addr: "db.internal:5432", wantHost: "db.internal", wantPort: 5432},
		{name: "no host", addr: ":8080", wantHost: "", wantPort: 8080},
		{name: "no port", addr: "db.internal", wantErr: true},
		{name: "port is not a number", addr: "db.internal:http", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := Split(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Split(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if host != tt.wantHost || port != tt.wantPort {
				t.Errorf("Split(%q) = %q, %d; want %q, %d", tt.addr, host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}
```

`go test -v` names every row, because each `t.Run` is a subtest with its own name and its own pass or fail:

```console
$ go test -v ./...
=== RUN   TestSplit
=== RUN   TestSplit/host_and_port
=== RUN   TestSplit/no_host
=== RUN   TestSplit/no_port
=== RUN   TestSplit/port_is_not_a_number
--- PASS: TestSplit (0.00s)
    --- PASS: TestSplit/host_and_port (0.00s)
    --- PASS: TestSplit/no_host (0.00s)
    --- PASS: TestSplit/no_port (0.00s)
    --- PASS: TestSplit/port_is_not_a_number (0.00s)
PASS
ok  	hostport	0.002s
```

Break the function and the failure names the row, the input, and both values, with no extra work from you:

```console
$ go test ./...
--- FAIL: TestSplit (0.00s)
    --- FAIL: TestSplit/host_and_port (0.00s)
        hostport_test.go:29: Split("db.internal:5432") = "db.internal", 5433; want "db.internal", 5432
    --- FAIL: TestSplit/no_host (0.00s)
        hostport_test.go:29: Split(":8080") = "", 8081; want "", 8080
FAIL
FAIL	hostport	0.001s
FAIL
```

Subtest names are also selectors: `go test -run 'TestSplit/no_port'` runs exactly that row, which is the debugging loop you want when one case out of forty is wrong.

## Fixtures without a cleanup ritual

```go
package hostport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithFixture(t *testing.T) {
	dir := t.TempDir() // removed automatically when the test ends
	path := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REPORT_MODE", "quiet") // restored automatically too

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	if got, want := len(data), 4; got != want {
		t.Errorf("len = %d, want %d", got, want)
	}
}
```

`t.TempDir()` gives a fresh directory per test and deletes it afterwards; `t.Setenv` restores the previous value; `t.Cleanup(fn)` registers anything else, running in reverse order like `defer` but tied to the test rather than the function (`defer-timing`). Between them, `File::Temp` plus an `END` block plus remembering to unset the environment variable becomes three lines that cannot leak.

## The mismatch

The vocabulary, mapped. `is`/`ok`/`like` become an `if` and a `t.Errorf`, formatted with the house convention `got %v, want %v` in that order, and naming the input; the message is read by whoever broke the build, so write it for them. `t.Errorf` marks the test failed and keeps going, `t.Fatalf` stops this test function immediately (it is `runtime.Goexit`, not a panic, so it must be called from the test goroutine, never from inside a `go func`). `is_deeply` is `reflect.DeepEqual` for a yes-or-no answer, or `cmp.Diff` when you want to see which field differs. `dies_ok`/`throws_ok` need no special support because errors are values: call the function, check the error (`errors-are-values`).

Structure differs in ways that pay off. There is no plan and no `done_testing`: the runner knows how many test functions there are. Test files live beside the code, so `package hostport` in a `_test.go` file can reach unexported identifiers, while `package hostport_test` (also allowed, in the same directory) sees only the exported API and is the honest way to test the package as its users see it. `TestMain(m *testing.M)` is the once-per-package setup hook. `t.Parallel()` marks a test as safe to run alongside its siblings, which combines with `-race` to find data races (`race-detector`). `t.Helper()` inside an assertion helper makes failures point at the caller rather than at the helper.

Two behaviours surprise everyone once. Results are **cached**: a second `go test ./...` with nothing changed prints `(cached)` and runs nothing, so `-count=1` is how you force a real run when a test depends on something outside the source tree. And an `Example` function whose body ends in an `// Output:` comment is compiled, run, and checked against that comment, so your documentation examples are tests and appear on the package's documentation page. There is nothing like it in the Perl toolchain, and it is the cheapest test you will ever write.

Further reading: https://pkg.go.dev/testing
