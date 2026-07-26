# 01-inventory-manager

Classic hashref-based OO in a single file with multiple `package` blocks.

## Constructs exercised
- `bless` on a hashref; two collaborating classes in one file (`Inventory::Item`, `Inventory`)
- `ref($proto) || $proto` constructor idiom -- constructor invoked both as class
  method and on an existing instance
- class-level state (`my $ITEMS_CREATED` closed over by methods) and a class
  method that dies when invoked on an instance
- read-only vs read/write accessors (`price($new)` set-when-args pattern)
- method chaining (`sell(25)->restock(5)`)
- `can` / `isa` duck-typing checks, `ref $obj` as class name
- `die` with `sprintf`-built message, caught with `eval { ... ; 1 } or $err = $@`
- `//` defined-or defaults in constructor args
- report formatting with `sprintf` field widths, `'-' x 57`, sorted hash keys

## Conversion challenges
- Perl "class" = package + blessed ref; Go needs a struct + methods, and the
  `ref($proto) || $proto` dual-mode constructor has no direct analogue
- class method vs instance method distinction is runtime (`ref $class`) not
  compile-time
- `can`/`isa` require either reflection or interface assertions in Go
- accessor that is getter *and* setter depending on arity
- package-level mutable state shared across all instances (Go: package var)
- numeric formatting must match Perl's `%8.2f` exactly

## Go teaching opportunities
- struct + constructor func + pointer receivers; errors instead of `die`
- interface satisfaction checks replacing `can`; type switch replacing `isa`
- returning the receiver for chaining
