---
id: statements-vs-expressions
title: Go separates statements from expressions, and Perl does not
tags: [control-flow, expressions, idiom]
perl_triggers: [do BLOCK, "do { }", if as expression, statement modifier, block value]
severity: info
prerequisites: [static-types-and-zero-values, var-vs-short-declaration]
---

In Perl almost everything is an expression. A block has a value, `if` has a value, an assignment has a value, and `do { ... }` exists precisely so you can put a block where a term belongs. Go draws the line the other way round and keeps it hard: `if`, `for` and `switch` are statements, they produce nothing, and there is no `do` block to borrow a value from. This is not a missing feature. It is the reason a Go function reads top to bottom with no work hidden inside a condition, and once you stop looking for the expression form the replacement is shorter than you expect.

## The Perl you know

```perl
# a block as a term: run some setup, hand back the last value
my $text = do {
    open my $fh, '<', $file or die "open $file: $!\n";
    local $/;
    <$fh>;
};

# a conditional as a term
my $bucket = do {
    if    ($n < 10)  { 'tiny' }
    elsif ($n < 100) { 'medium' }
    else             { 'huge' }
};
```

Both of these are one statement in Perl. The block is evaluated, and its value is whatever its last evaluated statement produced, which is why neither has a `return` in it.

## The Go you write

There are exactly two shapes, and between them they cover every `do` block you will meet.

**Setup, then the value.** The statements come out of the block and stand where the block was; the final expression is what gets assigned. Nothing wraps them.

**A conditional value.** Declare the variable first, then assign it on every path. The declaration states the type once, and the compiler makes sure every path produces one.

Compiled and run as shown:

```go
package main

import (
	"fmt"
	"io"
	"strings"
)

func main() {
	threshold := 42

	// A conditional value: declared once, assigned on every path.
	var bucket string
	switch {
	case threshold < 10:
		bucket = "tiny"
	case threshold < 100:
		bucket = "medium"
	default:
		bucket = "huge"
	}
	fmt.Println(bucket)

	// Setup, then the value the setup produced.
	b, err := io.ReadAll(strings.NewReader("one\ntwo\n"))
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}
	text := string(b)
	fmt.Printf("%d bytes, %d lines\n", len(text), strings.Count(text, "\n"))
}
```

```
medium
8 bytes, 2 lines
```

A `switch` with no subject and boolean cases is the Go spelling of an `if`/`elsif` chain, and it is what a Go developer reaches for once there are three arms. Two arms stay an `if`/`else`.

Reaching for the expression form directly does not compile, and the error is a parse error rather than a type error, which tells you how deep the rule goes:

```go-invalid
package main

import "fmt"

func main() {
	x := if true { 1 } else { 2 }
	fmt.Println(x)
}
```

```
./sample.go:6:7: syntax error: unexpected keyword if, expected expression
```

## The sub body is an expression too

The same rule runs one level up. A Perl sub yields whatever it evaluated last, so the `return` is optional and experienced Perl leaves it out whenever the sub is short. The three shapes below are all one line, and none of them says what it hands back:

```perl
package Counter;
sub new   { bless { n => 0, step => 1 }, shift }   # value: the new object
sub total { $_[0]{n} }                             # value: a field
sub twice { double( double( $_[0] ) ) }            # value: a call's result
```

Go has no such rule anywhere: a function with results must reach a `return`, and a function without results has no value to give. So each of those grows a `return`, and the type it returns becomes part of the function's signature — which is the point, because that signature is now the documentation the Perl never had.

The constructor is worth reading closely, because it is the one shape Perl developers expect Go to have and it does not. Go has no constructors, no `new` keyword for your own types, and no way to hook object creation. A plain function returning `*T`, named `New` followed by the type name, is the entire convention:

```go
package main

import "fmt"

// Counter is what `package Counter` and its `bless` became.
type Counter struct {
	N    int
	Step int
}

// NewCounter is the whole of `sub new { bless { n => 0, step => 1 }, shift }`.
func NewCounter() *Counter {
	return &Counter{N: 0, Step: 1}
}

func (c *Counter) Bump() int {
	c.N += c.Step
	return c.N
}

// Total is the accessor `sub total { $_[0]{n} }`, and in Go it should not
// exist at all: N is exported, so a caller reads it directly.
func (c *Counter) Total() int {
	return c.N
}

func main() {
	c := NewCounter()
	for range 3 {
		c.Bump()
	}
	fmt.Printf("counter reached %d\n", c.Total())
	fmt.Printf("or just read the field: %d\n", c.N)
}
```

```
counter reached 3
or just read the field: 3
```

Two habits to bring across and one to drop. Bring across: the constructor sets every field that has a non-zero starting value, and says nothing about the ones that start at zero, because Go already did. Bring across: `NewCounter` returns `*Counter` rather than `Counter`, so every caller is on the pointer side and the mutating methods work without a surprise (`methods-and-receivers` has the rule). Drop: the accessor. `Total` earns its place only when it computes something or when an interface has to promise it; a plain field is already a public interface in Go, and adding a method later changes no caller.

### When the last thing in the sub is an if

The awkward version of that rule is a sub whose whole body is a conditional, which is how a classifier gets written when nobody wants to type `return` four times:

