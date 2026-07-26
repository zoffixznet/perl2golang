# Exercises

These are tasks against the code in this directory, not toy examples. Each one is small enough for one sitting and ends with a check, so you can tell whether you got it right without asking anyone.

Work them in order. They are arranged so that each one leaves the code in a better state than it found it, and the later ones assume the earlier ones are done.

Run everything from the directory above this one, where `go.mod` lives.

## 1. Write a table-driven test for `summarise`

Create `main_test.go` next to `main.go`, in `package main`, and write `TestSummarise`. Use the table-driven form that Go code uses everywhere: a slice of anonymous structs holding a case name, the inputs, and the expected output, then one `t.Run(tc.name, ...)` per row. Cover an ordinary input and at least one edge case, such as empty input or a value the original handled by accident.

Done when: `go test ./...` passes, and `go test -run TestSummarise -v` lists one subtest per row of your table. Then change `summarise` to be wrong on purpose and confirm the failure message names the case that broke.

Lessons: [Multiple returns replace both list-return and wantarray](concepts/multiple-return-values.md)

## 2. Make the "nil is not undef, and nothing autovivifies" trap happen on purpose

Read [the lesson](concepts/nil-vs-undef.md), then write the smallest possible program in a scratch directory that triggers the failure it describes, and run it. Once you have seen the failure with your own eyes, go back to `main.go` and find the place where the same shape appears in your converted code. Decide whether the generated code is actually safe there, and write a comment saying why.

Done when: You have seen the real error message rather than read about it, and you can point at the line in `main.go` that would produce it if the guard were removed.

Lessons: [nil is not undef, and nothing autovivifies](concepts/nil-vs-undef.md)

## 3. Return an error instead of exiting

The generated code stops the program by calling `os.Exit`. Pick a call site that is not in `main`. Change the function it sits in so that it returns an `error` as its last result, propagate that error up with `fmt.Errorf("doing the thing: %w", err)` at each level, and let `main` be the only function that decides to stop the program. `defer` statements do not run when `os.Exit` is called, so this change also fixes cleanup you may not have noticed was being skipped.

Done when: Only `main` calls `os.Exit` (or nothing does, and `main` simply returns). `go vet ./...` is silent, the program still exits with a non-zero status on the failure path (check with `go run . ; echo $?`), and the error message now says what was being attempted, not just what went wrong.

Lessons: [Errors are return values, not exceptions](concepts/errors-are-values.md) and [The if err != nil rhythm, and why silence still compiles](concepts/if-err-nil-rhythm.md)

## 4. Replace one loop with a slices call

Find a loop in the generated code that searches for an element, tests membership, or removes elements, and replace it with `slices.Contains`, `slices.IndexFunc`, `slices.ContainsFunc`, or `slices.DeleteFunc` from the standard library. Leave the loops that transform or accumulate alone: Go has no `map` or `grep` over slices, and the explicit loop is the idiom there, not a failure of taste.

Done when: The program compiles, `go vet ./...` is silent, and running it against the same input produces byte-identical output (`go run . > after.txt` and `diff` it against the output you saved first). The file is shorter than it was.

Lessons: [Slices are views with capacity, arrays are values](concepts/slices-not-arrays.md), [range gives you the index first, and the element is a copy](concepts/range-is-not-foreach.md), and [Sorting is a function call, and the default is numeric-aware](concepts/sort-slice.md)

## 5. Finish string eval

The converter refused string eval at line 21 of `summarise.pl`. Decide what the string is really for. If it is a small expression language for users, parse it yourself or use an expression library. If it is a fixed set of alternatives, replace it with a map of named functions, which is what the code almost certainly wants. Implement it by hand in the generated code and delete the TODO that marks it.

Done when: The TODO is gone, the program compiles, and you have a test that fails against the version without your fix. If you conclude the original behaviour was not worth reproducing, write that decision down in a comment; that is a valid answer, and an undocumented one is not.

Lessons: [Errors are return values, not exceptions](concepts/errors-are-values.md) and [The compiler is the first test suite](concepts/compile-time-mindset.md)

## 6. Make the project idiomatic on its own terms

Run `gofmt -l .` and `go vet ./...` and fix anything they report. Then add a package comment above `package main` in `main.go` saying in one sentence what the program does, name any single-letter variable that survives more than five lines, and delete anything the compiler tells you is unused. Finally run `go doc .` and read what your own package now says about itself.

Done when: `gofmt -l .` prints nothing, `go vet ./...` prints nothing, and `go doc .` shows a sentence that would be useful to someone who has never seen this program.

Lessons: [Capitalisation is the entire privacy system](concepts/packages-and-exported-names.md)

---

When these stop being interesting, the next exercise is the real one: pick the part of the generated code you like least, delete it, and write it the way you would write it now. That is the point at which the translation stops being someone else's code.

Written by perl2go 0.1.0, from your source.
