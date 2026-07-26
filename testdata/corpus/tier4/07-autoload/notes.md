# 07-autoload: AUTOLOAD dispatch on arbitrary method names

Group: **A — genuinely impossible without an interpreter**

## Construct
`Record::AUTOLOAD` (line 11) intercepts every call to an undefined method, parses
the method NAME out of `$AUTOLOAD`, and synthesizes behaviour: `get_*` reads a
field, `set_*` writes one, anything else dies. `get_color`, `set_size`,
`get_size`, `launch_missiles` exist nowhere in the source.

## Why it resists conversion to Go
The callable method set is unbounded and defined by a regex over names at call
time. Go has no `method_missing` hook; every method must exist at compile time.
The general case is undecidable: the method name may itself be computed
(`$r->$method()`), and AUTOLOAD bodies may `goto &$sub` or define the sub on the
fly.

## What the converter should do
- Category: **refuse-statement** at the package level: convert `new` normally,
  refuse each call to a method that does not exist in the package (it would be a
  compile error in Go anyway) with a diagnostic explaining that dispatch would
  have gone to AUTOLOAD, and emit a panicking stub method for each such call
  site so the rest of the file compiles.
- Documented narrowing it MAY implement: when the AUTOLOAD body is a total
  function of the name over a statically enumerable call set (here:
  `get_color`, `set_size`, `get_size`, `launch_missiles` — all literal), the
  converter may specialize each name by symbolically applying the regexes, and
  must then reproduce `expected_stdout` exactly, including the death message for
  `launch_missiles` being caught by the surrounding `eval {}`.
- It must NOT emit a Go method set containing only the subs that literally exist
  (`new`, `AUTOLOAD`) and report success.

## Ideal diagnostic (word for word)
> input.pl:24: error P2G-E105: method 'get_color' is not defined anywhere;
> in Perl this call is handled by Record::AUTOLOAD (input.pl:11), which invents
> behaviour from the method name at run time. Emitted a panicking stub. Define
> the methods explicitly (get_color, set_size, get_size appear in this file), or
> convert the AUTOLOAD pattern to explicit getters/setters before converting.

## What a human should do instead
Enumerate the real API: write explicit `get_color`/`set_size`/... accessors (in
Go: ordinary methods on `Record`), and turn the "unknown method" branch into a
compile error, which is what Go gives for free.

## Observed with perl 5.42.2 (x86_64-linux)
`expected_stdout` (exit 0): `color=red`, `size=5`,
`error: no such method: launch_missiles`. Note the third line: even the FAILURE
path is part of the observable behaviour a specializing converter must keep.
