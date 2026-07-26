# 22-vending-fsm

Vending machine FSM: transition table as hash-of-hashes of coderefs,
stdin event stream, three failure modes (illegal event, rejected event,
soft refusal), audit trail.

## Constructs exercised
- `%fsm{state}{event} = \&handler` dispatch table; handler lookup failing
  means "illegal in this state" (absence-as-semantics)
- named subs taken by reference (`\&insert_coin`); handlers RETURN the next
  state; self-transitions keep state and credit
- `$handler->( defined $arg ? $arg : () )` -- conditionally passing zero or
  one argument via the empty-list trick
- three distinct failure channels a converter must keep apart:
  1. illegal event (no table entry, logged, no state change)
  2. `die` inside a handler, caught by `eval`, state unchanged
  3. soft refusal (handler runs, logs, returns current state)
- closure-free package-level mutable state (`$state`, `$credit`, `%stock`)
  mutated from handlers
- `note()` helper capturing `$state` at call time in a formatted audit line
- `split ' ', $line, 2` with a LIMIT; comment/blank filtering
- `grep { $amount == $_ } 10, 20, 50, 100` membership over a literal list
- takings computed against an inline anonymous-hashref literal of the
  initial stock (`{ cola => 2, ... }->{$_}`)

## Conversion challenges
- map-of-maps-of-function-values: Go type
  `map[string]map[string]func(string) (string, error)` -- but Perl handlers
  have flexible arity (refund takes none), forcing signature unification
- `die`/`eval` inside table-driven dispatch -> error returns threaded
  through the event loop, without conflating channel 1 with channel 2
- global mutable machine state -> a Machine struct with methods (the
  natural refactor Go teaches; the audit trail keeps behavior observable)
- the `()` empty-list argument trick has no Go analogue -- optional args
  become explicit
- audit formatting `%-10s` with a bracketed state token

## Go teaching opportunities
- state machines as typed constants + method tables, error taxonomies,
  event-loop testing with golden transcripts
