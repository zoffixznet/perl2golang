# 02-shape-hierarchy

Multi-file OO hierarchy: `Shape` (abstract base) <- `Rectangle` <- `Square`,
plus `Circle`, driven from `input.pl` via `use lib '.'`.

## Constructs exercised
- multi-file program: four `.pm` modules loaded with `use lib '.'`
- `use parent -norequire`, old-school `our @ISA = (...)`, and an inline
  `package Blob` block declared inside the driver
- two-level inheritance with `SUPER::` calls that skip a level of storage
  (Square's init forwards `side` as width/height to Rectangle's init)
- abstract base enforcement: `die if $class eq __PACKAGE__`, and base
  "pure virtual" methods that die with `ref($self)` in the message
- template-method pattern: `new` calls overridable `init`
- `use constant PI` inside Circle; `**` exponentiation
- polymorphic sort with `<=>` chain and tie-break; `List::Util sum0` over `map`
- `isa` across a 3-deep chain, `can`, `eval {} ; 1` idiom for catching `die`
- class-level serial counter in the base shared by all subclasses

## Conversion challenges
- Perl inheritance is dynamic method resolution over `@ISA`; Go has no
  inheritance -- needs embedding + interface design, and `SUPER::` has no
  direct equivalent (must call the embedded type's method explicitly)
- the base-class `new`/`init` template pattern inverts: in Go the constructor
  cannot dispatch virtually into the subclass without an interface param
- `isa('Rectangle')` on a Square must stay true after conversion
- an anonymous subclass (`Blob`) defined at runtime inside main
- shared serial counter must live in one place despite four "classes"

## Go teaching opportunities
- interface (`Area() float64; Perimeter() float64`) + struct embedding
- explicit `Base.Describe(self)` style delegation replacing SUPER
- sentinel errors for "abstract" misuse; sort.Slice with tie-break
