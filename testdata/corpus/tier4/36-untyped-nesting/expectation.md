# What the tool must say about this file

- category: `approximate`
- valid Go is emitted and the whole file compiles
- the report says the structure's element types did not resolve: `P2G3010`,
  the dynamic-value diagnostic, appears for `%doc`
- diagnostic-must-contain: `any`
- the built program **runs to its last line**. Every read of a value whose
  type did not resolve is a guess, and several of the guesses here are wrong;
  a wrong guess has to leave an empty collection and let the next line run,
  never stop the program. This is the standard R3.2 sets for a partial
  conversion, and the scorecard counts programs that fail it under
  "programs that panicked".
- the lines that do not depend on a guess are correct: the scalar field, the
  sorted key walk, the `ref` dispatch for the list and scalar cases, and
  `defined` on a missing key
- must-not: emit a bare `x.(map[string]any)` or `x.(\[\]any)` on a value the
  inference left dynamic, because that is the shape that stops the program
- must-not: claim the structure was typed
