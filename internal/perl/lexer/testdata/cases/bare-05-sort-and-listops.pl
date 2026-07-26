#!/usr/bin/perl
# CASE bare-05: `sort SUBNAME LIST` -- a bareword in the comparator slot, with no
# comma after it. Same slot accepts a BLOCK or a scalar holding a code ref.
# `print`, `exec`, `system` have the same "bareword in slot 1" shape.
use strict; use warnings;

sub by_num { return $a <=> $b }
my $cmp = sub { $b <=> $a };
my @n = (10, 2, 33);

print "bare-05 subname: ", join(",", sort by_num @n), "\n";
print "bare-05 block:   ", join(",", sort { $a <=> $b } @n), "\n";
print "bare-05 coderef: ", join(",", sort $cmp @n), "\n";
print "bare-05 default: ", join(",", sort @n), "\n";

# The trap: `sort by_num @n` has NO comma, but `sort($cmp, @n)` with a comma is a
# completely different (and wrong) call.
my @wrong = sort($cmp, @n);
print "bare-05 with-comma-is-different: ", scalar(@wrong), " elements\n";

# A bareword sub called as a list operator with no parens.
sub shout { return "[" . uc(join(",", @_)) . "]" }
print "bare-05 listop: ", shout "a", "b", "\n";

# `reverse`, `join` and friends in the same shape.
print "bare-05 nested-listops: ", join "-", reverse sort by_num @n;
print "\n";

# A bareword immediately followed by `(` is a call; followed by anything else it
# is a list operator (or a string under no strict).
print "bare-05 with-parens: ", shout("x"), "\n";
