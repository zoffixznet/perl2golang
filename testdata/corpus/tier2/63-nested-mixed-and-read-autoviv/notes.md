# 63 - nested hashes that are records, and read-autovivification

## What this exercises
The neighbour of entry 62. There every leaf under a key was the same kind of
thing, so the inner hash is a map with one value type. Here it is not.

```perl
$stat{$host}{count}++;
$stat{$host}{last} = $code;
push @{ $stat{$host}{tags} }, $tag;
```

Three literal keys, three different kinds of value. That inner hash is a
**record**, not a table: in Go it is a struct with a number, a string and a
slice, and it is a different translation from the counting hash even though it
is written with the same syntax. The evidence is that the keys are literals
and there are only three of them, which is the analysis entry 45 already does
for a hash *reference* and does not yet do for a nested hash built by
accumulation.

The second half is Perl's read-autovivification. `my $probe = $tree{beta}{two}`
creates `$tree{beta}` merely by looking through it, so a hash grows a key by
being read. Go's nested read is safe at every depth and creates nothing, which
is better behaviour and a genuine difference in output.

## What goes wrong today, and why it is here
The program panics: `interface conversion: interface {} is nil, not
[]interface {}`. The inner hash stays `map[string]any`, the `push` asserts a
slice out of a key that holds nothing, and the whole program dies on the first
row. That is an R3.2 violation on top of a typing gap: even with the type
wrong, the assertion should degrade to an empty slice rather than take the
program down.

The read-autovivification lines then print a different count from Perl's,
which is the honest answer: reproducing it would mean emitting a create-on-read
guard at every nested read, and that would make every generated report script
worse to read in exchange for copying a Perl wart.

## Go concepts a converter must teach
- A struct and a map are both written `$h{$k}` in Perl and are different types
  in Go. Literal keys and mixed value kinds are the evidence for a struct.
- An unchecked type assertion is a panic waiting for the first row of real
  data. The comma-ok form is the same code with the crash removed.
- Reading through nested maps is safe at any depth and creates nothing, which
  is one of the few places where Go's behaviour is both simpler and less
  surprising than Perl's.
