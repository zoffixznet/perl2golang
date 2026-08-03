# Go for Perl developers

You already know how to program. What you need is not a tour of `if` and `for` but an
account of where Perl instinct produces wrong Go, and of what Go gives you in exchange.
This document is that account, meant to be read next to the converted code in this
directory.

Go is a much smaller language than Perl: no context, no `tie`, no runtime symbol table,
no operator overloading worth the name, one loop keyword, one formatter. Most of what
follows is subtraction. What Go added instead is not syntax — it is a compiler that
refuses to build a program containing an unused variable, a race detector that finds bugs
your tests never will, and a deployment story in which "which Perl is on the box" is not
a question.

Alongside this document sit short single-concept lessons in `concepts/`, chosen by what
your script actually does. Where a topic here has a lesson it is named in backticks —
"see the `slice-aliasing-and-copy` lesson". The ones your code pulled in are links; for
any other, `perl2golang explain <name>` prints it. Those lessons go deep on one thing; this
is the map.

## Two decisions that explain the rest

**Types are checked before the program runs, and there is no coercion.** A Perl scalar is
a container holding a string, an integer, a float or a reference, converting on demand. A
Go variable has one type, fixed at compile time, and the language converts nothing for
you — not `string` to `int`, not even `int` to `int64`. Zero values, the death of
`"10" + 5`, two comparison operator sets collapsing into one, and `if $x` not compiling
all fall out of this.

**There is no context.** In Perl every expression is evaluated in a context imposed by its
surroundings, and `wantarray` lets one sub behave two ways. Go has one signature per
function. `localtime` cannot be a string here and a nine-element list there; `reverse`
cannot reverse a string in one place and a list in another. Perl's context is an invisible
extra argument to every call; Go makes you name it, usually as two functions where you had
one. See `multiple-return-values`.

A third, softer decision shapes the day-to-day: the compiler is a gate, not an advisor. An
unused import or unused local is an *error*, with no flag to downgrade it.

```
unused/main.go:5:2: "os" imported and not used
unused/main.go:9:2: declared and not used: count
```

The first week this is infuriating; after that it is the cheapest code review you will
ever get. See `compile-time-mindset`.

## Types, zero values, and the scalar that stopped being polymorphic

`var x T` declares with an explicit type and gives you the zero value; `x := expr` declares
and infers. Inside a function the short form is the default; at package scope only `var`
and `const` are legal. The trap: `:=` inside an `if` or `for` initialiser declares *new*
variables scoped to that block, which is how the shadowed-`err` bug happens. See
`var-vs-short-declaration`.

### Everything is initialised

There is no `undef` and no uninitialised storage. Every type has a zero value:

```go
	var (
		n int
		s string
		p *Job
		l []string
	)
	fmt.Printf("int %d  string %q  *Job %v\n", n, s, p)
	fmt.Printf("[]string %v len=%d nil=%t\n", l, len(l), l == nil)

	var j Job // struct with string, int, bool, slice, map and pointer fields
	fmt.Printf("%+v\n", j)

	l = append(l, "works on a nil slice") // usable with no constructor
	fmt.Println(l)
```

```
int 0  string ""  *Job <nil>
[]string [] len=0 nil=true
{Name: Retries:0 Done:false Tags:[] Meta:map[] Parent:<nil>}
[works on a nil slice]
```

The cultural instruction is "make the zero value useful": design structs so all-zero is a
sensible default and half your constructors disappear. `var buf bytes.Buffer` and
`var mu sync.Mutex` are ready to use with no `new`. The cost is that "declared but never
set" and "explicitly set to zero" are the same state. Perl code distinguishing `undef`
from `0` or `""` — tri-state flags, optional record fields, cache-negative entries — must
model that deliberately: a `*string` field where nil means absent, a second boolean
return, or a comma-ok map lookup. See `static-types-and-zero-values` and `comma-ok-idiom`.

### The string/number duality is gone

```perl
print "10"+5, "|", "10abc"+1, "|", "abc"+1, "|",
      ("10" == "1e1" ? "true" : "false"), "|", "10"."", "\n";
```

```
15|11|1|true|10
```

One scalar, three facets, silent conversion both ways. `"10abc" + 1` is `11` because
numification takes the longest leading numeric prefix; `"abc" + 1` is `1` because that
prefix is empty. Under `use warnings` you are told; otherwise you are not.

The direct translation does not compile, and neither does the version people expect to be
waved through:

```go
	s := "10"
	n := 5
	fmt.Println(s + n)
```

```
nocoerce/main.go:8:14: invalid operation: s + n (mismatched types string and int)
nocoerce2/main.go:8:14: invalid operation: i + j (mismatched types int and int64)
```

What you write instead is an explicit, *fallible* conversion:

```go
	n, err := strconv.Atoi("10") // the honest translation of "10" + 5
	if err != nil {
		return fmt.Errorf("not a number: %w", err)
	}
	fmt.Println(n + 5)

	for _, in := range []string{"10", "10abc", "abc", " 10", "1e1", ""} {
		v, err := strconv.Atoi(in)
		fmt.Printf("Atoi(%-7q) = %-3d err=%v\n", in, v, err)
	}
```

```
15
Atoi("10"   ) = 10  err=<nil>
Atoi("10abc") = 0   err=strconv.Atoi: parsing "10abc": invalid syntax
Atoi("abc"  ) = 0   err=strconv.Atoi: parsing "abc": invalid syntax
Atoi(" 10"  ) = 0   err=strconv.Atoi: parsing " 10": invalid syntax
Atoi("1e1"  ) = 0   err=strconv.Atoi: parsing "1e1": invalid syntax
Atoi(""     ) = 0   err=strconv.Atoi: parsing "": invalid syntax
```

No leading whitespace, no float syntax, no numeric prefix: an error, not a best-effort
number. `strconv.ParseFloat("1e1", 64)` does return `10`, because that *is* a valid float
literal — a different question from "what number does this string most resemble". Going
the other way never fails: `strconv.Itoa(42)`, `strconv.FormatFloat(3.5, 'f', -1, 64)`,
`fmt.Sprintf("%d apples", 42)`.

Why this is a feature rather than pedantry. The `"10 apples" + 1` bug is among the most
common sources of silent wrongness in long-lived Perl: a CSV field arrives as `"12 units"`,
becomes `12` in arithmetic, `"12 units"` in a report, and a warning nobody reads in a log.
In Go that field is a `string` until you say otherwise, and the moment you say otherwise
you are handed the question "what happens when it is not a number?" at the line where it
matters. The second-order effect is that Perl's two operator sets collapse into one. Perl
needs `==` and `eq` because the scalar cannot say which comparison it wants, which is why
`"10" lt "9"` is true and `$v1 <=> $v2` on `"1.10"` does something you did not intend. In
Go the variable carries the type, so one set suffices and comparing a `string` to an `int`
does not compile. See `explicit-conversions-no-coercion` and `strconv-parsing`.

### Truthiness does not exist either

Perl's rule: false iff `undef`, numeric zero, `""`, or the string `"0"` — so `"0.0"`,
`"00"`, `"0E0"` and `" "` are all true.

```perl
for my $v (0, "0", "0.0", "00", "0E0", " ", "", undef) {
    printf "%-6s => %s\n", defined($v) ? "\"$v\"" : "undef", ($v ? "true" : "false");
}
```

```
"0"    => false
"0"    => false
"0.0"  => true
"00"   => true
"0E0"  => true
" "    => true
""     => false
undef  => false
```

Go has no such rule. Only a `bool` may appear in a condition, so `if name { ... }` on a
string is `non-boolean condition in if statement`, and every truth test becomes an
explicit comparison chosen by type. With `s := "0"` and everything else zero:

