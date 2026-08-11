#!/usr/bin/perl
use strict;
use warnings;

# Three spellings that were ordinary before 2000 and are still all over
# installed module code: calling a sub with an ampersand in front of it,
# labelling a loop with a lower-case name, and letting an if/else be the last
# thing in a sub so that the branch taken is what the sub returns.

sub classify {
    my ($n) = @_;
    if ( $n < 0 ) {
        'negative';
    }
    elsif ( $n == 0 ) {
        'zero';
    }
    else {
        'positive';
    }
}

sub double { my ($n) = @_; return $n * 2 }

sub describe {
    my ( $label, $n ) = @_;
    printf "%-8s %3d -> %-8s doubled %d\n", $label, $n, &classify($n), &double($n);
    return;
}

&describe( 'below', -4 );
&describe( 'exactly', 0 );
&describe( 'above', 7 );

my @batches = ( [ 5, 6 ], [ 1, 2, 0 ], [ 3, -1, 4 ] );
my $kept = 0;

each_batch: for my $batch (@batches) {
    for my $n (@$batch) {
        next each_batch if $n == 0;
        last each_batch if $n < 0;
        $kept++;
    }
    print "batch of ", scalar(@$batch), " finished whole\n";
}
print "kept $kept value(s)\n";

# The same if/else tail, one level in: the branch is itself an if.
sub bucket {
    my ($n) = @_;
    if ( $n > 10 ) {
        if ( $n > 100 ) { 'huge' }
        else            { 'big' }
    }
    else {
        'small';
    }
}
print join( ' ', map { &bucket($_) } 5, 50, 500 ), "\n";
