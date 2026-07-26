#!/usr/bin/perl
# CASE stmt-05: `%` as a hash sigil versus modulus, and `x` as the repetition
# operator versus a variable name versus a bareword. Both are resolved purely by
# expect-term / expect-operator state.
use strict; use warnings;

my %h = (a=>1, b=>2);
my $r = \%h;

print "stmt-05 hash-sigil: ", join(",", sort keys %h), "\n";
print "stmt-05 hash-deref: ", join(",", sort keys %$r), "\n";
print "stmt-05 modulus: ", 17 % 5, "\n";
print "stmt-05 mod-assign: ", do { my $n = 17; $n %= 5; $n }, "\n";

# `%` immediately followed by `$`: sigil in term position, modulus in operator
# position. The same three characters, two meanings.
my $five = 5;
print "stmt-05 mod-by-var: ", 17 % $five, "\n";
print "stmt-05 deref-count: ", scalar(keys %$r), "\n";

# `x` repetition versus $x versus a bareword `x` key.
my $x = 3;
print "stmt-05 x-op-str: ", ("ab" x 2), "\n";
print "stmt-05 x-op-list: ", join(",", (1,2) x 2), "\n";
print "stmt-05 x-var: $x\n";
print "stmt-05 x-op-with-var: ", ("z" x $x), "\n";
print "stmt-05 x-no-space: ", (4 x 3), "\n";
print "stmt-05 x-assign: ", do { my $s = "a"; $s x= 3; $s }, "\n";
my %k = (x => "key-x");
print "stmt-05 x-as-key: $k{x}\n";

# `xor` is a word operator that starts with x.
print "stmt-05 xor: ", ((1 xor 0) ? "true" : "false"), "\n";

# `x` right after a `)` is the operator; right after `(` it would be a bareword.
print "stmt-05 after-paren: ", (("q") x 2), "\n";