```go
	fmt.Println(s != "")                 // "is non-empty": NOT the same as Perl
	fmt.Println(s != "" && s != "0")     // Perl's actual rule for strings
	fmt.Println(n != 0)                  // number
	fmt.Println(len(xs) > 0, len(m) > 0) // slice, map
	fmt.Println(p != nil)                // pointer
```

```
true
false
false
false false
false
```

The first two lines are the whole story: they disagree about the string `"0"`. When reading
converted code, look at every string truth test that became `!= ""` and ask whether a
legitimate `"0"` can arrive there. If it can, Perl was treating a real value as false, and
the type system has just surfaced a latent bug.

### Arithmetic, and strings as bytes

Go's `/` truncates on integers (`7 / 2` is `3`, and `float64(7) / 2` is the usual fix)
where Perl's is always floating point; Go's `%` takes its sign from the *left* operand
(`-7 % 3` is `-1`, `7 % -3` is `1`) where Perl takes it from the right (`2` and `-2`).
There is no `**`; use `math.Pow`. Integer overflow wraps silently rather than promoting to
a double. Those two operators are the arithmetic most likely to change a converted
program's answers without changing its shape.

A Go string is a read-only slice of bytes, and indexing gives a byte. Perl's `use utf8`
character semantics have no equivalent; you convert to `[]rune` when you mean characters.

```go
	s := "héllo"
	fmt.Println(len(s), utf8.RuneCountInString(s))
	fmt.Printf("%q\n", s[1:2])                 // half a rune
	fmt.Printf("%q\n", string([]rune(s)[1:2])) // what substr() meant
```

```
6 5
"\xc3"
"é"
```

Ranging a string gives *byte offsets* rather than indexes — `for i, r := range "héllo"`
yields `i` values `0 1 3 4 5`. See `strings-are-bytes`; this matters for every `length`,
`substr`, `reverse` and character loop in your converted code.

## `nil` is not `undef`

`undef` is a value any Perl scalar can hold. `nil` is not a value of most Go types at all.
It is the zero value of exactly six type families:

| Nilable | Not nilable |
|---|---|
| pointers (`*T`) | `int`, `int64`, `float64`, `bool`, `string` |
| slices (`[]T`) | arrays (`[N]T`) |
| maps (`map[K]V`) | structs |
| channels, function values, interfaces (`error`, `any`) | |

`x == nil` where `x` is an `int` is a compile error, so "is it defined?" is frequently a
question Go makes unaskable. Where your Perl asked it, the converted code either
restructured so the zero value is the answer, or introduced a pointer or comma-ok lookup to
carry the distinction explicitly.

The other half is that nothing autovivifies and dereferencing nil is a crash rather than a
warning. `nil-vs-undef` covers that, including a read/write asymmetry exactly inverted from
Perl's: Go reads through missing nested keys safely and creates nothing, but writes need
the path built by hand.

### The typed-nil trap

This one catches everybody once and has no Perl analogue — it is the single place where
Go's nil is *stranger* than `undef`. An interface value is a pair, a type and a value; an
interface holding a nil pointer is not a nil interface, because the type half is populated.

```go
type ValidationError struct{ Field string }

func (e *ValidationError) Error() string { return "invalid field " + e.Field }

// WRONG: returns a typed nil pointer inside an error interface.
func validateBad(name string) error {
	var e *ValidationError // nil
	if name == "" {
		e = &ValidationError{Field: "name"}
	}
	return e // the interface is never nil
}

// RIGHT: return the literal nil.
func validateGood(name string) error {
	if name == "" {
		return &ValidationError{Field: "name"}
	}
	return nil
}
```

```
validateBad:  err == nil? false  value=<nil>  type=*main.ValidationError
validateGood: err == nil? true   value=<nil>  type=<nil>
```

`validateBad` succeeded and still reported failure to every caller, while printing `<nil>`
when you debug it. The rule is absolute: a function returning `error` must `return nil`
literally, never a nil variable of a concrete error type. See `typed-nil-interface`.

## Slices, arrays, and the aliasing you did not ask for

An **array** is `[3]int`: fixed length, length part of the type, and a *value*. Assigning
copies, passing to a function copies, and `[3]int` and `[4]int` are different types. This
is what Perl gives you with `@b = @a`. A **slice** is `[]int`: a three-word header —
pointer, length, capacity — pointing into a backing array elsewhere. Assigning copies the
*header*, not the data.

```go
func mutate(arr [3]int, sl []int) {
	arr[0] = 100
	sl[0] = 100
}

	arr := [3]int{1, 2, 3} // an ARRAY: fixed size, a value
	sl := []int{1, 2, 3}   // a SLICE: a view onto a backing array
	mutate(arr, sl)
	fmt.Printf("after call: arr=%v  sl=%v\n", arr, sl)
	// Arrays are comparable; slices are not (slice == slice does not compile).
	fmt.Println([3]int{1, 2, 3} == [3]int{1, 2, 3})
```

```
after call: arr=[1 2 3]  sl=[100 2 3]
true
```

Nearly all Go code uses slices; arrays appear as fixed-size buffers, as map keys, and inside
struct definitions. Your `@a` almost always became a `[]T`. See `slices-not-arrays`.

### The append bug

`append` grows a slice only when capacity is exhausted, so whether it writes into shared
memory or reallocates is *data-dependent*. See this once deliberately rather than discover
it in production:

```go
	a := []int{1, 2, 3, 4}
	b := a[:2]
	fmt.Printf("a=%v  b=%v  len(b)=%d cap(b)=%d\n", a, b, len(b), cap(b))

	b = append(b, 99) // capacity is 4, so this writes into a's backing array
	fmt.Printf("after append(b, 99):     a=%v  b=%v\n", a, b)

	b = append(b, 5, 6, 7) // capacity exhausted: b reallocates and detaches
	b[0] = -1
	fmt.Printf("after append(b, 5, 6, 7): a=%v  b=%v\n", a, b)

	c := a[:2:2] // cap-clamped subslice: append is now forced to copy
	c = append(c, 1000)
	fmt.Printf("after append(c, 1000):   a=%v  c=%v\n", a, c)

	d := slices.Clone(a) // or just take the copy Perl would have given you
	d[0] = 0
	fmt.Printf("clone:                   a=%v  d=%v\n", a, d)
```

```
a=[1 2 3 4]  b=[1 2]  len(b)=2 cap(b)=4
after append(b, 99):     a=[1 2 99 4]  b=[1 2 99]
after append(b, 5, 6, 7): a=[1 2 99 4]  b=[-1 2 99 5 6 7]
after append(c, 1000):   a=[1 2 99 4]  c=[1 2 1000]
clone:                   a=[1 2 99 4]  d=[0 2 99 4]
```

Read the second line again: appending to `b` changed `a`. Then the third: after one more
append `b` detached, so writing `b[0] = -1` left `a` alone. The same statement did two
different things depending on capacity. Perl has aliasing too, but Perl's is explicit and
all-or-nothing (`\@a`, `foreach`, `@_`); slice aliasing is implicit and partial. Defences,
in order: `slices.Clone(a)` when you need an independent copy; the three-index form
`a[:2:2]` to clamp capacity so any append must copy; and treating `cap()` as something you
check before keeping a subslice around. See `slice-aliasing-and-copy`.

The mirror image is that a callee cannot grow your slice. The Perl instinct is to pass
`\@a` so the callee can `push`; passing a slice is cheap, but the length lives in the
header, which was copied, so `func addBroken(xs []string, v string) { xs = append(xs, v) }`
leaves the caller's slice untouched. That is why `append` is always written
`a = append(a, x)` and why a function that grows a slice returns it. Maps never have this
problem: a map header carries no length the callee could change independently.

## Maps

`map[K]V` replaces `%h` and most of it transfers. Three things do not.

