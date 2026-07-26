# 24-autoload-accessors

Accessors that do not exist until first call: AUTOLOAD materializes real
methods into the symbol table, DESTROY is guarded, and a subclass's real
method shadows the would-be accessor.

## Constructs exercised
- `our $AUTOLOAD` + `sub AUTOLOAD` catching undefined method calls;
  stripping the package with `s/.*:://`
- the mandatory `return if $name eq 'DESTROY';` guard (without it, object
  teardown triggers AUTOLOAD -- a classic production bug)
- symbol-table surgery: `no strict 'refs'; *{__PACKAGE__."::$name"} = sub`
  installing a closure over `$name` as a real method, then re-dispatching
  the original call (`$self->$name(@_)`)
- observable metaprogramming: `can('title')` flips no -> yes after first
  use; the `@generated` audit records materialization ORDER
- dual getter/setter closure (`shift if @_` pattern)
- whitelist `%ALLOWED` with die-on-unknown accessor
- subclass with a REAL `rating` method shadowing AUTOLOAD generation,
  calling `SUPER::rating()` which lands in the parent's *generated* method
- symbolic method dispatch via a variable: `$signed->$field`
- `'*' x ($rating // 0)` string repetition with defined-or

## Conversion challenges
- AUTOLOAD is dynamic method synthesis -- flatly impossible in static Go;
  the converter must recognize the PATTERN (whitelisted accessors) and
  emit ordinary getters/setters, while preserving observable behavior:
  the error message for unknown fields and, harder, the can() flip and
  generation-order audit, which have no faithful static equivalent and
  demand a documented semantic decision
- method calls through a runtime string (`$signed->$field`) -> switch
  statement or reflection
- symbol-table assignment and `no strict 'refs'` simply do not translate;
  a naive converter crashes here -- this entry is a canary for "recognize
  intent, not syntax"
- SUPER:: into a generated method: inheritance + codegen interplay

## Go teaching opportunities
- why Go chose explicit methods; struct tags/codegen (`go generate`) as
  the principled analogue of runtime accessor generation