```perl
sub classify {
    my ($n) = @_;
    if    ($n < 0)  { 'negative' }
    elsif ($n == 0) { 'zero' }
    else            { 'positive' }
}
```

There is no `do` here and no assignment: the `if` is the sub's last statement, so the branch that runs is the sub's answer. In Go the `if` is a statement and has no answer, so the `return` moves inside each branch, which is where the value always was.

```go
package main

import "fmt"

// classify is what a sub ending in an if/elsif/else becomes: the return moves
// inside each branch, which is where the value always was.
func classify(n int) string {
	if n < 0 {
		return "negative"
	} else if n == 0 {
		return "zero"
	}
	return "positive"
}

// The same decision written for a function that has more to do afterwards.
// One variable, assigned on every path, and the compiler checks that.
func bucket(n int) string {
	label := "small"
	switch {
	case n > 100:
		label = "huge"
	case n > 10:
		label = "big"
	}
	return label
}

func main() {
	for _, n := range []int{-4, 0, 7} {
		fmt.Print(classify(n), " ")
	}
	fmt.Println()
	for _, n := range []int{5, 50, 500} {
		fmt.Print(bucket(n), " ")
	}
	fmt.Println()
}
```

```
negative zero positive 
small big huge 
```

Both shapes are worth having. Early returns suit a function that is deciding and nothing else, and the trailing `return "positive"` with no `else` above it is the form Go developers write, because the last case is not really a case. The single variable assigned on every path suits a function that carries on afterwards, and it is the shape that survives someone adding a fourth arm.

## What the operator hands back

Separating statements from expressions has a second consequence, and it is the one that produces wrong answers rather than compile errors: several Perl operators hand back a value that Go's nearest equivalent does not.

`&&` and `||` are the biggest. In Perl they answer with an **operand** -- the first false one, or the last -- which is exactly why `$a || $b || 'fallback'` is a defaulting chain. Go's are strictly boolean and answer with nothing else, so the chain becomes a variable and a sequence of `if`s. That reads longer, and it makes the short-circuit visible, which is a fair trade.

`push` answers with the array's new length, `s///` answers with the number of replacements, `s///r` answers with the edited copy, and `chop` answers with the character it removed. None of those is what the Go call gives you, and each is a line of extra code:

```go
package main

import (
	"fmt"
	"regexp"
	"strings"
)

var errPattern = regexp.MustCompile(`ERROR`)

func main() {
	// $picked = $empty || $name || 'fallback'
	empty, name := "", "ada"
	picked := empty
	if picked == "" {
		picked = name
	}
	if picked == "" {
		picked = "fallback"
	}
	fmt.Println(picked)

	// my $n = push @queue, 4, 5
	queue := []int{1, 2, 3}
	queue = append(queue, 4, 5)
	fmt.Println(len(queue), queue)

	// my $hits = ( $log =~ s/ERROR/FATAL/g )
	log := "ERROR ERROR WARN ERROR"
	hits := len(errPattern.FindAllString(log, -1))
	log = errPattern.ReplaceAllString(log, "FATAL")
	fmt.Println(hits, log)

	// my $trimmed = $orig =~ s/^\s+|\s+$//gr
	orig := "  padded  "
	trimmed := strings.TrimSpace(orig)
	fmt.Printf("[%s] [%s]\n", orig, trimmed)
}
```

```
ada
5 [1 2 3 4 5]
3 FATAL FATAL WARN FATAL
[  padded  ] [padded]
```

Two things are worth noticing. The count has to be taken **before** the replacement, because afterwards there is nothing left to count -- which is a good reminder that `ReplaceAllString` is a function of its input rather than an edit. And `s///r` needs no special form at all: a Go string cannot be changed in place, so every replacement already returns a new one and leaves the original alone. Perl needed a modifier for the behaviour Go gives by default.

Where a value like this is genuinely wanted, `cmp.Or(a, b, "fallback")` covers the defaulting chain for comparable types in one call, returning the first non-zero argument. It is the closest thing Go has to `||` as a value, and it evaluates all its arguments, which the chain of `if`s does not.

## The mismatch

Perl's `do` block and Go's statements differ in three ways worth holding on to.

**Scope.** Perl's block is a scope, so a `my` inside it disappears at the closing brace. When the statements are lifted out, those variables live on in the surrounding function. That is usually what you want, and where it is not, Go has a bare `{ ... }` block that scopes exactly the same way.

**The value of a conditional with no `else`.** Perl hands back `undef` on the path that matched nothing. Go has no `undef`, so the declared variable holds its zero value on that path, and the empty string is not distinguishable from an empty string that was assigned. If the difference matters, declare a pointer and let `nil` mean absent, as `nil-vs-undef` describes.

**The wrapper you are tempted to write.** Go does have an expression that runs a block: an immediately-called function literal, `func() string { ... }()`. It works, it is legal, and it is almost always the wrong choice in converted code, because it puts a function boundary in the middle of a line for no reason and stops `return` inside it meaning what a reader expects. Use it only where the block genuinely must not run yet, for instance behind a `sync.Once` or as the argument to `defer`.

One habit follows from all of this. In Perl, a value's setup often hides inside the expression that consumes it; in Go, the setup gets its own lines and the value gets a name. The name is not clutter. It is the thing the next reader searches for.

Further reading: https://go.dev/ref/spec#Statements
