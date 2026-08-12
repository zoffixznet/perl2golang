# What this exercises

Four places where a Perl block's value is chosen inside it rather than
written at the end: a `do` block whose value is a conditional with a second
conditional nested in one branch, a `map` block that formats two ways, a
`grep` block whose test is a conditional, and a sub whose last statement is
an anonymous sub, which is the closure the caller gets back.

# What makes it hard

Go has no block value at all: `if` is a statement, a function literal is an
expression only where one is expected, and a bare value in the middle of a
block does not compile. So each of these has to be recognised as "the value
of the block" on the way in, because by the time the block has been lowered
the value is a statement that produced nothing and gets dropped as dead
code.

The nesting is the part that separates a rule from a special case. One
conditional at the end of a `do` block was already understood; a conditional
inside one of its branches was not, and the file that made this entry
necessary chose an encoding that way. The counter closure is the same
question one level up: `sub { $n++ }` ends in an increment, which is an
expression in Perl and a statement in Go, and the value it hands back is the
whole reason the closure exists.