**Reading a missing key returns the zero value, silently.** Perl warns under `use warnings`;
Go never does. Comma-ok is the `exists` question, and the only way to tell "absent" from
"present and zero":

```go
	counts := map[string]int{"apple": 3, "pear": 0}
	fmt.Println(counts["durian"]) // missing key: zero value, no warning

	for _, k := range []string{"apple", "pear", "durian"} {
		v, ok := counts[k]
		fmt.Printf("%-7s value=%d present=%t\n", k, v, ok)
	}

	counts["fig"]++         // the one place Go feels as breezy as Perl
	delete(counts, "apple") // no return value; read first if you need it

	// Deterministic output requires sorting the keys yourself.
	for _, k := range slices.Sorted(maps.Keys(counts)) {
		fmt.Printf("%s=%d ", k, counts[k])
	}
```

```
0
apple   value=3 present=true
pear    value=0 present=true
durian  value=0 present=false
fig=1 pear=0 
```

`delete` returns nothing, so `my $v = delete $h{k}` becomes two statements. And
`counts["fig"]++` works from nothing — the one crumb of autovivification Go gives you, and
why word-count loops still look almost like Perl. See `comma-ok-idiom`.

**A nil map reads fine and panics on write.** `var m map[string]int` is a usable read-only
empty map right up until you assign to it:

```go
	var m map[string]int // nil map
	fmt.Println(len(m), m["anything"], m == nil)
	m["boom"] = 1 // panics
```

```
0 0 true
panic: assignment to entry in nil map
```

Always create maps with `make(map[K]V)` or a literal. The asymmetry with slices — where a
nil slice appends happily — is the subject of `nil-slices-vs-nil-maps`.

**Iteration order is randomised on every loop.** Perl randomised hash order per hash per
process in 5.18 for HashDoS resistance, so `keys %h` twice in one program gives the same
order twice. Go goes further on purpose: the starting point is randomised for every single
`range`.

```go
	m := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}
	for i := range 4 {
		fmt.Print("pass ", i, ": ")
		for k := range m {
			fmt.Print(k, " ")
		}
		fmt.Println()
	}
```

```
pass 0: c d e a b 
pass 1: a b c d e 
pass 2: c d e a b 
pass 3: b c d e a 
```

This is a design decision, not an implementation accident: code depending on map order is
broken *today*, not on some future release. When you need deterministic output — reports,
diffs, tests — sort the keys, the exact analogue of `for my $k (sort keys %h)`. See
`map-iteration-order`.

Finally, keys are typed and can be structs. Perl stringifies every key, which is why `$h{1}`
and `$h{"1"}` are the same key and why multidimensional access is `$h{$x,$y}` with a `$;`
separator. In Go, `map[cell]string` with `type cell struct{ Row, Col int }` retires that
idiom entirely, and `map[[2]int]int` works too. Slices and maps are not comparable, so they
cannot be keys, and neither can a struct containing one. See `maps-of-slices`.

## Errors are values

Go has no exceptions in the working sense: nothing you call unwinds your stack on failure.
A function that can fail says so in its type, and failure comes back as an ordinary return
value.

```perl
my $ok = eval { die "boom\n"; 1 };
print "caught: $@" unless $ok;
print "alive\n";                    # prints: caught: boom / alive
```

Everything about that Perl is invisible from outside: nothing in the sub's signature says
it can die, `$@` is fragile global state any later `eval` can clobber, and forgetting the
`eval` crashes the program. The Go equivalent contains no `eval` at all, because there is
nothing to intercept — the error arrived in a variable.

```go
// Each fallible step is followed by its own decision. This is the shape of
// nearly every Go function you will write.
func sumFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("sum %s: %w", path, err)
	}
	defer f.Close()

	total := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		n, err := strconv.Atoi(strings.TrimSpace(sc.Text()))
		if err != nil {
			return 0, fmt.Errorf("sum %s: %w", path, err)
		}
		total += n
	}
	// Scanner reports its error separately; skipping this check is the
	// classic way to silently truncate input.
	return total, sc.Err()
}
```

```
42 <nil>
0 sum /no/such/file: open /no/such/file: no such file or directory
```

Two `if err != nil` blocks in one small function is normal. It is not boilerplate noise; it
is `or die` with the coercion rules removed and the decision made per call. The important
discipline is the inverted failure mode: an unhandled Perl `die` climbs until something
cares, so forgetting `eval` gives you a crash you *notice*, while an unchecked Go error
goes nowhere at all and gives you silence. This is the one place Go's defaults are less
safe than Perl's, and it is why vet, linters and reviewers all treat a discarded error as a
finding. See `if-err-nil-rhythm`.

### Wrapping, sentinels, and inspection

`error` is a one-method interface — `Error() string` — and that is the whole type.
`errors.New` makes an opaque one, `fmt.Errorf` formats one, and `%w` *wraps* another so the
chain stays inspectable.

```go
// A sentinel: a package-level error value callers may compare against.
var ErrNotFound = errors.New("record not found")

// A typed error: carries data the caller can inspect.
type ParseError struct{ Line int }

func (e *ParseError) Error() string { return fmt.Sprintf("line %d: empty record", e.Line) }

func lookup(id string) (string, error) {
	switch id {
	case "42":
		return "the answer", nil
	case "":
		return "", &ParseError{Line: 7}
	}
	return "", fmt.Errorf("lookup %q: %w", id, ErrNotFound)
}

func load(id string) (string, error) {
	v, err := lookup(id)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err) // context accumulates
	}
	return v, nil
}
```

```go
	_, err := load("13")
	fmt.Println(err)
	fmt.Println("errors.Is(err, ErrNotFound):", errors.Is(err, ErrNotFound))

	var pe *ParseError
	if _, err := load(""); errors.As(err, &pe) {
		fmt.Printf("errors.As reached a *ParseError with Line=%d\n", pe.Line)
	}

	_, err = os.Open("/nonexistent/path") // stdlib sentinels work the same way
	fmt.Println("errors.Is(err, os.ErrNotExist):", errors.Is(err, os.ErrNotExist))
```

```
load config: lookup "13": record not found
errors.Is(err, ErrNotFound): true
errors.As reached a *ParseError with Line=7
errors.Is(err, os.ErrNotExist): true
```

That message is built the way Perl builds `open ... or die "opening $f: $!"` — except every
layer stays machine-inspectable, and both `errors.Is` and `errors.As` see straight through
the `load config:` wrapper. `Is` asks about identity; `As` asks about type and hands you
the concrete value. Go errors carry no file and line: the idiom is to add *operation*
context at each level, which reads better than `at foo.pl line 42` ever did.

Three habits to unlearn. **Matching on error strings**: `$@ =~ /No such file/` becomes
`errors.Is(err, fs.ErrNotExist)`. **Reflexive `%w`**: wrapping makes the wrapped error part
of your API, so use `%v` when you are deliberately not promising that. **Building try/catch
out of panic and recover**: it compiles, and the community reads it as not knowing the
language.

Conventions: sentinels are `ErrSomething` at package scope; custom types are
`SomethingError` implementing `Error() string`, usually on the pointer receiver — which is
why `errors.As` targets are declared `var pe *ParseError`. See `errors-are-values`,
`error-wrapping`, `sentinel-and-custom-errors`.

## `defer`, `panic`, and `recover`

`defer` schedules a call for when the enclosing *function* returns, however it returns —
normal, early, or panicking. It is `local`'s restore guarantee, `DESTROY`'s cleanup and
`finally` in one keyword. Deferred calls run last-in-first-out, and their **arguments are
evaluated at `defer` time**:

```go
func demo() (result string) {
	i := 1
	defer fmt.Println("A: argument evaluated at defer time, i was", i)
	defer func() { fmt.Println("B: closure reads final state, i is", i) }()
	defer func() { result = result + " (amended by defer)" }()

	i = 2
	return "returned value"
}
```

