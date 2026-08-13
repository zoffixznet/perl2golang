# 14-args-aliasing: `@_` aliases the caller's variables

Group: **B - convertible only with an approximation that changes semantics**

## Construct
`sub bump { $_[0]++; $_[1] .= "!" }` (line 7) mutates its CALLER's `$n` and
`$s`. `blank_all` empties every element of the caller's array through the
aliases in `@_`. `chompy` is the classic real-world idiom: `chomp $_[0]`
modifies the caller's string.

## Why naive Go conversion changes semantics
Go arguments are copies. The obvious translation
(`func bump(a int, b string)`) silently loses every mutation: `n` stays 5, `s`
stays `hi`, the row stays `a b c`, the line keeps its newline. Nothing crashes -
the program is just wrong, which makes this one of the most dangerous silent
traps in real scripts.

## What the converter should do
- Category: **approximate**, driven by a mandatory analysis:
  - For each sub, determine whether any element of `@_` is written
    (`$_[n]` as lvalue, `++`, `.=`, `chomp $_[0]`, `for (@_)` with `$_`
    modified, passing `@_` onward to another mutating sub).
  - Mutating subs get pointer parameters (`func bump(a *int, b *string)`), and
    every call site is rewritten to pass addresses. Report entry per sub.
  - Non-mutating subs convert to value parameters (the common fast path).
  - If the analysis cannot decide (e.g. `@_` escapes into a code ref), refuse
    the sub with a diagnostic rather than guess "copy".
- The unacceptable outcome: converting a mutating sub with value parameters and
  no diagnostic.

## Ideal diagnostic (word for word)
> input.pl:7: warning P2G-W304: sub 'bump' writes to @_ elements, which alias
> the caller's variables in Perl. Converted with pointer parameters; call sites
> rewritten to pass addresses (see report). If this sub is ever called with a
> literal or a temporary, that call would have been a runtime error in Perl and
> is now a compile error in Go.

## What a human should do instead
Make mutation explicit: return the new values (`($n, $s) = bump($n, $s)`), or
take references in the Perl source first (`sub bump { my ($nref, $sref) = @_ }`)
so the conversion is mechanical.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `n=6 s=hi!`, `row=[  ]` (three blanked elements),
`line=<text>` (newline chomped in the caller). A value-parameter conversion
prints `n=5 s=hi`, `row=[a b c]`, `line=<text\n...>` - every line differs.
