#!/usr/bin/perl
# TRAP: % with a negative operand. Perl's result takes the sign of the
# RIGHT operand; Go's % takes the sign of the LEFT. Two of these four
# answers differ between the languages.
use strict;
use warnings;

for my $pair ( [ 7, 3 ], [ -7, 3 ], [ 7, -3 ], [ -7, -3 ] ) {
    my ( $a, $b ) = @$pair;
    printf "%3d %% %3d = %3d\n", $a, $b, $a % $b;
}
# Perl: 7%3=1  -7%3=2   7%-3=-2  -7%-3=-1
# Go:   7%3=1  -7%3=-1  7%-3=1   -7%-3=-1

my $idx = -1 % 5;
print "ring index: $idx\n";    # 4 in Perl, -1 in Go: classic ring-buffer bug
