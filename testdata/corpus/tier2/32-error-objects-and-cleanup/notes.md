# 32 - exception objects, error dispatch and cleanup

## What this exercises
Structured errors: dying with a blessed object, with a subclass, and with a
plain hashref; dispatching on the error's type; wrapping and rethrowing with
added context; and running cleanup on both the success and the failure path
via a destructor-based guard. **This entry exits non-zero (1).**

**expected_exit:** `1`

## Perl constructs
- `package` statements inside the script file, with `our @ISA` for inheritance
- `bless $self, $class` and accessor subs reading `$_[0]{field}`
- **`die $object`** - dying with a reference rather than a string
- three different thrown types from one function: `MyApp::Error`,
  `MyApp::Error::NotFound` (a subclass) and a bare `{ code => ... }` hashref
- `blessed($err)` from `Scalar::Util` to tell an object from a plain ref
- `$err->isa('MyApp::Error::NotFound')` for subclass dispatch, ordered
  most-specific-first
- `ref $err eq 'HASH'` for the unblessed case, and a string fallback
- **a guard object with a `DESTROY` method**, so `commit`/`rollback` runs when
  the lexical goes out of scope - including when the scope is left by `die`
- a wrapper sub that catches, builds a new error carrying the cause, and
  rethrows
- `exit($failures > 0 ? 1 : 0)`
- `sprintf` inside a logging helper that both prints and accumulates

## Go concepts a converter must teach
- Error objects map well: a struct implementing `error`, with `errors.As` for
  the type dispatch and `%w` wrapping for the cause chain. `isa` on a subclass
  becomes `errors.As(err, &notFound)`.
- Perl's inheritance is `@ISA` linearisation; Go has no inheritance, so a
  subclass hierarchy becomes embedding plus interface satisfaction, and
  most-specific-first dispatch must be preserved manually.
- Dying with a **plain hashref** (no class) is the awkward case - it becomes an
  anonymous struct or a `map[string]any` wrapped in an error type.
- **`DESTROY` is deterministic in Perl** (refcounting), which is what makes the
  guard pattern work. Go has no destructors; `runtime.SetFinalizer` is not a
  substitute. The correct translation is `defer`, but `defer` fires at function
  exit rather than scope exit, so a guard created inside a block needs the
  block extracted into its own function.
- The commit/rollback flag read by the destructor becomes a named return or a
  captured variable in the deferred closure - and `defer` captures by
  reference, so the closure sees the final value, which is what is wanted here.
- `exit` with a code after cleanup: `os.Exit` skips `defer`, so the status must
  be returned from a `run()` function.
