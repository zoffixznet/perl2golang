#!/usr/bin/perl
use strict;
use warnings;

# The neighbour of tier2 entry 87. `use integer` is lexical, and lexical means
# the text it encloses, not the calls it makes. A sub declared outside the
# pragma keeps floating-point arithmetic even when called from inside it, and a
# sub declared inside keeps whole-number arithmetic wherever it is called from.

sub outside_half { my ($n) = @_; return $n / 2 }

my $inside_half;
{
    use integer;
    $inside_half = sub { my ($n) = @_; return $n / 2 };
}

print "--- the same call, from both sides of the pragma ---\n";
printf "outside, called outside: %s\n", outside_half(7);
printf "inside,  called outside: %s\n", $inside_half->(7);
{
    use integer;
    printf "outside, called inside:  %s\n", outside_half(7);
    printf "inside,  called inside:  %s\n", $inside_half->(7);
}

print "--- the pragma reaches the whole rest of its block ---\n";
{
    use integer;
    my $mid = 9;
    printf "half of %d is %d\n", $mid, $mid / 2;
    {
        # A nested block inherits it.
        printf "and a third is %d\n", $mid / 3;
    }
}

print "--- a named sub declared inside the pragma's block ---\n";
sub outer_half { my ($n) = @_; return $n / 2 }
{
    use integer;
    sub inner_half { my ($n) = @_; return $n / 2 }
}
printf "outer_half(7) = %s\n", outer_half(7);
printf "inner_half(7) = %s\n", inner_half(7);

print "--- the pragma also truncates the operands of + - * ---\n";
{
    use integer;
    printf "7.9 + 0.2   = %s\n", 7.9 + 0.2;
    printf "2.5 * 3.5   = %s\n", 2.5 * 3.5;
    printf "10.9 - 0.5  = %s\n", 10.9 - 0.5;
}
printf "and outside:  7.9 + 0.2 = %s\n", 7.9 + 0.2;

print "--- no integer turns it back off ---\n";
{
    use integer;
    printf "with:    %s\n", 9 / 2;
    {
        no integer;
        printf "without: %s\n", 9 / 2;
    }
    printf "with:    %s\n", 9 / 2;
}
