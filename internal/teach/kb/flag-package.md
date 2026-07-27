---
id: flag-package
title: flag is not Getopt::Long, and the differences bite early
tags: [trap, cli, arguments, stdlib]
perl_triggers: [getopt-long, getoptions, getopt-std, getopts, argv, shift-argv, usage, pod-usage, exit-status]
severity: warning
prerequisites: [pointers-vs-references, strconv-parsing]
---

The standard `flag` package covers what most scripts need and costs nothing to adopt, but it is not a `Getopt::Long` clone and pretending otherwise produces a tool that behaves oddly for its users. Four of its rules differ from what thirty years of Perl have taught you: `-verbose` and `--verbose` are the same flag (the number of dashes carries no meaning), `-abc` is a flag *named* `abc` rather than three bundled short flags, a boolean never takes a separate value (`-v false` sets `-v` to true and leaves `false` as a positional argument), and **flag parsing stops at the first non-flag argument**, so `prog file.txt -v` never sees `-v`. Getopt::Long permutes by default and would have caught it.

## The Perl you know

```perl
use Getopt::Long;

my ($verbose, $name, $count, @libs) = (0, 'anon', 10);
GetOptions(
    'verbose!'  => \$verbose,      # gives --verbose and --no-verbose
    'name=s'    => \$name,
    'count=i'   => \$count,
    'lib=s@'    => \@libs,         # repeatable, collected into a list
) or die "usage: $0 [options] file...\n";

my @files = @ARGV;                 # whatever was left, in any position
```

Abbreviation (`--verb`), bundling (`-vn jane`), negation, and repeated options all come for free, and options may appear anywhere.

## The Go you write

```go
package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// list collects a repeated flag, which the standard set does not do for you.
type list []string

func (l *list) String() string { return strings.Join(*l, ",") }
func (l *list) Set(v string) error {
	*l = append(*l, v)
	return nil
}

func run(args []string) {
	// A FlagSet takes its arguments as a parameter, which is what makes this
	// testable. In a real main you would use the package-level flag.String
	// and flag.Parse(), which read os.Args[1:].
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // so this demo prints only what it decides to

	verbose := fs.Bool("verbose", false, "log every step")
	name := fs.String("name", "anon", "who is reporting")
	count := fs.Int("count", 10, "rows to print")
	timeout := fs.Duration("timeout", 5*time.Second, "give up after this long")
	var libs list
	fs.Var(&libs, "lib", "extra library directory (repeatable)")

	if err := fs.Parse(args); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("verbose=%-5v name=%-4s count=%-2d timeout=%-5s libs=%v rest=%q\n",
		*verbose, *name, *count, *timeout, libs, fs.Args())
}

func main() {
	run([]string{"-verbose", "--name", "jane", "-count=3", "-lib", "a", "-lib", "b", "in.txt"})
	run([]string{"-timeout", "1m30s"})
	run([]string{"in.txt", "-verbose"}) // flags after an argument are not flags
	run([]string{"-verbose", "false"})  // a bool never takes a separate value
	run([]string{"-verbose=false"})     // this is how you say it
	run([]string{"-count", "abc"})
	run([]string{"-nope"})
}
```

```
verbose=true  name=jane count=3  timeout=5s    libs=[a b] rest=["in.txt"]
verbose=false name=anon count=10 timeout=1m30s libs=[] rest=[]
verbose=false name=anon count=10 timeout=5s    libs=[] rest=["in.txt" "-verbose"]
verbose=true  name=anon count=10 timeout=5s    libs=[] rest=["false"]
verbose=false name=anon count=10 timeout=5s    libs=[] rest=[]
error: invalid value "abc" for flag -count: parse error
error: flag provided but not defined: -nope
```

Lines three and four are the two that cost people an afternoon. In line three the parser stopped at `in.txt`, so `-verbose` is a *positional argument* and the flag kept its default. In line four the boolean was set by its presence alone and `false` became a positional.

The generated help text comes free from the descriptions, and `-h` or `-help` prints it and exits 2 when no such flag is defined:

```console
$ report -h
Usage of report:
  -count int
    	rows to print (default 10)
  -lib value
    	extra library directory (repeatable)
  -name string
    	who is reporting (default "anon")
  -timeout duration
    	give up after this long (default 5s)
  -verbose
    	log every step
```

## The mismatch

The mapping, option type by option type. `'name=s' => \$name` is `flag.StringVar(&name, "name", "anon", "help text")`, or `name := flag.String(...)` if you would rather have a pointer; the `Var` forms are worth preferring because the rest of your code then reads `name` instead of `*name`. `'count=i'` is `flag.Int`, `'rate=f'` is `flag.Float64`, and `'verbose!'` is `flag.Bool`, which has no negated twin: `--no-verbose` does not exist and `-verbose=false` is the spelling. Perl's `=s@` repeatable option has no built-in equivalent at all, which is why the example implements `flag.Value` (a `String()` and a `Set(string) error` method); the same interface is how you accept an enum, a comma-separated list, or a validated path. `flag.Duration` is a small gift with no Perl counterpart: it parses `90s`, `1m30s`, and `2h` into a `time.Duration` (`time-layouts`).

The surrounding behaviour differs more than the types do. There is no such thing as a required flag: check for the zero value after `flag.Parse()` and write your own error. There is no abbreviation, so `-verb` is an unknown flag rather than a prefix match. Positional arguments are `flag.Args()` and `flag.NArg()`, never `os.Args` directly, and `os.Args[0]` is still the program name. On a bad flag the default `flag.ExitOnError` mode prints the usage and calls `os.Exit(2)`, which is fine in `main` and terrible in a library or a test, so construct a `flag.NewFlagSet` with `flag.ContinueOnError` when you want the error back as a value. Subcommands are several `FlagSet`s and a switch on `os.Args[1]`, which is more typing than a CPAN module and easy to read afterwards.

When the stdlib genuinely is not enough (GNU-style `-abc` bundling, `--flag` distinct from `-f`, flags after arguments), the ecosystem answer is `spf13/pflag` or a full CLI framework, and taking that dependency is a normal decision rather than a defeat. Start with `flag`; most scripts never outgrow it.

Further reading: https://pkg.go.dev/flag