```
B: closure reads final state, i is 2
A: argument evaluated at defer time, i was 1
demo returned: returned value (amended by defer)
```

`B` runs before `A` (LIFO). `A` printed `1` because its argument was evaluated when the
`defer` statement ran, while `B` printed `2` because a closure reads the variable later.
And the third deferred function modified the *named* return value after `return` had
already chosen it — which is exactly how `recover` turns a panic into an error.

The scope is the function, not the block. `defer f.Close()` inside a loop does not close
each iteration; it queues one close per iteration and runs them all at the end, which is
how you exhaust file descriptors. For block-precise cleanup, wrap the block in a function
literal and call it. See `defer-timing`.

This is also the honest translation of `local`, one of the constructs with no Go keyword at
all:

```go
var recordSep = "\n"

func slurp() {
	saved := recordSep
	recordSep = "" // Perl's local $/ = undef;
	defer func() { recordSep = saved }()
	// ... recordSep is "" here, and "\n" again for the caller
}
```

The differences from `local`: restoration happens at function exit rather than block exit,
and callees see the change only because it is a package-level variable — there is no
dynamic scoping. Most real uses of `local` (`local $/`, `local $_`, `local $SIG{...}`,
`local %ENV`) disappear in Go anyway, because those things are passed as explicit values
rather than stashed in globals.

### `panic` is not `die`

A panic unwinds the stack running deferred functions, prints the value and a goroutine
stack trace, and exits with status 2. The resemblance to `die` ends there, because the
*meaning* differs: a panic says "this program has a bug", not "this operation failed".
Runtime panics come from indexing out of range, dereferencing nil, dividing by zero and
writing to a nil map:

```
panic: runtime error: index out of range [5] with length 3

goroutine 1 [running]:
main.main()
	/.../panicdemo/main.go:8 +0x9
exit status 2
```

A Go function that panics because a file is missing is considered broken. File-missing is a
condition; it gets an `error`. Panic is for invariants you believe cannot be violated, and
for the `Must` convention — `regexp.MustCompile`, `template.Must` — where the argument is a
compile-time constant and failure means the source is wrong.

`recover()` does something only when called directly inside a deferred function during a
panic, and it has exactly two legitimate uses: converting a panic into an error at a
package boundary, and keeping a server alive when one request handler misbehaves.

```go
// A package boundary that refuses to let a panic escape into caller code.
func safeDivide(a, b int) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("safeDivide: %v", r)
		}
	}()
	return a / b, nil
}
```

```
5 <nil>
0 safeDivide: runtime error: integer divide by zero
still running
```

The named return values are what let the deferred function set `err`. And note the one
thing `recover` categorically cannot do: there is no cross-goroutine `$@`. A panic in a
goroutine you started cannot be recovered by the goroutine that started it, even with a
deferred `recover` in `main` — it takes down the process:

```go
	defer func() {
		// This recover cannot see the goroutine's panic.
		if r := recover(); r != nil {
			fmt.Println("recovered in main:", r)
		}
	}()

	go func() { panic("from the goroutine") }()
	time.Sleep(100 * time.Millisecond)
	fmt.Println("never reached")
```

```
panic: from the goroutine

goroutine 19 [running]:
main.main.func2()
	/.../goroutinepanic/main.go:16 +0x25
created by main.main in goroutine 1
	/.../goroutinepanic/main.go:16 +0x3b
exit status 2
```

Every goroutine that can panic needs its own deferred recover, or none of them do. See
`panic-and-recover`.

## Interfaces are satisfied implicitly

An interface is a set of method signatures. A type satisfies it by having those methods —
no `implements`, no registration, no base class. This is `$obj->can('read')`, decided at
compile time at every assignment rather than hoped for at runtime.

```go
// The interface belongs to the consumer: Reporter needs one method, so it
// asks for one method. Accept an interface, return a struct.
func (r Reporter) WriteTo(w io.Writer, lines []string) (int, error) {
	total := 0
	for _, l := range lines {
		n, err := fmt.Fprintf(w, "%s%s\n", r.Prefix, l)
		if err != nil {
			return total, fmt.Errorf("writing report: %w", err)
		}
		total += n
	}
	return total, nil
}

// Compile-time proof that these satisfy io.Writer. Nothing "implements".
var (
	_ io.Writer = (*os.File)(nil)
	_ io.Writer = (*bytes.Buffer)(nil)
)

	r := Reporter{Prefix: "> "}
	var buf bytes.Buffer
	n, _ := r.WriteTo(&buf, []string{"first", "second"})
	fmt.Printf("wrote %d bytes to a buffer\n", n)
	fmt.Print(buf.String())

	r.WriteTo(os.Stdout, []string{"straight to stdout"}) // same func, a file
```

```
wrote 17 bytes to a buffer
> first
> second
> straight to stdout
```

`os.File` and `bytes.Buffer` were written without knowing `Reporter` would exist. The
`var _ io.Writer = (*os.File)(nil)` lines assert conformance at compile time without
allocating; use them when you want a build failure the moment a type stops satisfying an
interface it is meant to.

Two cultural rules follow, and they invert what most object-oriented backgrounds teach.
**Interfaces are small, and they belong to the consumer**: `io.Reader` and `io.Writer` have
one method each, and you define an interface in the package that *uses* it, listing only
the methods that package calls. There is no need to ship an interface beside every type in
case someone wants to mock it. **Accept interfaces, return structs**: parameters should be
the smallest interface that does the job, so callers can pass anything; results should be
concrete, so callers get full capability. A function taking `io.Reader` instead of a
filename gains stdin support for free and becomes testable with `strings.NewReader` instead
of a temporary file. See `accept-interfaces-return-structs` and `io-reader-writer`.

The dynamic escape hatch is `any` (the alias for `interface{}`) with type assertions and
type switches, which is what `ref($x)` becomes. It is occasionally right — decoding
arbitrary JSON, mainly — but reaching for it by default because Perl scalars were
polymorphic is the most common way converted code stays un-Go. See `implicit-interfaces` and
`type-assertions-and-switches`.

## Goroutines and channels are not `fork`

`go f(x)` runs `f(x)` concurrently in the same process, same address space, sharing every
variable. That is the whole difference from `fork`: no copy-on-write isolation, no separate
memory, no `waitpid`, no serialising results back through a pipe. It is not ithreads
either — nothing is cloned. Goroutines start with a couple of kilobytes of stack and grow;
hundreds of thousands are unremarkable.

Two consequences arrive immediately. **`main` waits for nobody**: when `main` returns the
process exits and every running goroutine dies mid-statement. **Everything is shared**, so
unsynchronised access to one variable from two goroutines is a data race — a bug even when
the output looks right.

Channels are typed pipes carrying values rather than bytes:

```go
	jobs := make(chan int)          // unbuffered: send blocks until a receive
	results := make(chan string, 8) // buffered: 8 sends before blocking

	var wg sync.WaitGroup
	for w := 1; w <= 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs { // ranges until the channel is closed
				results <- fmt.Sprintf("job %d squared is %d", j, j*j)
			}
		}()
	}

	for j := 1; j <= 5; j++ {
		jobs <- j
	}
	close(jobs) // tells every worker's range loop to end

	wg.Wait()      // the join; nothing like waitpid is involved
	close(results) // safe now: no sender remains
```

That is the direct replacement for `Parallel::ForkManager` with a result queue, and it is
roughly the whole pattern: a channel of work, N goroutines ranging over it, a
`sync.WaitGroup` for the join, a close to signal the end. Closing is the sender's job, never
the receiver's; sending on a closed channel panics; receiving from a closed channel yields
the zero value immediately, and `v, ok := <-ch` tells you which happened. (The
`wg.Add`/`defer wg.Done()` pair also has a shorter modern form, `wg.Go(func() { ... })`.)

