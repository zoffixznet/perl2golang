# What this exercises

Every way of reading @_ without naming it: a slice of the first two
arguments, `scalar @_`, an index past the end answered with a default, and
`\@_` carried into a summing loop that only reads. All of it converts onto
the variadic parameter the sub is given.

# What makes it hard

The slice and reference forms reach the argument list through the array
variable rather than through `$_[i]`, so each lowering has to resolve `@_`
to the parameter instead of inventing a package-level variable for it. The
read past the end has to stay a zero value, not a panic.
