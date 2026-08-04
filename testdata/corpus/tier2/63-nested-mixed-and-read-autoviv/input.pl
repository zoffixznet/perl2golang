#!/usr/bin/perl
use strict;
use warnings;

# The two nested-hash shapes that still do not come out right.

# 1. A record under each key, where the fields are of different kinds: a
#    number, some text and a list. One Go map has one value type, so the
#    inner hash cannot be a map of numbers the way a counter can.
my %stat;
for my $line ( 'web1 200 fast', 'web1 500 slow', 'db1 200 fast' ) {
    my ( $host, $code, $tag ) = split ' ', $line;
    $stat{$host}{count}++;
    $stat{$host}{last} = $code;
    push @{ $stat{$host}{tags} }, $tag;
}
for my $host ( sort keys %stat ) {
    printf "%-5s count=%d last=%s tags=%s\n",
        $host, $stat{$host}{count}, $stat{$host}{last},
        join( ',', @{ $stat{$host}{tags} } );
}

# 2. Reading through a nested hash creates the levels above the one being
#    read. This is Perl's read-autovivification, and it surprises Perl
#    developers too.
my %tree;
$tree{alpha}{one} = 1;
printf "before: %d key(s)\n", scalar keys %tree;
my $probe = $tree{beta}{two};
printf "after reading tree{beta}{two}: %d key(s): %s\n",
    scalar keys %tree, join( ',', sort keys %tree );
printf "beta exists: %s, two under it: %s\n",
    ( exists $tree{beta} ? 'yes' : 'no' ),
    ( exists $tree{beta}{two} ? 'yes' : 'no' );
