#!/usr/bin/perl
# A counter package sharing state with the script through package variables:
# the same hash spelled %TALLY inside the package and %Counter::TALLY from
# main, plus a threshold the script pokes in from outside, fully qualified.
use strict;
use warnings;

package Counter;

our %TALLY;
our $LIMIT = 3;

sub bump {
    my ($what) = @_;
    $TALLY{$what}++;
    return $TALLY{$what} <= $LIMIT ? 'ok' : 'over';
}

sub report {
    my @out;
    for my $k ( sort keys %TALLY ) {
        push @out, "$k=$TALLY{$k}";
    }
    return join ' ', @out;
}

package main;

for my $item (qw(apple apple pear apple apple pear)) {
    my $state = Counter::bump($item);
    print "$item -> $state\n";
}
print "tally: ", Counter::report(), "\n";

# main reads and writes the package's variables by their full names.
printf "apples counted: %d\n", $Counter::TALLY{apple};
$Counter::LIMIT = 10;
$Counter::TALLY{plum} = 7;
print "after meddling: ", Counter::bump('plum'), " / ", Counter::report(), "\n";
