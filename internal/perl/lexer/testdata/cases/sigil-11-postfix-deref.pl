#!/usr/bin/perl
# CASE sigil-11: postfix dereference (`->@*`, `->%*`, `->$*`, `->$#*`, `->@[...]`,
# `->@{...}`, `->%[...]`). The `*` here is NOT multiplication and the `$#` is NOT
# a comment; they are part of a fixed postfix token following `->`.
use strict; use warnings;

my $ar = [10,20,30];
my $hr = { a=>1, b=>2 };
my $sr = \"scalar";
my $cr = sub { "code(" . join(",",@_) . ")" };

print "sigil-11 array: ", join(",", $ar->@*), "\n";
print "sigil-11 hash: ", join(",", map {"$_=$hr->{$_}"} sort keys $hr->%*), "\n";
print "sigil-11 scalar: ", $sr->$*, "\n";
print "sigil-11 lastindex: ", $ar->$#*, "\n";
print "sigil-11 code: ", $cr->(1,2), "\n";
print "sigil-11 array-slice: ", join(",", $ar->@[0,2]), "\n";
print "sigil-11 hash-slice: ", join(",", $hr->@{qw(a b)}), "\n";
print "sigil-11 kv-slice: ", join(",", $hr->%{'a'}), "\n";

# Postfix deref interpolates in strings ONLY under the postderef_qq feature.
# Without it, `$ar->@*` is the ref stringified followed by literal "->@*".
print "sigil-11 interp-no-feature: ", ("$ar->@*" =~ /->\@\*\z/ ? "literal ->\@* kept" : "interpolated"), "\n";
{
    use feature 'postderef_qq';
    print "sigil-11 interp-with-feature: [$ar->@*]\n";
}
# `->$*` is NEVER interpolated: in a string `$*` is the removed pre-5.30 variable.

# The tokens that would collide: multiply and comment.
my $n = 3;
print "sigil-11 not-multiply: ", $n * 2, " not-comment: $#{$ar}\n";
