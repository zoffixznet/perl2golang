# What this exercises

A tree built out of flat paths by walking a cursor down it:
`$node = \%{ $node->{$part} }` once per path component, with no statement
anywhere that creates a node. Then a recursive render that tells a branch
from a leaf with `ref`.

# What makes it hard

Two separate things have to be right, and each one is silent when it is not.

Taking a reference to a hash element is what creates the element, so the
line that reads like navigation is the line that builds the tree. Go creates
nothing on its own, so the check and the make have to be written out in
front of the read, and a conversion that leaves them out produces a program
that runs to the end and prints an empty tree.

And the reference itself is not an address. A Go map already refers to its
data, so `\%{ ... }` is the map value, where taking its address is not even
legal Go for a map element. Emitting `&m[k]` there does not compile, which
is at least loud; emitting a copy would not be, and would produce a tree
whose branches were all thrown away.