Beyond that: `select` waits on several channel operations at once and is where timeouts
(`case <-time.After(d)`) and cancellation (`case <-ctx.Done()`) live; `context.Context` is a
cancellation and deadline token threaded as the first parameter through call chains,
replacing `alarm` plus `$SIG{ALRM}` outright; `sync.Mutex` guards struct fields where a
channel would be contrived; `sync.Once` is lazy initialisation. See `goroutines-not-fork`,
`channels-and-select`, `waitgroup-and-mutex`, `context-cancellation`. One non-negotiable
habit: if your program starts a goroutine, run its tests with `-race`.

## Packages, modules, and where code lives

Every `.go` file in a directory belongs to the same package, and a name is exported if and
only if it starts with a capital letter. `Parse` is public, `parse` is private. No
`Exporter`, no `@EXPORT_OK`, no underscore convention — and no reaching in: unexported means
genuinely inaccessible outside the package, not merely impolite.

```
report/
  go.mod                     module example.com/report
  cmd/report/main.go         package main       — the binary
  format/format.go           package format     — importable by anyone
  internal/parse/parse.go    package parse      — importable only within this module
```

```go
// Line parses one line. Exported: capital L.
func Line(s string) (Record, bool) {
	level, msg, found := strings.Cut(s, " ")
	if !found {
		return Record{}, false
	}
	return Record{Level: level, Msg: normalise(msg)}, true
}

// normalise is package-private: lowercase n, inaccessible outside this package.
func normalise(s string) string { return strings.TrimSpace(s) }
```

Reaching for the private name from another package in the same module is a compile error,
not a lint warning; and `internal/` is enforced by the toolchain, not by convention — a
package under `internal/` may be imported only by code rooted at `internal/`'s parent:

```
cmd/report/main.go:10:20: undefined: parse.normalise

package example.com/other
	main.go:6:2: use of internal package example.com/report/internal/parse not allowed
```

That is the mechanism CPAN never had. Anything under `internal/` is yours to rename,
restructure or delete without breaking anyone. Cyclic imports are a compile error too, which
Perl merely tolerates with scars. See `packages-and-exported-names`.

A module is a versioned tree of packages with a `go.mod` at its root. `go.mod` declares the
module path, the language version and the dependency requirements; `go.sum` records
cryptographic hashes of every module version in the build graph. Together they are roughly
`cpanfile` plus a lock file, except the lock is not optional and the hashes are checked on
every build. The workflow: `go get example.com/pkg@v1.2.3` adds a requirement, `go mod tidy`
adds what your imports need and drops what they do not, `go list -m all` shows the resolved
build list. There is no install step and no `local::lib`. Source is fetched into a module
cache under `go env GOMODCACHE`, content-addressed, shared across all your projects, and
**read-only**:

```
dr-xr-xr-x 9 ... /home/you/go/pkg/mod/golang.org/x/mod@v0.40.0
-r--r--r--     /home/you/go/pkg/mod/golang.org/x/mod@v0.40.0/LICENSE
```

You cannot patch a dependency in place the way you might edit something under `site_perl` at
three in the morning; the intended equivalents are a `replace` directive in `go.mod` while
you work, and a fork with a new module path when you ship. Version selection is not "newest
that satisfies": Go uses minimal version selection, so you get the highest version
*explicitly required* by anything in the graph, and it does not change until you change it.

Two more things worth knowing early. The standard library is unusually large and is the
first place to look — `net/http`, `encoding/json`, `database/sql`, `crypto/*`,
`text/template`, `os/exec` are all in the box, and the cultural default is to add a
dependency reluctantly (see `small-stdlib-philosophy`). And `go build` produces a single
statically linked binary with no runtime and no interpreter:

```
$ go build -o report ./cmd/report && file report
report: ELF 64-bit LSB executable, x86-64, ..., statically linked, ...
$ ldd report
	not a dynamic executable
```

Deployment is `scp`. No `@INC`, no perlbrew, no system-Perl drift. For a lot of migrations
that single fact is the entire business case. See `go-mod-vs-cpan`.

## `gofmt`, and why nobody argues about formatting

`gofmt` is `perltidy` with zero options: no configuration file, no line-width flag, no
brace-style setting. It reads Go and writes canonical Go, and the ecosystem uses it
unmodified. Feed it this:

```go
type user struct{ Name string
   Email string
      Active bool }
func main(){
  us := []user{ {Name:"a",Email:"a@example.com",Active:true} }
  for _,u:=range us { if u.Active { fmt.Println(strings.ToUpper(u.Name),u.Email) } }
}
```

and `gofmt -w` rewrites it as:

```go
type user struct {
	Name   string
	Email  string
	Active bool
}

func main() {
	us := []user{{Name: "a", Email: "a@example.com", Active: true}}
	for _, u := range us {
		if u.Active {
			fmt.Println(strings.ToUpper(u.Name), u.Email)
		}
	}
}
```

Tabs for indentation, aligned struct fields and trailing comments, one import per line.
`gofmt -l` lists files that differ from canonical form and `gofmt -d` shows the diff, which
is what CI runs. Two things to internalise rather than resent. The alignment is computed,
not typed: you do not maintain those columns, and a longer field name re-aligns the block
for you. And the opening brace must be on the same line as the `func` or `if` — not a style
preference but a consequence of automatic semicolon insertion, which would otherwise
terminate the statement.

The payoff is not beauty. It is that every Go file you will ever read is formatted the same
way, diffs contain only real changes, and no code review in this language has ever spent a
comment on brace placement. Your editor should run `gofmt` (or `goimports`, which also fixes
the import block) on save; converted code arrives gofmt-clean and should stay that way,
because unformatted Go reads as foreign. See `toolchain-gofmt-godoc`.

## Tests live next to the code

Tests are ordinary Go files named `*_test.go`, in the same directory and usually the same
package as the code they test. No TAP, no `prove`, no test library to choose: `testing` is
in the standard library and `go test` is the harness. The house style is the table-driven
test — a slice of case structs and a loop of subtests, the `@cases`-loop half of
`Test::More` culture, standardised:

```go
func TestCount(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"one word", "perl", 1},
		{"leading and trailing space", "   two words  ", 2},
		{"tabs and newlines", "a\tb\nc", 3},
		{"only whitespace", " \t\n", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Count(tt.in)
			if got != tt.want {
				t.Errorf("Count(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
```

```
$ go test ./wordcount/
ok  	scratch/wordcount	0.001s

$ go test -v ./wordcount/     (abridged)
--- PASS: TestCount (0.00s)
    --- PASS: TestCount/empty (0.00s)
    --- PASS: TestCount/leading_and_trailing_space (0.00s)
PASS
```

Add a wrong case and the subtest name tells you which, with no test numbering:

```
--- FAIL: TestCount (0.00s)
    --- FAIL: TestCount/deliberately_wrong (0.00s)
        wordcount_test.go:23: Count("one two") = 2, want 3
```

Notes that matter in practice:

- There are no assertion functions in the standard library. You write
  `if got != want { t.Errorf("got %v, want %v", got, want) }`. `t.Errorf` records a failure
  and continues; `t.Fatalf` stops that subtest. For deep comparisons,
  `github.com/google/go-cmp/cmp.Diff` is the community standard; `testify` is contested.
- Subtests are selectable by name: `go test -run 'TestCount/only_whitespace' ./...`.
- `t.TempDir()` gives a directory cleaned up automatically — `File::Temp` built into the
  harness. `t.Cleanup(fn)` is the general form.
- `go test ./...` runs every package in the module; results are cached, and an unchanged
  package prints `(cached)`. `-cover` needs no extra tooling.
- Documentation examples are tests. `func ExampleCount()` with an `// Output:` comment is
  compiled, run and compared, so examples cannot rot.

