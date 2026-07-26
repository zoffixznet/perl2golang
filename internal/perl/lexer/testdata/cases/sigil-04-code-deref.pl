#!/usr/bin/perl
# CASE sigil-04: `&` as a CODE sigil versus `&` as bitwise-and. `&$x`, `&{$x}`,
# `\&func`, `&func` (implicit @_ pass-through) and `$x & $y`.
use strict; use warnings;

sub add2 { return "add2(" . join(",", @_) . ")" }
my $code = \&add2;

print "sigil-04 deref-call: ", &$code(1,2), "\n";
print "sigil-04 braced-call: ", &{$code}(3,4), "\n";
print "sigil-04 arrow-call: ", $code->(5,6), "\n";
print "sigil-04 ref-to-named: ", ref(\&add2), "\n";

# `&func;` with no parens passes the CURRENT @_ through.
sub outer { return &inner }
sub inner { return "inner got [" . join(",", @_) . "]" }
print "sigil-04 implicit-args: ", outer("a","b"), "\n";

# Bitwise and, in operator position.
my ($p, $q) = (0b1100, 0b1010);
printf "sigil-04 bitand: %04b\n", $p & $q;
print "sigil-04 bitand-assign: ", do { my $t = 0b1100; $t &= 0b1010; sprintf("%04b",$t) }, "\n";
print "sigil-04 logical-and: ", (1 && 2), "\n";

# The trap: `&` followed by an identifier in TERM position is a call.
my @r = (&add2(7), 0b1111 & 0b0101);
print "sigil-04 mixed: $r[0] / $r[1]\n";
