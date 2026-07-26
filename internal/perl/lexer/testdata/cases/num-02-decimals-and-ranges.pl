#!/usr/bin/perl
# CASE num-02: `.5`, `1.`, `1_000_000`, and the collision between a trailing `.`
# on a number and the `..` range / `.` concatenation operators.
use strict; use warnings;

print "num-02 leading-dot: ", .5, " ", 0.5, "\n";
print "num-02 trailing-dot: ", 1., "\n";
print "num-02 underscores: ", 1_000_000, " ", 1_0.0_1, "\n";
print "num-02 exponent: ", 1e10, " ", 1E-5, " ", 1.5e3, " ", 1_0e1, "\n";
print "num-02 exp-plus: ", 1e+3, "\n";

# `1..5` is a RANGE: the lexer must not read `1.` then `.5`.
print "num-02 range: ", join(",", 1..5), "\n";
print "num-02 range-count: ", scalar(my @r = 1..5), "\n";

# `1...5` is the flip-flop-ish three-dot range in list context (same list).
print "num-02 three-dot: ", join(",", 1...4), "\n";

# `1 . 5` with spaces is CONCATENATION producing the string "15".
print "num-02 concat: ", 1 . 5, " (string of length ", length(1 . 5), ")\n";

# `1.5.2` is a v-string, not a number.
my $vs = 1.5.2;
print "num-02 vstring-ords: ", join(",", map { ord } split //, $vs), "\n";

# Numeric literal immediately followed by a method-ish arrow is not a thing;
# but `1..$n` where $n follows the dots must still lex as a range.
my $n = 3;
print "num-02 range-var: ", join(",", 1..$n), "\n";