The `table-driven-tests` lesson takes this apart row by row and maps the rest of the
`Test::More` vocabulary onto it; `benchmarks-and-coverage` covers what else `go test`
does, which is benchmarks, coverage, fuzzing and profiles, all without a module to
install.

## The tools you will actually type

There is one binary. `go help` lists everything; these are the daily ones.

| Command | What it does | Perl analogue |
|---|---|---|
| `go build ./...` | compile everything; write a binary for `main` packages | `perl -c`, then packaging |
| `go run .` | compile and run, leaving no binary behind | `perl script.pl` |
| `go test ./...` | build and run every test in the module | `prove -r t/` |
| `go test -race ./...` | the same, with the race detector on | nothing |
| `go vet ./...` | static checks the compiler deliberately allows | `perlcritic`, but shipped and quiet |
| `gofmt -l .` / `go fmt ./...` | canonical formatting | `perltidy`, with no options |
| `go doc pkg.Name` | documentation from source, offline | `perldoc` |
| `go mod tidy` | reconcile `go.mod` with actual imports | editing `cpanfile` by hand |
| `go env GOMODCACHE` | where dependencies live | `perl -V:installsitelib` |

### `go vet`

`go vet` finds bugs the compiler is willing to allow. Its best-known check is `Printf`
argument agreement, which catches the whole class of formatting mistakes Perl never
diagnosed. This compiles cleanly:

```go
	c := Config{Name: "web", Port: 8080}
	fmt.Printf("%d listening on %s\n", c.Name, c.Port)
	fmt.Printf("%s\n")
```

```
vetbait/main.go:12:14: fmt.Printf format %d has arg c.Name of wrong type string
vetbait/main.go:13:14: fmt.Printf format %s reads arg #1, but call has 0 args
```

Run it anyway and `fmt` tells you in its own way — usefully, but much later:
`%!d(string=web) listening on %!s(int=8080)` followed by `%!s(MISSING)`. A subset of vet
runs automatically as part of `go test`, and it fails the build rather than warning:

```
vettest/x_test.go:8:25: (*testing.common).Errorf format %s has arg got of wrong type int
FAIL	scratch/vettest [build failed]
```

Vet also catches struct tags that will silently not work, a `sync.Mutex` copied by value,
unreachable code, and lost `context.CancelFunc`s. When a project wants more, `staticcheck`
is the usual next step. See `vet-and-staticcheck`.

### `go doc`

Documentation is the comment immediately above a declaration — no POD, no separate file, no
markup beyond a few conventions. `go doc` reads it offline, from source, for the standard
library and your own packages alike:

```
$ go doc strings.Fields
package strings // import "strings"

func Fields(s string) []string
    Fields splits the string s around each instance of one or more consecutive
    white space characters, as defined by unicode.IsSpace, returning a slice
    of substrings of s or an empty slice if s contains only white space.
    ...

$ go doc ./wordcount
package wordcount // import "scratch/wordcount"

Package wordcount counts words in a line of text.

func Count(s string) int
```

The convention is that a doc comment starts with the name it documents ("Count returns…")
and that a package has one `// Package x …` comment. The same text appears on pkg.go.dev.
Unexported names get no public documentation, which is another reason capitalisation carries
so much weight.

### The race detector

The tool with no Perl equivalent, and the reason to reach for it is that concurrent bugs do
not reproduce on demand. A counter incremented from 1000 goroutines with no synchronisation,
run three times, printed `972`, `990`, `972` — plausibly wrong, and on a smaller loop it
would print the right answer most of the time. With `-race`:

```
==================
WARNING: DATA RACE
Read at 0x00c000018178 by goroutine 17:
  main.main.func1()
      /.../racy/main.go:15 +0x7b

Previous write at 0x00c000018178 by goroutine 14:
  main.main.func1()
      /.../racy/main.go:15 +0x8d

Goroutine 17 (running) created at:
  main.main()
      /.../racy/main.go:13 +0x78
==================
595
Found 2 data race(s)
exit status 66
```

It names both goroutines, the line each was executing and where each was created, and exits
non-zero so CI notices. It reports only races it actually observes, so it finds nothing in
code paths your tests do not exercise — but it has almost no false positives, which makes
any report worth stopping for. It costs roughly five to ten times the CPU and a lot of
memory, so it is a test-and-staging tool, not a production flag. Correct output proves
nothing about a concurrent program; a clean `-race` run over a good suite is the closest
thing to proof you get. See `race-detector`.

## A Perl-to-Go cheat table

Every Go expression below was compiled. Slice and map operations assume `a []int`,
`m map[string]int`, `s string`.

| Perl | Go | Note |
|---|---|---|
| `my @a;` | `var a []int` | nil, and appendable as-is |
| `my %h;` | `m := map[string]int{}` | `var m map[string]int` is nil and panics on write |
| `push @a, 4` | `a = append(a, 4)` | must reassign |
| `pop @a` | `x := a[len(a)-1]; a = a[:len(a)-1]` | panics on empty; Perl returns `undef` |
| `shift @a` | `x := a[0]; a = a[1:]` | pins the backing array in memory |
| `unshift @a, 0` | `a = slices.Insert(a, 0, 0)` | O(n), same as Perl |
| `splice @a, 1, 1` | `a = slices.Delete(a, 1, 2)` | end index is exclusive |
| `$a[-1]` | `a[len(a)-1]` | no negative indexing |
| `$#a` / `scalar @a` | `len(a) - 1` / `len(a)` | |
| `@b = @a` | `b := slices.Clone(a)` | plain `b := a` aliases |
| `reverse @a` | `slices.Reverse(a)` | in place; Perl returns a new list |
| `@a[1..3]` | `a[1:4]` | half-open, and a view not a copy |
| `grep { $_ == 2 } @a` | `slices.Contains(a, 2)` | boolean case; otherwise an append loop |
| `first { ... } @a` | `slices.IndexFunc(a, f)` | returns `-1` when absent |
| `sort @a` | `slices.Sort(a)` | **order differs**: Perl's default is stringwise |
| `sort { $a->{k} cmp $b->{k} } @a` | `slices.SortStableFunc(a, f)` | plain `SortFunc` is not stable |
| `exists $h{k}` | `v, ok := m[k]` | |
| `delete $h{k}` | `delete(m, k)` | returns nothing; read first if needed |
| `sort keys %h` | `slices.Sorted(maps.Keys(m))` | `maps.Keys` alone is an iterator |
| `values %h` | `slices.Collect(maps.Values(m))` | |
| `$h{$k}++` | `m[k]++` | works from nothing in both |
| `while (my ($k,$v) = each %h)` | `for k, v := range m` | stateless, nestable, random order |
| `split /,/, $s` | `strings.Split(s, ",")` | trailing empties differ |
| `split ' ', $s` | `strings.Fields(s)` | exact match |
| `split /=/, $s, 2` | `strings.Cut(s, "=")` | returns before, after, found |
| `join ",", @a` | `strings.Join(a, ",")` | needs `[]string` |
| `"-" x 5` | `strings.Repeat("-", 5)` | |
| `index($s,"b") >= 0` | `strings.Contains(s, "b")` | |
| `$s =~ /^ab/` | `strings.HasPrefix(s, "ab")` | when the pattern is a literal |
| `chomp $s` | `strings.TrimSuffix(s, "\n")` | |
| `s/^\s+\|\s+$//g` | `strings.TrimSpace(s)` | |
| `0 + $s` / `"" . $n` | `strconv.Atoi(s)` / `strconv.Itoa(n)` | `Atoi` returns `(int, error)` |
| `qr/(\d+)/` | ``regexp.MustCompile(`(\d+)`)`` | compile once, at package scope |
| `($1) = $s =~ /re/` | `m := re.FindStringSubmatch(s)` | `m == nil` on no match; `m[0]` is `$&` |
| `$s =~ /re/g` | `re.FindAllString(s, -1)` | the `-1` means "all" |
| `s/re/<$1>/g` | `re.ReplaceAllString(s, "<$1>")` | prefer `${1}` when text follows |
| `\Q$s\E` | `regexp.QuoteMeta(s)` | |
| `die "msg"` | `return fmt.Errorf("msg")` | inside a function |
| `... or die "msg: $!"` | `if err != nil { return fmt.Errorf("msg: %w", err) }` | |
| `eval {}; if ($@)` | `if err != nil` | nothing to catch |
| `warn "msg"` | `fmt.Fprintln(os.Stderr, "msg")` | `log.Printf` adds a timestamp |
| `exit 1` | `os.Exit(1)` | skips deferred functions |
| `local $x = 1` | `saved := x; x = 1; defer func() { x = saved }()` | function scope, not block |
| `END { ... }` | `defer` in `main` | not run by `os.Exit` |
| `$ENV{HOME}` | `os.Getenv("HOME")` | `os.LookupEnv` for set-versus-unset |
| `@ARGV` | `os.Args[1:]` | `flag` package for options |
| `system("ls")` / `` `ls` `` | `exec.Command("ls").Run()` / `.Output()` | no shell involved |
| `ref($x) eq 'ARRAY'` | `if v, ok := x.([]any); ok` | type assertion |
| `$obj->can('read')` | `if r, ok := x.(io.Reader); ok` | interface assertion |
| `use constant PI => 3` | `const Pi = 3` | `iota` for enumerations |

