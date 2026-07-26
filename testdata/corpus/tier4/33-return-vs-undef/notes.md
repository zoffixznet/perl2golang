# 33-return-vs-undef: `return;` vs `return undef;` in list context

Group: **C — convertible, but the naive conversion is subtly wrong**

## Construct
`return;` (line 8) yields the EMPTY LIST in list context and undef in scalar
context. `return undef;` (line 9) yields a ONE-ELEMENT list (undef) in list
context. Assigned to arrays: 0 elements vs 1 element (lines 11-12). Inside a
hash constructor (line 17) the empty list makes everything SHIFT:
`(name => bare(), age => 30)` flattens to `("name", "age", 30)`, producing
keys `name` (value "age") and `30` (value undef) — observed keys `30,name`.
With `return undef;` the intended `age,name` keys appear. In scalar context
the two are indistinguishable (line 24).

## Why the naive conversion is subtly wrong
Go functions return fixed shapes; a converter that maps both forms to
"return nil/zero" collapses a real semantic distinction. The hash-shift
behaviour is the nasty part: it depends on list flattening, which Go does not
have, so the converter's list model must represent "empty list return"
faithfully or the constructed map gets the WRONG KEYS while compiling
cleanly.

## What the converter should do
- Category: **convert-verify**: the internal list model must distinguish
  `return;` (empty list) from `return undef;` (one undef). Where the call
  site is in list context (array assignment, hash constructor, function
  args), splice the returned list by its actual length. Where scalar
  context, both become undef.
- The hash constructor from a flattened list with an odd/shifted shape must
  reproduce Perl's pairing (and the converter should emit a warning mirroring
  Perl's "Odd number of elements" when it can prove it, as here).
- Forbidden: treating `return;` as `return undef;` (or vice versa) anywhere
  a list context can observe the difference.

## Ideal diagnostic (word for word)
> input.pl:17: warning P2G-W409: 'bare()' returns the empty list here, so
> the hash constructor pairs shift: keys become ("name" => "age", 30 =>
> undef). This matches Perl but is almost certainly a bug in the source;
> consider 'return undef;' in 'bare' or filtering the list.

## What a human should do instead
In Perl style guides: never `return undef;` for "no result" — use `return;`
— but when a one-element list is the contract, say so. When porting, replace
list-shape tricks with explicit multi-value returns
(`func f() (string, bool)`).

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `bare in list:  0 elements`,
`undef in list: 1 elements`, `keys:  30,name`, `keys2: age,name`,
`scalar ctx: both undef`. The `keys:  30,name` line is the shifted-hash
smoking gun.
