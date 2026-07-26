# Concept lessons

These 24 lessons were selected by what is in your code, not from a fixed syllabus. They are ordered so that a lesson never depends on one further down the list, which makes top to bottom a sensible way to read them.

A lesson marked as a trap describes something that produces a crash or wrong data from code that looks correct. Read those first if you are short on time.

1. [The compiler is the first test suite](compile-time-mindset.md). Errors you are used to discovering in production at 3 a.m. — a typoed function name, a call into a module that never loaded, a variable you meant to use but did not — do not exist in a running Go...
2. [Every variable has a type and is never uninitialised](static-types-and-zero-values.md). In Perl, a freshly declared variable holds `undef` and its "type" is whatever you use it as next.
3. [Capitalisation is the entire privacy system](packages-and-exported-names.md). Go has no `Exporter`, no `@EXPORT_OK`, no `use Pkg qw(func)`, and no convention-only privacy.
4. [Structs replace hashrefs, and embedding is not inheritance](structs-and-embedding.md). The blessed hashref — Perl's universal object — becomes a struct.
5. [nil is not undef, and nothing autovivifies](nil-vs-undef.md) (trap). `undef` is a universal value any scalar can hold.
6. [Slices are views with capacity, arrays are values](slices-not-arrays.md) (easy to get wrong). Perl's `@array` maps to Go's *slice* (`[]int`), not Go's array (`[3]int`) — an array in Go has its length baked into its type, is copied wholesale on assignment, and you will rarely declare one on...
7. [range gives you the index first, and the element is a copy](range-is-not-foreach.md) (easy to get wrong). Two habits from `foreach` will produce wrong Go on day one.
8. [A nil slice works; writing to a nil map panics](nil-slices-vs-nil-maps.md) (trap). In Perl, `my @list` and `my %hash` are both immediately usable.
9. [Comma-ok replaces exists, and defined has no seat at the table](comma-ok-idiom.md). A Go map lookup always succeeds: `m[k]` on a missing key quietly returns the zero value, so `visits["bob"]` is `0` whether bob visited zero times or was never seen at all.
10. [Sorting is a function call, and the default is numeric-aware](sort-slice.md). Perl's `sort` is a builtin that returns a new list and defaults to string comparison, which is why `sort { $a <=> $b }` is muscle memory for every Perl programmer alive.
11. [Map order is randomised per loop, on purpose](map-iteration-order.md) (easy to get wrong). You already know hashes are unordered — Perl has randomised per-process since 5.18 — but Go goes a step further that will break a specific class of ported code.
12. [Declaring variables, := versus var, and the shadowing trap](var-vs-short-declaration.md) (easy to get wrong). Go has two ways to declare a variable, and picking between them is mechanical, not stylistic.
13. [Multiple returns replace both list-return and wantarray](multiple-return-values.md). Go functions return a fixed, typed tuple — `(int, error)` is the canonical shape — and the caller must account for every value.
14. [Errors are return values, not exceptions](errors-are-values.md). Go has no exceptions in the working sense.
15. [No coercion, ever - numbers and strings never blur](explicit-conversions-no-coercion.md) (easy to get wrong). Perl's scalar is simultaneously a number and a string and converts on demand.
16. [strconv turns strings into numbers, and refuses to guess](strconv-parsing.md). Every place your Perl relied on a string quietly becoming a number — reading a config value, a CGI parameter, a column from a file — becomes an explicit `strconv` call in Go that returns a value...
17. [Compile once at package level with MustCompile](mustcompile-pattern.md). Perl compiles a literal regex once, transparently, no matter how many times execution passes over it — the interpreter caches it, and `qr//` exists only for the dynamic cases.
18. [FindStringSubmatch replaces $1, and no-match returns nil](submatch-and-named-groups.md) (easy to get wrong). There are no match variables: `$1`, `$&`, `%+`, `@-` all vanish, replaced by methods returning slices — `FindStringSubmatch` gives `[$&, $1, $2, ...]` as a `[]string`, and the family of `Find*`...
19. [Pointers are explicit references, and nothing aliases @_](pointers-vs-references.md). Go pointers are Perl references with the training wheels *and* the magic removed.
20. [Pointer versus value receivers, and the method set rules](methods-and-receivers.md) (trap). Every Perl method gets `$self` as a reference, so mutating `$self->{field}` always sticks.
21. [Interfaces are satisfied implicitly - duck typing, checked](implicit-interfaces.md). Go's interface is Perl duck typing with the runtime risk removed.
22. [io.Reader and io.Writer - the universal plumbing](io-reader-writer.md). Perl unified I/O around the filehandle, and `open my $fh, '<', \$string` let scalars impersonate files.
23. [The if err != nil rhythm, and why silence still compiles](if-err-nil-rhythm.md). Every fallible call in Go is followed by three lines.
24. [bufio.Scanner reads lines, and gives up on long ones](bufio-scanner-limit.md) (trap). `while (my $line = <$fh>)` has no length limit and no error to check.

---

To read any of these outside a conversion, or to look up something that is not here, run `perl2go explain <topic>`; `perl2go explain --list` prints every topic the tool knows.

Written by perl2go 0.1.0, from your source.
