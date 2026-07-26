# 11 - closures, counters and generators

## What this exercises
Subs that capture and keep private state: counters, a set of closures sharing
one variable, iterators, and the difference between `foreach my $x` (fresh
binding per iteration) and a C-style `for` (one shared variable).

## Perl constructs
- `sub make_counter { my $n = $start; return sub { ... $n += $step ... } }`
- multiple closures over the *same* lexical (`$deposit`, `$withdraw`, `$peek`
  all see one `$balance`)
- returning a list of code refs and destructuring it
- `return undef` versus `return` and the `defined` check at the call site
- **`for my $name (...)` gives each closure its own binding**, so the three
  printers report `alpha`, `beta`, `gamma`
- **`for (my $i = 0; ...)` shares one variable**, so all three closures report
  `3` - the two loops are side by side deliberately
- a generator: `return if $idx >= @items;` (empty return -> `undef` in scalar
  context) driving `while (defined(my $x = $it->()))`
- a memoising higher-order function taking and returning code refs
- `map { $tick->() } 1 .. 5` calling a closure inside `map`
- `**` exponentiation

## Go concepts a converter must teach
- Go closures capture variables the same way, so most of this converts well -
  `func() int { n += step; return n - step }`.
- The loop-variable question is version-dependent in Go: before 1.22 the loop
  variable was shared (matching the C-style case); from 1.22 each iteration has
  its own copy (matching Perl's `foreach my`). A converter must pin the target
  Go version or emit an explicit `x := x` shadow.
- Perl's C-style `for` genuinely shares the variable, so a faithful conversion
  needs a variable declared *outside* the Go loop.
- `return;` yielding `undef` in scalar context but `()` in list context is the
  iterator-exhausted signal. Go's idiom is `(value, ok)` - the converter must
  add the second return value and rewrite every call site.
- Returning several closures that share state is a struct with methods in Go;
  recognising that `make_account` is really a constructor is the interesting
  transformation.
- `%cache` inside a closure is a `map` captured by reference - fine in Go, but
  not safe for concurrent use, which is worth a generated comment.
