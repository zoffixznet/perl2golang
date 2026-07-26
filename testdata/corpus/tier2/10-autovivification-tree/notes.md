# 10 - autovivification building a tree

## What this exercises
**The second load-bearing autovivification entry.** A directory tree is grown
from flat path strings with no node-creation code at all: a cursor reference
is walked downwards and each missing level materialises on contact.

## Perl constructs
- `my $node = \%tree;` then `$node = \%{ $node->{$part} };` - reassigning a
  cursor to a hashref that is created by the very act of dereferencing it
- `$node->{$leaf} = undef;` using `undef` as a sentinel meaning "file, not
  directory", distinguished later by `defined $child`
- `split m{/}, $path` with brace delimiters to avoid escaping slashes
- `pop @parts` to peel the leaf off
- recursive `render` with an indent-prefix accumulator, `sort keys %$node`
- a second autovivifying accumulator: `$dir_total{$so_far} += $size{$path};`
  building running prefixes
- `$depth_count{ scalar(() = $_ =~ m{/}g) }++` - the count-of idiom on a `//g`
  match, used directly as a hash key
- `sort { $a <=> $b } keys` for numeric key ordering
- ternary in string building: `length $so_far ? "$so_far/$part" : $part`

## Go concepts a converter must teach
- The cursor idiom `$node = \%{ $node->{$part} }` is the purest form of the
  problem: in Go it must become
  `if _, ok := node[part]; !ok { node[part] = map[string]any{} }; node = node[part].(map[string]any)`
  including a type assertion, because the value type is heterogeneous
  (map for directories, nil for files).
- A better target is a real `type Node struct { Children map[string]*Node;
  IsFile bool }`. Recognising that the `undef` sentinel encodes a *variant*
  is the interesting inference for a converter.
- `undef` as a value that is distinguishable from "absent" - Go needs either a
  pointer or an explicit boolean field; a nil map value cannot carry that
  distinction on its own.
- `+=` on a missing key works in Go for depth 1 (`m[k] += n`), so the totals
  loop converts cleanly while the tree loop does not - a useful contrast in
  one file.
- The recursive `render` maps to a normal Go recursion, but `sort keys` must be
  reproduced explicitly since Go map iteration is randomised.
- `scalar(() = $s =~ /x/g)` is "count matches": `len(re.FindAllString(s, -1))`
  or `strings.Count`.
