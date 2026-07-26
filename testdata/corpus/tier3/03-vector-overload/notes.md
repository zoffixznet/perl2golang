# 03-vector-overload

3-D vector class blessed from an ARRAY reference with rich `use overload`.

## Constructs exercised
- object backed by a blessed arrayref (`bless [@xyz]`), element access via
  `$_[0][0]`-style accessors
- `use overload` for `+ - * == != <=> "" bool neg`
- swapped-operand handling: third arg to `-` and `<=>` handlers, `2 * $v`
- one operator (`*`) polymorphic on operand type: scalar scaling vs dot product
  returning a plain number (result type depends on runtime `ref`/`isa` check)
- overloaded `""` firing inside string interpolation and `printf %s`
- overloaded `bool` controlling truthiness of an object in `? :`
- overloaded `<=>` used directly by `sort { $a <=> $b }`
- statement-modifier `for` with `or return` inside (`equals`)
- `map` over `0 .. 2` building new objects; unary minus via `neg`

## Conversion challenges
- Go has no operator overloading at all: every overloaded op must become a
  named method, and every use site (`$v + $i * 2`, interpolation, `sort`)
  must be rewritten to call them -- precedence of the original expression
  must be preserved manually
- `*` returning either a `Vec3` or a `float64` depending on operand type
  forces either two methods (`Scale`, `Dot`) and call-site disambiguation
- implicit stringification in `"i=$i"` must become explicit `String()` calls
  (fmt.Stringer helps only inside fmt verbs)
- object truthiness (`bool` overload) has no Go analogue
- array-backed object vs the hashref pattern used elsewhere in the corpus

## Go teaching opportunities
- fmt.Stringer, methods on small value types, `[3]float64` arrays
- sort.Slice with a magnitude comparator; explicit epsilon-free float compare
