# 84 - the same sub read in three contexts, and one that asks which

## What this exercises
The neighbour of entry 59. There the sub was read in list context and as a
truth value, and one Go signature covers both: return the capture groups, and
nil when the pattern did not match, because nil is empty in the list and false
in the test.

This entry adds the two readings that signature cannot cover.

- **Scalar context on the sub itself.** `my $one = parts($row)` puts the match
  in scalar context through the return, where it is 1 or the empty string
  rather than the groups. A Go function has one return type, and this one
  returns the groups, so the scalar reading gets the list.
- **`wantarray`.** The sub asks which context it was called in and answers
  differently. Go has no such question: a function cannot see how its result
  will be used, and there is nothing in the language to attach the answer to.

## Perl constructs
- `return $t =~ /(\w+)\s+(\d{4})/` read in list, boolean and scalar context
- `//` supplying a default for a capture that is not there
- a statement-modifier `for` over a ternary that calls the sub as a test
- `wantarray ? @got : scalar @got`

## What goes wrong today
The list and boolean readings are right. The scalar reading prints the whole
list where Perl printed 1, and `wantarray` picks one branch for both callers,
so the list reading of it reports one value instead of two.

The honest answer for `wantarray` is a refusal at the `wantarray` itself,
naming the two call sites that disagree, rather than a silent pick. For the
scalar reading, the converter knows both the sub's shape and the assignment's
shape at the call site, so `len(...) > 0` is available there and is what the
Perl meant.

## Go concepts a converter must teach
- Context is the single largest thing Perl has that Go does not. A Go function
  returns what it returns; the caller cannot change it by how it asks. Every
  Perl idiom built on context has to become an explicit choice somewhere, and
  the honest place for it is the signature.
- Where one function really does have to answer two questions, Go's answer is
  two functions, or one function and a second result. Both are visible at the
  call site, which is the point.