Several rows have a lesson of their own: `fmt-and-verbs` for the printf family and the
verbs that no longer mean what they did, `strings-package` for the string builtins and
where `tr///` went, `strconv-parsing` for the numeric conversions, `sort-slice` for
sorting, `regexp-is-re2` for patterns, `time-layouts` for dates, `filepath-and-paths` for
paths and file tests, `flag-package` for `Getopt::Long`, `encoding-json` for JSON, and
`os-exec` for `system` and backticks.

## What you will miss, and what to do instead

### Regular expressions are RE2

Go's `regexp` is not a backtracking engine. It guarantees linear time in the length of the
input, and the price is that features requiring backtracking do not exist. These Perl
patterns all match:

```perl
"foofoo bar"    =~ /(\w+)\1/       # backreference   -> matched "foofoo"
"price: 30 USD" =~ /\d+(?= USD)/   # lookahead       -> matched "30"
"id=4711"       =~ /(?<=id=)\d+/   # lookbehind      -> matched "4711"
```

In Go two of those are not patterns at all:

```
error parsing regexp: invalid or unsupported Perl syntax: `(?=`
error parsing regexp: invalid escape sequence: `\1`
```

You get everything else, plus a guarantee: named captures (`(?P<name>...)` and
`(?<name>...)`), inline flags `(?i)`, `(?m)`, `(?s)`, Unicode classes `\p{Greek}`, POSIX
classes, and the certainty that no input makes a match take exponential time. Perl-style
regexes are a denial-of-service vector; RE2's missing features are the price of not having
one.

```go
// Compile once, at package scope. Perl did this for you behind /literal/.
var reLog = regexp.MustCompile(`^(?P<level>\w+)\s+\[(?P<ts>[^\]]+)\]\s+(?P<msg>.*)$`)

	m := reLog.FindStringSubmatch(line)
	if m == nil { // no match is nil, not an empty slice
		return
	}
	for i, name := range reLog.SubexpNames() {
		if name != "" {
			fmt.Printf("%s = %q\n", name, m[i])
		}
	}
	fmt.Println(reLog.ReplaceAllString(line, "${level}: ${msg}"))
```

```
level = "WARN"
ts = "2026-01-02T15:04:05Z"
msg = "disk almost full"
WARN: disk almost full
```

Practical notes. Write patterns in backquoted raw strings so backslashes stay single — the
analogue of choosing a Perl delimiter to avoid escaping. `MustCompile` panics on a bad
pattern, correct for a literal and wrong for a runtime-built pattern, where you want
`regexp.Compile` and its error. Compile at package scope, not inside a loop: a `MustCompile`
in a hot path is the most common Go regex performance bug. `\d`, `\w` and `\b` are
ASCII-only. `$1` in a replacement is greedy about name characters, so `"$1x"` looks for a
group named `1x` and expands to nothing — write `"${1}x"`. When a pattern genuinely needs
backreferences or lookaround, `dlclark/regexp2` provides them, at the cost of the
linear-time guarantee and with no match timeout unless you set one. See `regexp-is-re2`,
`mustcompile-pattern`, `submatch-and-named-groups`, `replace-and-expansion`.

### No context, so `wantarray` has nothing to be

```perl
sub ctx { return wantarray ? "list" : defined(wantarray) ? "scalar" : "void" }
my @a = ctx(); my $s = ctx();
print "$a[0] $s\n";     # prints: list scalar
```

One Perl sub serves three callers from one body. A Go function has one signature, so a
context-polymorphic sub becomes two functions with names that say what they do — `Lines()`
and `LineCount()`, `Hostname()` and `HostnameAll()`. That is documentation Perl never had,
and `my ($first) = f()` versus `my $count = f()` bugs become unwritable.

Two related losses. Lists do not flatten: `f(@a, @b)` passed one merged list, whereas Go has
no list type and a variadic call takes at most one trailing `slice...` — combine first with
`slices.Concat(a, b)`. And multiple return values are not a first-class list:
`func f() (int, error)` returns two values that must be assigned, discarded with `_`, or
passed straight to a call taking exactly those types. You cannot store them together and you
cannot splice them into a longer argument list.

### Nothing autovivifies

`my %h; $h{a}{b}{c} = 1;` builds the intermediate hashes. Go requires the path:

```go
	if h["a"] == nil {
		h["a"] = map[string]map[string]int{}
	}
	if h["a"]["b"] == nil {
		h["a"]["b"] = map[string]int{}
	}
	h["a"]["b"]["c"] = 1
```

Intermediate levels have types, and a type has to come from somewhere, so it cannot be
implicit. The better answer is usually to stop nesting: a struct with real fields, or a flat
map with a struct key, beats `map[string]map[string]map[string]int` every time. You gain
something too — Perl's autovivification fires on some *reads*, so `exists $h{x}{y}` silently
creates `$h{x}`, while Go's reads never mutate anything. See `nil-vs-undef` and
`maps-of-slices`.

### No `local`, no dynamic scoping

Covered above under `defer`: save the old value, assign the new, `defer` the restore.
Restoration happens at function exit rather than block exit, and there is no dynamic scoping
at all — callees see the change only because the variable is package-level. Across
goroutines the idea is meaningless, and a `local`-style global mutated while goroutines run
is a data race.

### No string increment, and no ternary

```perl
for my $s (qw(aa Az zz a9 Zz)) { my $t = $s; $t++; print "$s -> $t\n" }
```

```
aa -> ab
Az -> Ba
zz -> aaa
a9 -> b0
Zz -> AAa
```

In Go `++` applies to numbers only, and it is a *statement*, not an expression:

```
strinc/main.go:7:2: invalid operation: s++ (non-numeric type string)
ternary/main.go:7:10: invalid character U+003F '?'
```

If you were generating identifiers with `"aa" .. "zz"`, you need a small carry-propagating
function. If you were writing `$x = $i++ + ++$i`, the entire genre is gone and you will not
miss it. For the ternary, use an `if`, a small helper, or `cmp.Or` when the question is
"first non-zero value".

### No `sprintf` vector flags

