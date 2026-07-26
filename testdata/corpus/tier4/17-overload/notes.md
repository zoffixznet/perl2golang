# 17-overload: overloaded operators

Group: **B — convertible only with an approximation that changes semantics**

## Construct
Package `Money` overloads `+`, `*`, `""` (stringification) and `==` (line 8).
`$a + $b` dispatches to a method; `"sum: $sum\n"` calls the `""` handler;
`$sum == 4.00` compares an object against a plain number via the `==` handler
(which itself branches on whether the other operand is a ref).

## Why naive Go conversion changes semantics
Go has no operator overloading. A converter that translates `$a + $b`
numerically will add ADDRESSES or zero-values; one that stringifies an object
with `%v` prints a struct dump instead of `$4.00`. Both are silent corruption.
The conversion is only possible by resolving, at every operator site, whether an
operand can be an overloading object — a type-inference problem, undecidable in
general (values flow through untyped containers).

## What the converter should do
- Category: **shim** with a static prerequisite:
  - Where operand types are statically known to be `Money`, lower operators to
    method calls: `a.Add(b)`, `a.MulInt(3)`, `sum.String()` in interpolation,
    `sum.EqNum(4.00)`.
  - Where an operand's type cannot be proven (could be object or number at
    runtime), the converter must refuse that statement — a diagnostic naming
    the expression — rather than pick numeric or object semantics.
- The `""` overload must also apply anywhere the object reaches print/concat/
  hash-key position; the report must list each rewritten site.
- Forbidden: converting any arithmetic on a possibly-overloaded value with
  native Go operators and no diagnostic.

## Ideal diagnostic (word for word)
> input.pl:23: warning P2G-W307: '$a + $b' uses the '+' overload of package
> Money (declared input.pl:8). Lowered to the method call Add. All operator
> sites for Money in this file were statically typed and lowered (see report);
> if Money values ever flow through untyped containers, re-run conversion on
> the affected code.

## What a human should do instead
Name the operations: `add($a, $b)` / `$m->to_string` in Perl, ordinary methods
in Go. Operator sugar is the only thing lost.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `sum: $4.00`, `tripled: $4.50`, `equal: yes`.
`$4.00` (formatted cents) proves stringification went through the overload;
`equal: yes` proves the mixed object==number comparison multiplied 4.00 by 100.