`printf "%vd\n", v1.22.333` prints `1.22.333` in Perl. Go's `fmt` has no vector flag; its
`%v` means "default format for this value", a different thing entirely. Version strings get
parsed and formatted explicitly, or compared with `golang.org/x/mod/semver`. What Go's `fmt`
has that Perl's does not: `%+v` and `%#v` for structs, `%T` for the type, `%q` for a quoted
string, and vet checking every call.

```go
	p := Point{3, 4}
	fmt.Printf("%v %+v %#v %T\n", p, p, p, p)
	fmt.Printf("%q %08.3f|%-6d|%x|%b\n", "a\tb", 3.14159, 42, 255, 5)
	fmt.Printf("%*d\n", 6, 42)                    // width from an argument
	fmt.Printf("%[2]s %[1]s\n", "world", "hello") // explicit argument indexes
```

```
{3 4} {X:3 Y:4} main.Point{X:3, Y:4} main.Point
"a\tb" 0003.142|42    |ff|101
    42
hello world
```

### Sorting has no default, and Perl's default was stringwise

This one changes output. Perl's bare `sort` compares as strings even when every element is a
number: `sort (10,9,2,1)` gives `1,10,2,9`, while `sort { $a <=> $b }` gives `1,2,9,10`. Go
has no default comparison to get wrong, because sorting is a function call and the function
is chosen by element type:

```go
	people := []Person{{"Wall", "Larry", 71}, {"Pike", "Rob", 69}, {"Wall", "Alison", 70}}

	nums := []int{10, 9, 2, 1}
	slices.Sort(nums) // numeric, because the element type is int
	fmt.Println(nums)

	// Reproducing Perl's default stringwise sort, when byte-identical output matters:
	nums = []int{10, 9, 2, 1}
	slices.SortFunc(nums, func(a, b int) int {
		return cmp.Compare(strconv.Itoa(a), strconv.Itoa(b))
	})
	fmt.Println(nums)

	// cmp.Or chains comparators: last name, then first name. Stable, like Perl's sort.
	slices.SortStableFunc(people, func(a, b Person) int {
		return cmp.Or(cmp.Compare(a.Last, b.Last), cmp.Compare(a.First, b.First))
	})
	fmt.Println(people)

	// Descending: invert the comparator. Sorting a copy: slices.Sorted.
	slices.SortFunc(people, func(a, b Person) int { return cmp.Compare(b.Age, a.Age) })
	orig := []string{"pear", "apple", "fig"}
	fmt.Println(people, slices.Sorted(slices.Values(orig)))
```

```
[1 2 9 10]
[1 10 2 9]
[{Pike Rob 69} {Wall Alison 70} {Wall Larry 71}]
[{Wall Larry 71} {Wall Alison 70} {Pike Rob 69}] [apple fig pear]
```

If a converted script's sorted output no longer matches byte for byte, this is almost always
why, and Go's answer is almost always the one you wanted. Four things to know: `slices.Sort`
and `slices.SortFunc` sort **in place** and return nothing, so clone first if the original
order still matters; `SortFunc` is **not stable** while Perl's `sort` is, so use
`slices.SortStableFunc` when equal keys must keep input order; the comparator returns a
negative, zero or positive `int` exactly like `<=>` and `cmp`, and `cmp.Compare` produces
one; and `cmp.Or` chains comparators, replacing
`$a->{last} cmp $b->{last} || $a->{first} cmp $b->{first}` almost one for one. The older
`sort.Slice` takes a less-than predicate rather than a three-way comparator and is still
everywhere in existing code. See `sort-slice`.

### Smaller absences

- **No statement modifiers.** `$x++ if $cond` becomes a three-line `if`. There is no `unless`
  and no `until`; write `if !cond` and `for !cond`.
- **One loop keyword.** `for` plays `while` as `for cond {}`, infinite loop as `for {}`, and
  counted loop as `for i := range n`. See `range-is-not-foreach`: the single-variable
  `for x := range items` binds the *index*, the most common conversion bug of all.
- **No `@_` aliasing.** `sub bump { $_[0]++ }` modifies the caller's variable; Go arguments
  are always copies, and mutation needs an explicit pointer parameter. See
  `pointers-vs-references`.
- **No default arguments and no overloading.** One name, one signature. The
  named-arguments-hash idiom becomes an options struct; see `variadic-and-no-defaults`.
- **No `tie`, no `AUTOLOAD`, no runtime symbol table.** Where Perl installs a sub into a
  package at runtime, Go passes a function value or an interface — same wiring, moved from
  the symbol table into explicit parameters.
- **`switch` breaks by default.** No `last` needed; `fallthrough` is the explicit rarity.
  Cases can be arbitrary expressions, so `switch { case x > 10: ... }` replaces an if/elsif
  chain.
- **Inheritance is not there.** Struct embedding promotes fields and methods and looks like
  inheritance, but there is no virtual dispatch through the embedded type; the polymorphism
  tool is interfaces. See `structs-and-embedding` and `methods-and-receivers`.

## Where to go next

**Start here.** [A Tour of Go](https://go.dev/tour/) is the interactive introduction; skip
ahead to "Methods and interfaces". [Effective Go](https://go.dev/doc/effective_go) is dated
in places and still the best statement of *why* Go code looks the way it does.
[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) is the checklist real
reviewers use — short, and it will change how you name things.

**The topics above, in depth.**

- [Go Slices: usage and internals](https://go.dev/blog/slices-intro) and
  [Arrays, slices (and strings): The mechanics of 'append'](https://go.dev/blog/slices)
- [Go maps in action](https://go.dev/blog/maps) —
  [Strings, bytes, runes and characters in Go](https://go.dev/blog/strings)
- [Error handling and Go](https://go.dev/blog/error-handling-and-go),
  [Errors are values](https://go.dev/blog/errors-are-values), and
  [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors) for `%w`, `errors.Is`
  and `errors.As`
- [Defer, Panic, and Recover](https://go.dev/blog/defer-panic-and-recover) —
  [Go Concurrency Patterns: Pipelines and cancellation](https://go.dev/blog/pipelines)
- [Organizing a Go module](https://go.dev/doc/modules/layout) and
  [Managing dependencies](https://go.dev/doc/modules/managing-dependencies)
- [go fmt your code](https://go.dev/blog/gofmt) —
  [Data Race Detector](https://go.dev/doc/articles/race_detector) —
  [Go Doc Comments](https://go.dev/doc/comment)

**Reference.** [The specification](https://go.dev/ref/spec) is short enough to read end to
end in an afternoon, which is not something you could say about perlop plus perldata plus
perlsub. [The modules reference](https://go.dev/ref/mod) covers versioning, the module
cache, `replace` and minimal version selection. [The FAQ](https://go.dev/doc/faq) has the
design rationale, including
[why a nil error is not always nil](https://go.dev/doc/faq#nil_error). Browse the
[standard library index](https://pkg.go.dev/std) once; it is larger than you expect and the
answer is usually in it.

**Package documentation you will open constantly:**
[`fmt`](https://pkg.go.dev/fmt), [`strings`](https://pkg.go.dev/strings),
[`strconv`](https://pkg.go.dev/strconv), [`errors`](https://pkg.go.dev/errors),
[`slices`](https://pkg.go.dev/slices), [`maps`](https://pkg.go.dev/maps),
[`cmp`](https://pkg.go.dev/cmp), [`sort`](https://pkg.go.dev/sort),
[`regexp`](https://pkg.go.dev/regexp) with
[`regexp/syntax`](https://pkg.go.dev/regexp/syntax) for the exact accepted patterns,
[`testing`](https://pkg.go.dev/testing), [`io`](https://pkg.go.dev/io),
[`os`](https://pkg.go.dev/os), [`sync`](https://pkg.go.dev/sync),
[`context`](https://pkg.go.dev/context), and
[`cmd/go`](https://pkg.go.dev/cmd/go) with [`cmd/vet`](https://pkg.go.dev/cmd/vet) for the
full command and check lists.
